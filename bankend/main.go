package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

//go:embed schema.sql
var schema string
var zone, _ = time.LoadLocation("Asia/Shanghai")

type app struct{ db *sql.DB }
type userKey struct{}
type plan struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Cycle       string `json:"cycle"`
	Target      int    `json:"target"`
	Start       string `json:"start_date"`
	End         string `json:"end_date"`
	Count       int    `json:"checkin_count"`
	Today       bool   `json:"checked_today"`
}

func env(k, d string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return d
}
func output(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, s string) {
	output(w, status, map[string]string{"error": s})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		fail(w, 400, "请求格式错误")
		return false
	}
	if d.Decode(&struct{}{}) != io.EOF {
		fail(w, 400, "请求格式错误")
		return false
	}
	return true
}
func uid(r *http.Request) int64 { return r.Context().Value(userKey{}).(int64) }
func today() string             { return time.Now().In(zone).Format("2006-01-02") }
func hash(s string) string      { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func (a *app) dbError(w http.ResponseWriter, e error) {
	log.Printf("database: %v", e)
	fail(w, 500, "服务暂时不可用")
}
func (a *app) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == r.Header.Get("Authorization") || len(token) != 64 {
			fail(w, 401, "请先登录")
			return
		}
		var id int64
		e := a.db.QueryRowContext(r.Context(), "SELECT user_id FROM sessions WHERE token_hash=? AND expires_at>UTC_TIMESTAMP()", hash(token)).Scan(&id)
		if errors.Is(e, sql.ErrNoRows) {
			fail(w, 401, "登录已过期，请重新登录")
			return
		}
		if e != nil {
			a.dbError(w, e)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, id)))
	})
}
func (a *app) credentials(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &v) {
		return
	}
	v.Username = strings.TrimSpace(v.Username)
	if len([]rune(v.Username)) < 3 || len([]rune(v.Username)) > 32 || len(v.Password) < 8 || len(v.Password) > 72 {
		fail(w, 400, "用户名需 3–32 个字符，密码需 8–72 字节")
		return
	}
	var id int64
	if r.URL.Path == "/api/register" {
		h, e := bcrypt.GenerateFromPassword([]byte(v.Password), bcrypt.DefaultCost)
		if e != nil {
			fail(w, 500, "密码处理失败")
			return
		}
		res, e := a.db.ExecContext(r.Context(), "INSERT INTO users(username,password_hash) VALUES(?,?)", v.Username, string(h))
		if e != nil {
			var me *mysql.MySQLError
			if errors.As(e, &me) && me.Number == 1062 {
				fail(w, 409, "用户名已存在")
			} else {
				a.dbError(w, e)
			}
			return
		}
		id, _ = res.LastInsertId()
	} else {
		var h string
		e := a.db.QueryRowContext(r.Context(), "SELECT id,password_hash FROM users WHERE username=?", v.Username).Scan(&id, &h)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			a.dbError(w, e)
			return
		}
		if e != nil || bcrypt.CompareHashAndPassword([]byte(h), []byte(v.Password)) != nil {
			time.Sleep(150 * time.Millisecond)
			fail(w, 401, "用户名或密码错误")
			return
		}
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		fail(w, 500, "令牌生成失败")
		return
	}
	token := hex.EncodeToString(b)
	exp := time.Now().UTC().Add(7 * 24 * time.Hour)
	if _, e := a.db.ExecContext(r.Context(), "INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,?)", hash(token), id, exp); e != nil {
		a.dbError(w, e)
		return
	}
	output(w, 200, map[string]any{"token": token, "expires_at": exp, "username": v.Username})
}
func validPlan(p plan) bool {
	s, e := time.Parse("2006-01-02", p.Start)
	t, f := time.Parse("2006-01-02", p.End)
	return strings.TrimSpace(p.Name) != "" && len([]rune(p.Name)) <= 100 && len([]rune(p.Description)) <= 2000 && (p.Cycle == "daily" || p.Cycle == "weekly") && p.Target > 0 && p.Target <= 10000 && e == nil && f == nil && !t.Before(s) && t.Sub(s) <= 3660*24*time.Hour
}
func (a *app) getPlans(r *http.Request) ([]plan, error) {
	rows, e := a.db.QueryContext(r.Context(), `SELECT p.id,p.name,p.description,p.cycle,p.target,DATE_FORMAT(p.start_date,'%Y-%m-%d'),DATE_FORMAT(p.end_date,'%Y-%m-%d'),COUNT(c.id),COALESCE(MAX(c.day=?),0) FROM plans p LEFT JOIN checkins c ON c.plan_id=p.id WHERE p.user_id=? GROUP BY p.id ORDER BY p.id DESC`, today(), uid(r))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	list := []plan{}
	for rows.Next() {
		var p plan
		if e = rows.Scan(&p.ID, &p.Name, &p.Description, &p.Cycle, &p.Target, &p.Start, &p.End, &p.Count, &p.Today); e != nil {
			return nil, e
		}
		list = append(list, p)
	}
	return list, rows.Err()
}
func (a *app) plans(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if r.Method == "GET" {
		ps, e := a.getPlans(r)
		if e != nil {
			a.dbError(w, e)
			return
		}
		if id > 0 {
			for _, p := range ps {
				if p.ID == id {
					output(w, 200, p)
					return
				}
			}
			fail(w, 404, "计划不存在")
			return
		}
		output(w, 200, ps)
		return
	}
	if r.Method == "DELETE" {
		res, e := a.db.ExecContext(r.Context(), "DELETE FROM plans WHERE id=? AND user_id=?", id, uid(r))
		if e != nil {
			a.dbError(w, e)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			fail(w, 404, "计划不存在")
			return
		}
		output(w, 200, map[string]bool{"ok": true})
		return
	}
	var p plan
	if !decode(w, r, &p) {
		return
	}
	if !validPlan(p) {
		fail(w, 400, "请检查计划名称、周期、目标次数和日期范围")
		return
	}
	if r.Method == "POST" {
		res, e := a.db.ExecContext(r.Context(), "INSERT INTO plans(user_id,name,description,cycle,target,start_date,end_date) VALUES(?,?,?,?,?,?,?)", uid(r), strings.TrimSpace(p.Name), p.Description, p.Cycle, p.Target, p.Start, p.End)
		if e != nil {
			a.dbError(w, e)
			return
		}
		p.ID, _ = res.LastInsertId()
		output(w, 201, p)
		return
	}
	// Lock the plan while checking history so an edit cannot race with a check-in.
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer tx.Rollback()
	var owner int64
	e = tx.QueryRowContext(r.Context(), "SELECT user_id FROM plans WHERE id=? AND user_id=? FOR UPDATE", id, uid(r)).Scan(&owner)
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "计划不存在")
		return
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	var outside int
	e = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM checkins WHERE plan_id=? AND (day<? OR day>?)", id, p.Start, p.End).Scan(&outside)
	if e != nil {
		a.dbError(w, e)
		return
	}
	if outside > 0 {
		fail(w, 409, "日期范围不能排除已有打卡记录")
		return
	}
	_, e = tx.ExecContext(r.Context(), "UPDATE plans SET name=?,description=?,cycle=?,target=?,start_date=?,end_date=? WHERE id=? AND user_id=?", strings.TrimSpace(p.Name), p.Description, p.Cycle, p.Target, p.Start, p.End, id, uid(r))
	if e != nil {
		a.dbError(w, e)
		return
	}
	if e = tx.Commit(); e != nil {
		a.dbError(w, e)
		return
	}
	p.ID = id
	output(w, 200, p)
}

