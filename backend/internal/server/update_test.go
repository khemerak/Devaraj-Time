package server

import (
	"context"
	"devaraj-time/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Test-only sessions isolate attendance engine testing from WebAuthn. Browser tests
// separately complete real signed ceremonies with a virtual CTAP2 authenticator.
func fixtureEmployeeSession(t *testing.T, db *pgxpool.Pool, cl *testClient, email string) {
	t.Helper()
	ctx := context.Background()
	token, csrf, credential := auth.Token(), auth.Token(), auth.Token()
	_, e := db.Exec(ctx, `INSERT INTO employee_credentials(id,company_id,membership_id,credential,rp_id) SELECT $1,m.company_id,m.id,'{}','localhost' FROM memberships m JOIN users u ON u.id=m.user_id WHERE u.email=$2`, credential, email)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(ctx, `INSERT INTO user_sessions(token_hash,user_id,company_id,csrf,credential_id,expires_at) SELECT $1,m.user_id,m.company_id,$2,$3,now()+interval '1 hour' FROM memberships m JOIN users u ON u.id=m.user_id WHERE u.email=$4`, auth.Digest(token), csrf, credential, email)
	if e != nil {
		t.Fatal(e)
	}
	cl.cookie = &http.Cookie{Name: "devaraj_session", Value: token}
	cl.csrf = csrf
}

func updateSecurityChecks(t *testing.T, db *pgxpool.Pool, router http.Handler, owner, other, platform *testClient, branch, shift, employeeID, password string) {
	ctx := context.Background()
	anon := &testClient{t: t, r: router}
	anon.expect("POST", "/auth/register", map[string]any{}, 404)
	anon.expect("POST", "/auth/login", map[string]any{"email": "employee@test.example", "password": password}, 401)
	anon.expect("POST", "/attendance", map[string]any{}, 401)
	owner.expect("GET", "/platform/companies", nil, 403)
	me := other.expect("GET", "/auth/me", nil, 200)
	if me["user"].(map[string]any)["email"] != "personal.admin@gmail.com" {
		t.Fatal("personal email not supported")
	}
	owner.expect("POST", "/admin/employees", map[string]any{"name": "HR manager", "email": "hr@test.example", "password": password, "employee_code": "HR-TEST", "role": "hr", "branch_id": branch, "shift_id": shift, "start_date": "2026-01-01", "active": true, "reason": "Authorized HR"}, 200)
	hr := &testClient{t: t, r: router}
	hr.expect("POST", "/auth/login", map[string]any{"email": "hr@test.example", "password": password}, 200)
	hr.refresh()
	hr.expect("GET", "/platform/companies", nil, 403)
	hr.expect("GET", "/platform/config", nil, 403)
	platform.expect("GET", "/platform/config", nil, 200)
	platform.expect("PATCH", "/platform/config", map[string]any{"invitation_hours": 12, "credential_days": 180, "support_email": "support@test.example", "reason": "Operator configuration test"}, 200)
	hr.expect("GET", "/admin/workplaces", nil, 403)
	shiftInput := map[string]any{"name": "HR night", "start_time": "22:00", "end_time": "06:00", "grace_minutes": 5, "expected_minutes": 420, "unpaid_break_minutes": 30, "early_window_minutes": 120, "workdays": []int{0, 1, 2, 3, 4, 5, 6}, "reason": "HR night schedule"}
	night := hr.expect("POST", "/admin/shifts", shiftInput, 200)["id"].(string)
	shiftInput["start_time"] = "21:00"
	hr.expect("PATCH", "/admin/shifts/"+night, shiftInput, 200)
	today := time.Now().UTC().Format("2006-01-02")
	tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	assignment := map[string]any{"membership_ids": []string{employeeID}, "shift_ids": []string{night, "rest", shift}, "effective_from": tomorrow, "effective_to": tomorrow, "reason": "Temporary rotation test"}
	other.expect("POST", "/schedules", assignment, 404)
	hr.expect("POST", "/schedules", assignment, 201)
	rows := owner.expect("GET", "/schedules?membership_id="+employeeID, nil, 200)["items"].([]any)
	if len(rows) < 2 {
		t.Fatal("history lost")
	}
	if _, e := db.Exec(ctx, `UPDATE schedule_assignments SET reason='overwrite' WHERE membership_id=$1`, employeeID); e == nil {
		t.Fatal("schedule history mutable")
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var i Identity
	e = tx.QueryRow(ctx, `SELECT id::text,company_id::text FROM memberships WHERE id=$1`, employeeID).Scan(&i.MembershipID, &i.CompanyID)
	i.Timezone = "UTC"
	p, e := schedulePolicy(ctx, tx, i, today)
	if e != nil || p.Start != "00:00" {
		t.Fatalf("future assignment rewrote today: %+v %v", p, e)
	}
	p, e = schedulePolicy(ctx, tx, i, tomorrow)
	if e != nil || p.Start != "21:00" {
		t.Fatalf("future assignment missing: %+v %v", p, e)
	}
	after := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
	p, e = schedulePolicy(ctx, tx, i, after)
	if e != nil || p.Start != "00:00" {
		t.Fatal("temporary override did not expire")
	}
	tx.Rollback(ctx)
	other.expect("POST", "/devices/invitations", map[string]any{"membership_id": employeeID, "reason": "Cross company denied"}, 404)
	invite := hr.expect("POST", "/devices/invitations", map[string]any{"membership_id": employeeID, "reason": "Verified employee for replacement"}, 201)["url"].(string)
	token := strings.Split(invite, "#")[1]
	begin := anon.expect("POST", "/auth/enroll/begin", map[string]any{"token": token}, 200)
	anon.expect("POST", "/auth/enroll/finish", map[string]any{"ceremony": begin["ceremony"], "response": map[string]any{}}, 401)
	anon.expect("POST", "/auth/enroll/finish", map[string]any{"ceremony": begin["ceremony"], "response": map[string]any{}}, 401)
	db.Exec(ctx, `UPDATE enrollment_invitations SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, auth.Digest(token))
	anon.expect("POST", "/auth/enroll/begin", map[string]any{"token": token}, 401)
	cl := &testClient{t: t, r: router}
	fixtureEmployeeSession(t, db, cl, "employee@test.example")
	cl.refresh()
	var cred string
	db.QueryRow(ctx, `SELECT credential_id FROM user_sessions WHERE token_hash=$1`, auth.Digest(cl.cookie.Value)).Scan(&cred)
	other.expect("POST", "/devices/"+cred+"/revoke", map[string]any{"reason": "Cross company denied"}, 404)
	hr.expect("POST", "/devices/"+cred+"/revoke", map[string]any{"reason": "Lost device reported"}, 200)
	cl.expect("GET", "/auth/me", nil, 401)
	fixtureEmployeeSession(t, db, cl, "employee@test.example")
	db.Exec(ctx, `UPDATE employee_credentials SET expires_at=now()-interval '1 second' WHERE id=(SELECT credential_id FROM user_sessions WHERE token_hash=$1)`, auth.Digest(cl.cookie.Value))
	cl.expect("GET", "/auth/me", nil, 401)
	recoveryTests(t, db, password)
	autoAttendanceTests(t, db, router, owner, branch, shift)
}

func autoAttendanceTests(t *testing.T, db *pgxpool.Pool, router http.Handler, owner *testClient, branch, shift string) {
	ctx := context.Background()
	member := owner.expect("POST", "/admin/employees", map[string]any{"name": "Auto scan tester", "email": "auto@test.example", "employee_code": "AUTO-TEST", "role": "employee", "branch_id": branch, "shift_id": shift, "start_date": "2026-01-01", "active": true, "reason": "Automatic scan regression"}, 200)["id"].(string)
	points := owner.expect("GET", "/admin/points", nil, 200)["items"].([]any)
	point := points[0].(map[string]any)
	cl := &testClient{t: t, r: router}
	fixtureEmployeeSession(t, db, cl, "auto@test.example")
	cl.refresh()
	payload := func() map[string]any {
		return map[string]any{"point": point["public_id"], "action": "auto", "idempotency_key": auth.Token(), "location": map[string]any{"latitude": 11.5564, "longitude": 104.9282, "accuracy": 10, "measured_at": time.Now().UTC()}}
	}
	record := cl.expect("POST", "/attendance", payload(), 201)
	if record["action"] != "check_in" {
		t.Fatal("auto did not check in")
	}
	repeated := cl.expect("POST", "/attendance", payload(), 200)
	if repeated["already_recorded"] != true {
		t.Fatal("rapid scan changed state")
	}
	// Use another employee with an older open session; raw event triggers remain enabled.
	second := owner.expect("POST", "/admin/employees", map[string]any{"name": "Auto checkout tester", "email": "autoout@test.example", "employee_code": "AUTO-OUT", "role": "employee", "branch_id": branch, "shift_id": shift, "start_date": "2026-01-01", "active": true, "reason": "Automatic checkout regression"}, 200)["id"].(string)
	var company string
	db.QueryRow(ctx, `SELECT company_id::text FROM memberships WHERE id=$1`, second).Scan(&company)
	db.Exec(ctx, `INSERT INTO attendance_sessions(company_id,membership_id,point_id,work_date,state,check_in,policy,status) SELECT company_id,$1,point_id,work_date,'checked_in',check_in-interval '5 minutes',policy,status FROM attendance_sessions WHERE membership_id=$2`, second, member)
	var session string
	db.QueryRow(ctx, `SELECT id::text FROM attendance_sessions WHERE membership_id=$1`, second).Scan(&session)
	_, e := db.Exec(ctx, `INSERT INTO attendance_events(company_id,membership_id,session_id,point_id,action,recorded_at,idempotency_key,request_hash,verification) VALUES($1,$2,$3,$4,'check_in',now()-interval '5 minutes',$5,'test','test')`, company, second, session, point["id"], auth.Token())
	if e != nil {
		t.Fatal(e)
	}
	other := &testClient{t: t, r: router}
	fixtureEmployeeSession(t, db, other, "autoout@test.example")
	other.refresh()
	out := other.expect("POST", "/attendance", payload(), 201)
	if out["action"] != "check_out" {
		t.Fatal("auto did not check out")
	}
	details := owner.expect("GET", "/attendance/"+session, nil, 200)
	if len(details["events"].([]any)) != 2 {
		t.Fatal("auto events lost")
	}
}

func recoveryTests(t *testing.T, db *pgxpool.Pool, password string) {
	messages := []string{}
	destinations := []string{}
	s := New(db, Config{Origin: "http://localhost:5173", ResetTTL: time.Minute, SendMail: func(to, subject, body string) error {
		destinations = append(destinations, to)
		messages = append(messages, body)
		return nil
	}})
	cl := &testClient{t: t, r: s.Router()}
	ctx := context.Background()
	cl.expect("POST", "/auth/recovery", map[string]any{"email": "unknown@test.example"}, 200)
	if len(messages) != 0 {
		t.Fatal("unknown recovery sent mail")
	}
	cl.expect("POST", "/auth/recovery", map[string]any{"email": "employee@test.example"}, 200)
	if len(messages) != 0 {
		t.Fatal("employee password bypass")
	}
	cl.expect("POST", "/auth/recovery", map[string]any{"email": "hr@test.example", "recovery_email": "attacker@test.example"}, 200)
	if len(destinations) != 1 || destinations[0] != "hr@test.example" {
		t.Fatal("reset redirected", destinations)
	}
	token := strings.Split(strings.Split(messages[0], "#")[1], "\n")[0]
	db.Exec(ctx, `UPDATE recovery_tokens SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, auth.Digest(token))
	cl.expect("POST", "/auth/reset", map[string]any{"token": token, "password": "replacement-password"}, 400)
	cl.expect("POST", "/auth/recovery", map[string]any{"email": "hr@test.example"}, 200)
	token = strings.Split(strings.Split(messages[len(messages)-1], "#")[1], "\n")[0]
	cl.expect("POST", "/auth/reset", map[string]any{"token": token, "password": "replacement-password"}, 200)
	cl.expect("POST", "/auth/reset", map[string]any{"token": token, "password": "another-password"}, 400)
	cl.expect("POST", "/auth/login", map[string]any{"email": "hr@test.example", "password": "replacement-password"}, 200)
	cl.refresh()
	cl.expect("POST", "/auth/recovery-email", map[string]any{"email": "verified.personal@gmail.com", "password": "replacement-password"}, 200)
	count := len(messages)
	verify := strings.Split(messages[count-1], "#")[1]
	cl.expect("POST", "/auth/recovery", map[string]any{"email": "verified.personal@gmail.com"}, 200)
	if len(messages) != count {
		t.Fatal("unverified destination accepted")
	}
	cl.expect("POST", "/auth/verify-recovery", map[string]any{"token": verify}, 200)
	db.Exec(ctx, `UPDATE recovery_tokens SET expires_at=now()-interval '1 second' WHERE purpose='password'`)
	cl.expect("POST", "/auth/recovery", map[string]any{"email": "verified.personal@gmail.com"}, 200)
	if destinations[len(destinations)-1] != "verified.personal@gmail.com" {
		t.Fatal("verified recovery failed")
	}
}
