package seed

import (
	"context"
	"fmt"
	"time"

	"devaraj-time/internal/attendance"
	"devaraj-time/internal/auth"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(ctx context.Context, db *pgxpool.Pool, password, timezone string) error {
	loc, e := time.LoadLocation(timezone)
	if e != nil {
		return e
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var exists bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email='admin@lotus.example')`).Scan(&exists); e != nil {
		return e
	}
	if exists {
		fmt.Println("Demo already provisioned; existing credentials were not changed.")
		return nil
	}
	hash := auth.Hash(password)
	for companyIndex, companyName := range []string{"Lotus Studio", "Riverstone Academy"} {
		var company, branch, branch2, department, department2, shift, night, workplace, point string
		e = tx.QueryRow(ctx, `INSERT INTO companies(name,timezone) VALUES($1,$2) RETURNING id::text`, companyName, timezone).Scan(&company)
		if e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO branches(company_id,name) VALUES($1,'Head office') RETURNING id::text`, company).Scan(&branch)
		if e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO branches(company_id,name) VALUES($1,'North campus') RETURNING id::text`, company).Scan(&branch2)
		if e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO departments(company_id,name) VALUES($1,'Operations') RETURNING id::text`, company).Scan(&department)
		if e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO departments(company_id,name) VALUES($1,'Design') RETURNING id::text`, company).Scan(&department2)
		if e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO shifts(company_id,name,start_time,end_time,expected_minutes,workdays) VALUES($1,'Day shift','08:00','17:00',480,ARRAY[0,1,2,3,4,5,6]) RETURNING id::text`, company).Scan(&shift)
		if e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO shifts(company_id,name,start_time,end_time,expected_minutes,workdays) VALUES($1,'Night shift','22:00','06:00',480,ARRAY[0,1,2,3,4,5,6]) RETURNING id::text`, company).Scan(&night)
		if e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO workplaces(company_id,branch_id,name,address,latitude,longitude,radius_m,max_accuracy_m) VALUES($1,$2,'Reception','Fictional demo workplace — set real coordinates before testing',11.5564,104.9282,150,75) RETURNING id::text`, company, branch).Scan(&workplace)
		if e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `INSERT INTO attendance_points(company_id,workplace_id,name,public_id) VALUES($1,$2,'Reception entrance',$3) RETURNING id::text`, company, workplace, auth.Token()).Scan(&point)
		if e != nil {
			return e
		}
		domain := "lotus"
		if companyIndex == 1 {
			domain = "riverstone"
		}
		roles := []string{"admin", "hr", "manager", "employee", "employee", "employee"}
		names := []string{"Arun Dara", "Maya Chen", "Sokha Lin", "Nara Kim", "Lina Park", "Ravi Sen"}
		for n, role := range roles {
			email := fmt.Sprintf("%s@%s.example", role, domain)
			if n > 3 {
				email = fmt.Sprintf("employee%d@%s.example", n-2, domain)
			}
			var user, member string
			e = tx.QueryRow(ctx, `INSERT INTO users(email,name,password_hash) VALUES($1,$2,$3) RETURNING id::text`, email, names[n], hash).Scan(&user)
			if e != nil {
				return e
			}
			dept := department
			if n == 5 {
				dept = department2
			}
			e = tx.QueryRow(ctx, `INSERT INTO memberships(company_id,user_id,role,department_id,branch_id,shift_id,employee_code,position) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text`, company, user, role, dept, branch, shift, fmt.Sprintf("EMP-%03d", n+1), "Team member").Scan(&member)
			if e != nil {
				return e
			}
			p := attendance.Policy{Version: 1, Timezone: timezone, ShiftName: "Day shift", Start: "08:00", End: "17:00", Grace: 10, Expected: 480, EarlyWindow: 120, Breaks: true, Workdays: []int{0, 1, 2, 3, 4, 5, 6}}
			policy, _ := json.Marshal(p)
			for days := 1; days <= 6; days++ {
				date := time.Now().In(loc).AddDate(0, 0, -days).Format("2006-01-02")
				start, end, _ := attendance.Bounds(date, p)
				if (n+days)%4 == 0 {
					start = start.Add(25 * time.Minute)
				}
				calc, _ := attendance.Calculate(date, start, &end, 3600, p)
				var session string
				e = tx.QueryRow(ctx, `INSERT INTO attendance_sessions(company_id,membership_id,point_id,work_date,state,check_in,check_out,break_seconds,policy,worked_minutes,late_minutes,arrival_delay_minutes,status) VALUES($1,$2,$3,$4,'checked_out',$5,$6,3600,$7,$8,$9,$10,$11) RETURNING id::text`, company, member, point, date, start, end, policy, calc.Worked, calc.Late, calc.ArrivalDelay, calc.Status).Scan(&session)
				if e != nil {
					return e
				}
				for a, t := range map[string]time.Time{"check_in": start, "start_break": start.Add(4 * time.Hour), "end_break": start.Add(5 * time.Hour), "check_out": end} {
					_, e = tx.Exec(ctx, `INSERT INTO attendance_events(company_id,membership_id,session_id,point_id,action,recorded_at,idempotency_key,request_hash,verification) VALUES($1,$2,$3,$4,$5,$6,$7,'development_seed','development_seed')`, company, member, session, point, a, t, auth.Token())
					if e != nil {
						return e
					}
				}
			}
			if n == 3 {
				date := time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
				start, end, _ := attendance.Bounds(date, p)
				_, e = tx.Exec(ctx, `INSERT INTO requests(company_id,membership_id,kind,issue,start_date,end_date,requested_in,requested_out,reason) VALUES($1,$2,'correction','Incorrect check-in',$3,$3,$4,$5,'Reception scanner was unavailable; please review my arrival time.')`, company, member, date, start, end)
				if e != nil {
					return e
				}
			}
			if n == 4 {
				date := time.Now().In(loc).AddDate(0, 0, 2).Format("2006-01-02")
				_, e = tx.Exec(ctx, `INSERT INTO requests(company_id,membership_id,kind,issue,start_date,end_date,reason,status,reviewer_id,review_reason,reviewed_at) VALUES($1,$2,'leave','Annual leave',$3,$3,'Planned family appointment','approved',(SELECT id FROM memberships WHERE company_id=$1 AND role='hr' LIMIT 1),'Approved development example',now())`, company, member, date)
				if e != nil {
					return e
				}
			}
		}
		_, e = tx.Exec(ctx, `INSERT INTO audit_logs(company_id,action,target,reason,request_id) VALUES($1,'development.seed',$2,'Fictional historical records; never production data','seed')`, company, company)
		if e != nil {
			return e
		}
	}
	if e = tx.Commit(ctx); e == nil {
		fmt.Println("Provisioned fictional demo companies. Accounts: admin, hr, manager, employee, employee2, employee3 @lotus.example and @riverstone.example. Password comes from DEMO_PASSWORD.")
	}
	return e
}