type checkin struct {
	ID      int64  `json:"id"`
	PlanID  int64  `json:"plan_id"`
	Name    string `json:"plan_name"`
	Day     string `json:"day"`
	Note    string `json:"note"`
	Minutes int    `json:"minutes"`
}

func (a *app) checkins(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		pid := r.URL.Query().Get("plan_id")
		rows, e := a.db.QueryContext(r.Context(), `SELECT c.id,c.plan_id,p.name,DATE_FORMAT(c.day,'%Y-%m-%d'),c.note,c.minutes FROM checkins c JOIN plans p ON p.id=c.plan_id WHERE c.user_id=? AND (?='' OR c.plan_id=?) ORDER BY c.day DESC,c.id DESC LIMIT 500`, uid(r), pid, pid)
		if e != nil {
			a.dbError(w, e)
			return
		}
		defer rows.Close()
		list := []checkin{}
		for rows.Next() {
			var c checkin
			if e = rows.Scan(&c.ID, &c.PlanID, &c.Name, &c.Day, &c.Note, &c.Minutes); e != nil {
				a.dbError(w, e)
				return
			}
			list = append(list, c)
		}
		if e = rows.Err(); e != nil {
			a.dbError(w, e)
			return
		}
		output(w, 200, list)
		return
	}
	var c checkin
	if !decode(w, r, &c) {
		return
	}
	if c.PlanID <= 0 || c.Minutes < 1 || c.Minutes > 1440 || len([]rune(c.Note)) > 1000 {
		fail(w, 400, "学习时长需为 1–1440 分钟，心得不超过 1000 字")
		return
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer tx.Rollback()
	var start, end string
	e = tx.QueryRowContext(r.Context(), `SELECT DATE_FORMAT(start_date,'%Y-%m-%d'),DATE_FORMAT(end_date,'%Y-%m-%d') FROM plans WHERE id=? AND user_id=? FOR UPDATE`, c.PlanID, uid(r)).Scan(&start, &end)
	if errors.Is(e, sql.ErrNoRows) {
		fail(w, 404, "计划不存在")
		return
	}
	if e != nil {
		a.dbError(w, e)
		return
	}
	c.Day = today()
	if c.Day < start || c.Day > end {
		fail(w, 400, "今天不在计划日期范围内")
		return
	}
	res, e := tx.ExecContext(r.Context(), "INSERT INTO checkins(plan_id,user_id,day,note,minutes) VALUES(?,?,?,?,?)", c.PlanID, uid(r), c.Day, c.Note, c.Minutes)
	if e != nil {
		var me *mysql.MySQLError
		if errors.As(e, &me) && me.Number == 1062 {
			fail(w, 409, "该计划今天已打卡")
		} else {
			a.dbError(w, e)
		}
		return
	}
	if e = tx.Commit(); e != nil {
		a.dbError(w, e)
		return
	}
	c.ID, _ = res.LastInsertId()
	output(w, 201, c)
}
func streak(days map[string]int, now time.Time) int {
	d := now.In(zone)
	if days[d.Format("2006-01-02")] == 0 {
		d = d.AddDate(0, 0, -1)
	}
	n := 0
	for days[d.Format("2006-01-02")] > 0 {
		n++
		d = d.AddDate(0, 0, -1)
	}
	return n
}
func (a *app) stats(w http.ResponseWriter, r *http.Request) {
	ps, e := a.getPlans(r)
	if e != nil {
		a.dbError(w, e)
		return
	}
	rows, e := a.db.QueryContext(r.Context(), `SELECT DATE_FORMAT(day,'%Y-%m-%d'),COUNT(*),SUM(minutes) FROM checkins WHERE user_id=? GROUP BY day`, uid(r))
	if e != nil {
		a.dbError(w, e)
		return
	}
	defer rows.Close()
	days := map[string]int{}
	total, minutes := 0, 0
	for rows.Next() {
		var d string
		var n, m int
		if e = rows.Scan(&d, &n, &m); e != nil {
			a.dbError(w, e)
			return
		}
		days[d] = n
		total += n
		minutes += m
	}
	if e = rows.Err(); e != nil {
		a.dbError(w, e)
		return
	}
	trend := []map[string]any{}
	now := time.Now().In(zone)
	for i := 29; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		trend = append(trend, map[string]any{"day": d, "count": days[d]})
	}
	done := 0
	completion := []map[string]any{}
	for _, p := range ps {
		rate := float64(p.Count) / float64(p.Target)
		if rate >= 1 {
			done++
			rate = 1
		}
		completion = append(completion, map[string]any{"id": p.ID, "name": p.Name, "count": p.Count, "target": p.Target, "rate": rate})
	}
	rate := 0.0
	if len(ps) > 0 {
		rate = float64(done) / float64(len(ps))
	}
	output(w, 200, map[string]any{"total_checkins": total, "total_minutes": minutes, "streak": streak(days, now), "total_plans": len(ps), "completed_plans": done, "completion_rate": rate, "trend": trend, "plans": completion})
}
func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", a.credentials)
	mux.HandleFunc("POST /api/login", a.credentials)
	business := http.NewServeMux()
	for _, route := range []string{"GET /api/plans", "POST /api/plans", "GET /api/plans/{id}", "PUT /api/plans/{id}", "DELETE /api/plans/{id}"} {
		business.HandleFunc(route, a.plans)
	}
	business.HandleFunc("GET /api/checkins", a.checkins)
	business.HandleFunc("POST /api/checkins", a.checkins)
	business.HandleFunc("GET /api/stats", a.stats)
	business.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		_, e := a.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=?", hash(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
		if e != nil {
			a.dbError(w, e)
			return
		}
		output(w, 200, map[string]bool{"ok": true})
	})
	mux.Handle("/api/", a.auth(business))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if a.db.PingContext(r.Context()) != nil {
			fail(w, 503, "数据库不可用")
			return
		}
		output(w, 200, map[string]string{"status": "ok"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				log.Printf("panic: %v", e)
				fail(w, 500, "服务异常")
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		origin := r.Header.Get("Origin")
		if origin != "" && origin == env("CORS_ORIGIN", "http://localhost:8081") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("请配置 MYSQL_DSN")
	}
	cfg, e := mysql.ParseDSN(dsn)
	if e != nil {
		log.Fatal(e)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Timeout = 5 * time.Second
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		e = db.PingContext(ctx)
		if e == nil {
			break
		}
		select {
		case <-ctx.Done():
			log.Fatal("无法连接 MySQL")
		case <-time.After(time.Second):
		}
	}
	for _, q := range strings.Split(schema, ";") {
		if strings.TrimSpace(q) != "" {
			if _, e = db.ExecContext(ctx, q); e != nil {
				log.Fatal(e)
			}
		}
	}
	a := app{db}
	server := &http.Server{Addr: env("LISTEN_ADDR", ":8080"), Handler: a.handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 16}
	go func() {
		log.Printf("listening %s", server.Addr)
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	shutdown, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	fmt.Println("shutting down")
	server.Shutdown(shutdown)
}
