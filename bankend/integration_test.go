package main

import (
 "bytes"
 "database/sql"
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/http/httptest"
 "os"
 "strings"
 "testing"
 "time"
)

// Uses a dedicated test database. Only users created by this test are removed.
func TestMySQLIntegration(t *testing.T) {
 dsn:=os.Getenv("TEST_MYSQL_DSN");if dsn==""{t.Skip("set TEST_MYSQL_DSN for real MySQL integration")}
 db,e:=sql.Open("mysql",dsn);if e!=nil{t.Fatal(e)};defer db.Close()
 for _,q:=range strings.Split(schema,";"){if strings.TrimSpace(q)!=""{if _,e=db.Exec(q);e!=nil{t.Fatal(e)}}}
 server:=httptest.NewServer((&app{db}).handler());defer server.Close()
 call:=func(method,path,token string,payload any,want int) map[string]any {
  t.Helper();var body io.Reader;if payload!=nil{b,_:=json.Marshal(payload);body=bytes.NewReader(b)};r,_:=http.NewRequest(method,server.URL+path,body);if token!=""{r.Header.Set("Authorization","Bearer "+token)};res,e:=http.DefaultClient.Do(r);if e!=nil{t.Fatal(e)};defer res.Body.Close();b,_:=io.ReadAll(res.Body);if res.StatusCode!=want{t.Fatalf("%s %s status %d want %d: %s",method,path,res.StatusCode,want,b)};var out map[string]any;json.Unmarshal(b,&out);return out
 }
 username:=fmt.Sprintf("integration_%d",time.Now().UnixNano());other:=username+"b"
 defer db.Exec("DELETE FROM users WHERE username IN (?,?)",username,other)
 user:=map[string]any{"username":username,"password":"test-password-2026"}
 token:=call("POST","/api/register","",user,200)["token"].(string)
 call("POST","/api/register","",user,409)
 call("POST","/api/login","",map[string]any{"username":username,"password":"wrong-password"},401)
 call("POST","/api/login","",user,200)
 otherToken:=call("POST","/api/register","",map[string]any{"username":other,"password":"test-password-2026"},200)["token"].(string)
 call("GET","/api/plans","",nil,401)
 p:=map[string]any{"name":"测试学习计划","description":"真实数据库集成验证","cycle":"daily","target":1,"start_date":today(),"end_date":today()}
 result:=call("POST","/api/plans",token,p,201);id:=fmt.Sprintf("%.0f",result["id"]);path:="/api/plans/"+id
 call("GET",path,otherToken,nil,404)
 call("PUT",path,otherToken,p,404)
 call("DELETE",path,otherToken,nil,404)
 check:=map[string]any{"plan_id":result["id"],"minutes":35,"note":"学习完成"}
 call("POST","/api/checkins",otherToken,check,404)
 call("POST","/api/checkins",token,check,201)
 call("POST","/api/checkins",token,check,409)
 detail:=call("GET",path,token,nil,200);if detail["checked_today"]!=true||detail["checkin_count"]!=float64(1){t.Fatal(detail)}
 stats:=call("GET","/api/stats",token,nil,200);if stats["total_checkins"]!=float64(1)||stats["streak"]!=float64(1)||stats["completion_rate"]!=float64(1)||stats["total_minutes"]!=float64(35){t.Fatal(stats)}
 p["name"]="编辑后的计划";call("PUT",path,token,p,200)
 call("DELETE",path,token,nil,200);call("GET",path,token,nil,404)
 stats=call("GET","/api/stats",token,nil,200);if stats["total_checkins"]!=float64(0){t.Fatal("cascade deletion failed")}
 call("POST","/api/logout",token,nil,200);call("GET","/api/stats",token,nil,401)
 db.Exec("UPDATE sessions SET expires_at=UTC_TIMESTAMP()-INTERVAL 1 SECOND WHERE token_hash=?",hash(otherToken))
 call("GET","/api/plans",otherToken,nil,401)
}
