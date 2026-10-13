package server

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRBAC(t *testing.T) {
	if (Identity{Role: "employee"}).Can("review") {
		t.Fatal("employee can review")
	}
	if !(Identity{Role: "manager"}).Can("review") || (Identity{Role: "manager"}).Can("configure") {
		t.Fatal("manager permissions")
	}
	i := Identity{Role: "manager"}
	if departmentScope(i) == nil {
		t.Fatal("unassigned manager can read all departments")
	}
	if departmentScope(Identity{Role: "hr"}) != nil {
		t.Fatal("HR unexpectedly limited")
	}
}
func TestOriginAndSessionGate(t *testing.T) {
	s := New(nil, Config{Origin: "http://localhost:5173"})
	r := s.Router()
	for _, tc := range []struct {
		method, path, origin string
		code                 int
	}{{"POST", "/api/auth/login", "https://attacker.example", 403}, {"GET", "/api/attendance", "", 401}, {"POST", "/api/attendance", "", 401}} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatalf("%s %s got %d", tc.method, tc.path, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("sensitive response cacheable")
		}
	}
}
func TestCookieAndConfig(t *testing.T) {
	s := New(nil, Config{Secure: true})
	r := s.Router()
	r.GET("/cookie", func(c *gin.Context) { s.cookie(c, "example", 43200); c.Status(200) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/cookie", nil))
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe cookie")
	}
	if ValidateConfig(Config{Origin: "http://example.com", Secure: true, Production: true}) == nil {
		t.Fatal("insecure production allowed")
	}
}
func TestSafeExport(t *testing.T) {
	for _, v := range []string{"=CMD()", " +1", "-2", "@SUM(A1)", "\tbad"} {
		if !strings.HasPrefix(safeCell(v), "'") {
			t.Fatal("unsafe cell", v)
		}
	}
	if safeCell("Employee") != "Employee" {
		t.Fatal("safe cell modified")
	}
}
