package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlanValidation(t *testing.T) {
	base := plan{Name: "Go 学习", Cycle: "daily", Target: 30, Start: "2026-10-01", End: "2026-10-30"}
	if !validPlan(base) {
		t.Fatal("valid plan rejected")
	}
	for _, mutate := range []func(*plan){func(p *plan) { p.End = "2026-09-01" }, func(p *plan) { p.Target = 0 }, func(p *plan) { p.Cycle = "bad" }, func(p *plan) { p.Name = "  " }, func(p *plan) { p.Start = "2026-02-30" }} {
		p := base
		mutate(&p)
		if validPlan(p) {
			t.Fatalf("invalid plan accepted: %+v", p)
		}
	}
}
func TestStreak(t *testing.T) {
	now := time.Date(2026, 10, 29, 1, 0, 0, 0, zone)
	for _, c := range []struct {
		days map[string]int
		want int
	}{{map[string]int{}, 0}, {map[string]int{"2026-10-29": 3, "2026-10-28": 1}, 2}, {map[string]int{"2026-10-28": 1, "2026-10-27": 2}, 2}, {map[string]int{"2026-10-27": 1}, 0}} {
		if n := streak(c.days, now); n != c.want {
			t.Fatalf("streak %d want %d", n, c.want)
		}
	}
}
func TestProtectedEndpoints(t *testing.T) {
	a := app{}
	for _, path := range []string{"/api/plans", "/api/plans/1", "/api/checkins", "/api/stats", "/api/logout", "/api/unknown"} {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("%s status=%d", path, w.Code)
		}
	}
}
func TestRejectTrailingJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"ok"} {}`))
	w := httptest.NewRecorder()
	var p plan
	if decode(w, r, &p) || w.Code != 400 {
		t.Fatal("accepted multiple JSON values")
	}
}
