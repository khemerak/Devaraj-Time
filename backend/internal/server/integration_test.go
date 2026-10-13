package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"devaraj-time/internal/auth"
	"devaraj-time/internal/database"
	"devaraj-time/internal/seed"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testClient struct {
	t      *testing.T
	r      http.Handler
	cookie *http.Cookie
	csrf   string
}

func (cl *testClient) call(method, path string, body any) (int, map[string]any, []byte) {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, "/api"+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("X-CSRF-Token", cl.csrf)
	if cl.cookie != nil {
		req.AddCookie(cl.cookie)
	}
	w := httptest.NewRecorder()
	cl.r.ServeHTTP(w, req)
	for _, cookie := range w.Result().Cookies() {
		cl.cookie = cookie
	}
	raw := w.Body.Bytes()
	obj := map[string]any{}
	_ = json.Unmarshal(raw, &obj)
	return w.Code, obj, raw
}
func (cl *testClient) expect(method, path string, body any, want int) map[string]any {
	cl.t.Helper()
	status, res, raw := cl.call(method, path, body)
	if status != want {
		cl.t.Fatalf("%s %s: want %d got %d %s", method, path, want, status, raw)
	}
	return res
}
func (cl *testClient) refresh() {
	me := cl.expect("GET", "/auth/me", nil, 200)
	cl.csrf = me["user"].(map[string]any)["csrf"].(string)
}
func TestDatabaseWorkflow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("Set TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := "test_" + strings.ToLower(strings.ReplaceAll(auth.Token()[:16], "-", "a"))
	schema = strings.ReplaceAll(schema, "_", "a")
	if _, e = admin.Exec(ctx, `CREATE SCHEMA `+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	migration, e := os.ReadFile(filepath.Join("..", "..", "migrations", "000001_init.up.sql"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, string(migration)); e != nil {
		t.Fatal(e)
	}
	privateMigration, e := os.ReadFile(filepath.Join("..", "..", "migrations", "000002_private_location.up.sql"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, string(privateMigration)); e != nil {
		t.Fatal(e)
	}
	updateMigration, e := os.ReadFile(filepath.Join("..", "..", "migrations", "000003_enrollment_schedules.up.sql"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, string(updateMigration)); e != nil {
		t.Fatal(e)
	}
	recoveryMigration, e := os.ReadFile(filepath.Join("..", "..", "migrations", "000004_verified_recovery_email.up.sql"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, string(recoveryMigration)); e != nil {
		t.Fatal(e)
	}
	platformMigration, e := os.ReadFile(filepath.Join("..", "..", "migrations", "000005_platform_configuration.up.sql"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, string(platformMigration)); e != nil {
		t.Fatal(e)
	}
	router := New(db, Config{Origin: "http://localhost:5173"}).Router()
	owner := &testClient{t: t, r: router}
	other := &testClient{t: t, r: router}
	password := "test-only-long-password"
	platform := &testClient{t: t, r: router}
	_, e = db.Exec(ctx, `INSERT INTO users(email,name,password_hash,platform_admin) VALUES('platform@test.example','Platform',$1,true)`, auth.Hash(password))
	if e != nil {
		t.Fatal(e)
	}
	platform.expect("POST", "/auth/login", map[string]any{"email": "platform@test.example", "password": password}, 200)
	platform.refresh()
	platform.expect("POST", "/platform/companies", map[string]any{"company": "Company A", "name": "Owner A", "email": "a@test.example", "password": password, "timezone": "UTC"}, 201)
	owner.expect("POST", "/auth/login", map[string]any{"email": "a@test.example", "password": password}, 200)
	owner.refresh()
	platform.expect("POST", "/platform/companies", map[string]any{"company": "Company B", "name": "Owner B", "email": "personal.admin@gmail.com", "password": password, "timezone": "UTC"}, 201)
	other.expect("POST", "/auth/login", map[string]any{"email": "personal.admin@gmail.com", "password": password}, 200)
	other.refresh()
	refs := owner.expect("GET", "/references", nil, 200)
	branch := refs["branches"].([]any)[0].(map[string]any)["id"].(string)
	shift := owner.expect("POST", "/admin/shifts", map[string]any{"name": "All days", "start_time": "00:00", "end_time": "23:59", "grace_minutes": 10, "expected_minutes": 480, "early_window_minutes": 720, "unpaid_break_minutes": 0, "overtime_enabled": true, "workdays": []int{0, 1, 2, 3, 4, 5, 6}, "reason": "Integration schedule"}, 200)["id"].(string)
	workplace := owner.expect("POST", "/admin/workplaces", map[string]any{"name": "Reception", "branch_id": branch, "address": "Test site", "latitude": 11.5564, "longitude": 104.9282, "radius_m": 150, "max_accuracy_m": 75, "location_required": true, "active": true, "reason": "Integration workplace"}, 200)["id"].(string)
	pointID := owner.expect("POST", "/admin/points", map[string]any{"name": "Door", "workplace_id": workplace, "active": true, "reason": "Integration point"}, 200)["id"].(string)
	points := owner.expect("GET", "/admin/points", nil, 200)
	point := points["items"].([]any)[0].(map[string]any)["public_id"].(string)
	createEmployee := func(email string) string {
		return owner.expect("POST", "/admin/employees", map[string]any{"name": "Test employee", "email": email, "password": password, "employee_code": email, "role": "employee", "branch_id": branch, "shift_id": shift, "start_date": "2026-01-01", "active": true, "reason": "Integration employee"}, 200)["id"].(string)
	}
	employeeID := createEmployee("employee@test.example")
	createEmployee("concurrent@test.example")
	departmentID := owner.expect("POST", "/admin/departments", map[string]any{"name": "Managers department", "reason": "Department access boundary test"}, 200)["id"].(string)
	owner.expect("POST", "/admin/employees", map[string]any{"name": "Scoped manager", "email": "manager@test.example", "password": password, "employee_code": "MANAGER-001", "role": "manager", "department_id": departmentID, "branch_id": branch, "shift_id": shift, "start_date": "2026-01-01", "active": true, "reason": "Department manager access test"}, 200)
	manager := &testClient{t: t, r: router}
	fixtureEmployeeSession(t, db, manager, "manager@test.example")
	manager.refresh()
	employee := &testClient{t: t, r: router}
	fixtureEmployeeSession(t, db, employee, "employee@test.example")
	employee.refresh()
	t.Run("role and tenant protection", func(t *testing.T) {
		employee.expect("GET", "/admin/employees", nil, 403)
		employee.expect("GET", "/audit", nil, 403)
		employee.expect("GET", "/reports", nil, 403)
		other.expect("GET", "/points/"+point, nil, 404)
		other.expect("POST", "/admin/points/"+pointID+"/rotate", map[string]any{"reason": "Cross tenant denied"}, 409)
		other.expect("PATCH", "/admin/workplaces/"+workplace, map[string]any{"name": "Hijack", "branch_id": branch, "latitude": 0, "longitude": 0, "radius_m": 100, "max_accuracy_m": 50, "active": true, "reason": "Cross tenant denied"}, 409)
		data := other.expect("GET", "/attendance", nil, 200)
		if len(data["items"].([]any)) != 0 {
			t.Fatal("tenant records leaked")
		}
		old := employee.csrf
		employee.csrf = "bad"
		employee.expect("POST", "/attendance", map[string]any{}, 403)
		employee.csrf = old
	})
	location := map[string]any{"latitude": 11.5564, "longitude": 104.9282, "accuracy": 10, "measured_at": time.Now().UTC().Format(time.RFC3339Nano)}
	payload := func(action string) map[string]any {
		return map[string]any{"point": point, "action": action, "idempotency_key": auth.Token(), "location": location}
	}
	t.Run("verification and transitions", func(t *testing.T) {
		bad := payload("check_in")
		bad["location"] = map[string]any{"latitude": 0, "longitude": 0, "accuracy": 10, "measured_at": time.Now().UTC()}
		employee.expect("POST", "/attendance", bad, 422)
		employee.expect("POST", "/attendance", payload("check_out"), 409)
	})
	in := payload("check_in")
	record := employee.expect("POST", "/attendance", in, 201)
	session := record["session_id"].(string)
	employee.expect("POST", "/attendance", in, 200)
	in["action"] = "check_out"
	employee.expect("POST", "/attendance", in, 409)
	employee.expect("POST", "/attendance", payload("check_in"), 409)
	employee.expect("POST", "/attendance", payload("start_break"), 201)
	employee.expect("POST", "/attendance", payload("check_out"), 409)
	employee.expect("POST", "/attendance", payload("end_break"), 201)
	employee.expect("POST", "/attendance", payload("check_out"), 201)
	employee.expect("POST", "/attendance", payload("check_out"), 409)
	other.expect("GET", "/attendance/"+session, nil, 404)
	manager.expect("GET", "/attendance/"+session, nil, 404)
	t.Run("correction preserves original events", func(t *testing.T) {
		now := time.Now().UTC()
		date := now.Format("2006-01-02")
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		end := now.Add(-time.Second)
		if end.Before(start) {
			t.Skip("midnight interval")
		}
		request := employee.expect("POST", "/requests", map[string]any{"kind": "correction", "issue": "Incorrect attendance calculation", "start_date": date, "end_date": date, "requested_in": start, "requested_out": end, "reason": "Please review the scanner malfunction."}, 201)["id"].(string)
		other.expect("POST", "/requests/"+request+"/review", map[string]any{"decision": "approved", "reason": "Cross company must be denied"}, 404)
		manager.expect("POST", "/requests/"+request+"/review", map[string]any{"decision": "approved", "reason": "Cross department must be denied"}, 404)
		employee.expect("POST", "/requests/"+request+"/review", map[string]any{"decision": "approved", "reason": "Self approval must be denied"}, 403)
		owner.expect("POST", "/requests/"+request+"/review", map[string]any{"decision": "approved", "reason": "Reviewed supporting details with employee"}, 200)
		owner.expect("POST", "/requests/"+request+"/review", map[string]any{"decision": "rejected", "reason": "Duplicate decision must be denied"}, 409)
		d := employee.expect("GET", "/attendance/"+session, nil, 200)
		if len(d["events"].([]any)) != 4 || !d["record"].(map[string]any)["adjusted"].(bool) {
			t.Fatal("raw events overwritten")
		}
		if _, e = db.Exec(ctx, `UPDATE attendance_events SET action='check_out' WHERE session_id=$1`, session); e == nil {
			t.Fatal("immutable events editable")
		}
		if _, e = db.Exec(ctx, `DELETE FROM audit_logs WHERE company_id=(SELECT company_id FROM memberships WHERE id=$1)`, employeeID); e == nil {
			t.Fatal("audit deletable")
		}
	})
	t.Run("concurrent submissions", func(t *testing.T) {
		cl := &testClient{t: t, r: router}
		fixtureEmployeeSession(t, db, cl, "concurrent@test.example")
		cl.refresh()
		codes := make(chan int, 8)
		var wg sync.WaitGroup
		for n := 0; n < 8; n++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				code, _, _ := cl.call("POST", "/attendance", payload("check_in"))
				codes <- code
			}()
		}
		wg.Wait()
		close(codes)
		success := 0
		for code := range codes {
			if code == 201 {
				success++
			} else if code != 409 {
				t.Fatalf("unexpected status %d", code)
			}
		}
		if success != 1 {
			t.Fatalf("%d check-ins committed", success)
		}
	})
	t.Run("leave review and exports", func(t *testing.T) {
		date := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
		id := employee.expect("POST", "/requests", map[string]any{"kind": "leave", "issue": "Annual leave", "start_date": date, "end_date": date, "reason": "Planned family appointment for testing"}, 201)["id"].(string)
		owner.expect("POST", "/requests/"+id+"/review", map[string]any{"decision": "approved", "reason": "Leave approved after schedule review"}, 200)
		overlap := employee.expect("POST", "/requests", map[string]any{"kind": "leave", "issue": "Personal leave", "start_date": date, "end_date": date, "reason": "Overlapping leave request must not be approved"}, 201)["id"].(string)
		owner.expect("POST", "/requests/"+overlap+"/review", map[string]any{"decision": "approved", "reason": "Conflict should be rejected by the policy"}, 409)
		workDate := time.Now().UTC().Format("2006-01-02")
		conflict := employee.expect("POST", "/requests", map[string]any{"kind": "leave", "issue": "Other", "start_date": workDate, "end_date": workDate, "reason": "Attendance overlap must remain unresolved"}, 201)["id"].(string)
		owner.expect("POST", "/requests/"+conflict+"/review", map[string]any{"decision": "approved", "reason": "Attendance conflict should prevent approval"}, 409)
		_, _, csv := owner.call("GET", "/reports?from="+time.Now().UTC().Format("2006-01-02")+"&to="+date, nil)
		if !strings.Contains(string(csv), "Effective check-in") {
			t.Fatal("CSV missing fields")
		}
		code, _, xlsx := owner.call("GET", "/reports?format=xlsx&from="+time.Now().UTC().Format("2006-01-02")+"&to="+date, nil)
		if code != 200 || !bytes.HasPrefix(xlsx, []byte("PK")) {
			t.Fatal("XLSX invalid")
		}
	})
	owner.expect("POST", "/admin/points/"+pointID+"/rotate", map[string]any{"reason": "Replace compromised QR code"}, 200)
	employee.expect("GET", "/points/"+point, nil, 404)
	t.Run("expired session", func(t *testing.T) {
		_, e := db.Exec(ctx, `UPDATE user_sessions SET expires_at=now()-interval '1 minute' WHERE token_hash=$1`, auth.Digest(employee.cookie.Value))
		if e != nil {
			t.Fatal(e)
		}
		employee.expect("GET", "/auth/me", nil, 401)
	})
	t.Run("forgotten checkout resolved without fabricated scan", func(t *testing.T) {
		cl := &testClient{t: t, r: router}
		fixtureEmployeeSession(t, db, cl, "concurrent@test.example")
		cl.refresh()
		now := time.Now().UTC()
		date := now.Format("2006-01-02")
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		end := now.Add(-time.Millisecond)
		id := cl.expect("POST", "/requests", map[string]any{"kind": "correction", "issue": "Forgot check-out", "start_date": date, "end_date": date, "requested_in": start, "requested_out": end, "reason": "Forgot to check out after leaving the workplace."}, 201)["id"].(string)
		owner.expect("POST", "/requests/"+id+"/review", map[string]any{"decision": "approved", "reason": "Reviewed the forgotten checkout with employee"}, 200)
		history := cl.expect("GET", "/attendance", nil, 200)
		record := history["items"].([]any)[0].(map[string]any)
		if record["state"] != "checked_out" || record["adjusted"] != true {
			t.Fatal("session remains stuck", record)
		}
		details := cl.expect("GET", "/attendance/"+record["id"].(string), nil, 200)
		if len(details["events"].([]any)) != 1 {
			t.Fatal("fabricated checkout event")
		}
	})
	t.Run("own profile and password change revokes every session", func(t *testing.T) {
		updateSecurityChecks(t, db, router, owner, other, platform, branch, shift, employeeID, password)
		second := &testClient{t: t, r: router}
		second.expect("POST", "/auth/login", map[string]any{"email": "a@test.example", "password": password}, 200)
		second.refresh()
		owner.expect("PATCH", "/auth/profile", map[string]any{"name": "Updated owner"}, 200)
		owner.expect("POST", "/auth/password", map[string]any{"current_password": "incorrect", "new_password": "new-test-only-password"}, 403)
		owner.expect("POST", "/auth/password", map[string]any{"current_password": password, "new_password": "new-test-only-password"}, 200)
		owner.expect("GET", "/auth/me", nil, 401)
		second.expect("GET", "/auth/me", nil, 401)
		second.expect("POST", "/auth/login", map[string]any{"email": "a@test.example", "password": "new-test-only-password"}, 200)
	})
	fmt.Println("Verified authentication, tenancy, RBAC, CSRF, geofence, replay, concurrency, corrections, leave, export and immutability against PostgreSQL")
	t.Run("repeatable demo seed", func(t *testing.T) {
		if e := seed.Run(ctx, db, password, "Asia/Phnom_Penh"); e != nil {
			t.Fatal(e)
		}
		if e := seed.Run(ctx, db, password, "Asia/Phnom_Penh"); e != nil {
			t.Fatal(e)
		}
		var count int
		if e := db.QueryRow(ctx, `SELECT count(*) FROM users WHERE email LIKE '%@lotus.example' OR email LIKE '%@riverstone.example'`).Scan(&count); e != nil || count != 12 {
			t.Fatal(count, e)
		}
	})
	t.Run("precise location erasure preserves clock events", func(t *testing.T) {
		_, e := db.Exec(ctx, `UPDATE event_locations SET recorded_at=now()-interval '400 days'`)
		if e != nil {
			t.Fatal(e)
		}
		if e = database.PurgeLocation(ctx, db); e != nil {
			t.Fatal(e)
		}
		var count int
		if e = db.QueryRow(ctx, `SELECT count(*) FROM event_locations`).Scan(&count); e != nil || count != 0 {
			t.Fatal(count, e)
		}
		if e = db.QueryRow(ctx, `SELECT count(*) FROM attendance_events WHERE session_id=$1`, session).Scan(&count); e != nil || count != 4 {
			t.Fatal("clock events erased", count, e)
		}
	})
}
