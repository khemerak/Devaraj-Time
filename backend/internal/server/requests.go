package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"devaraj-time/internal/attendance"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (s *Server) requests(c *gin.Context) {
	i := who(c)
	p, limit := page(c)
	rows, e := s.jsonRows(c, `SELECT to_jsonb(r)||jsonb_build_object('name',u.name,'reviewer',ru.name) FROM requests r JOIN memberships m ON m.id=r.membership_id JOIN users u ON u.id=m.user_id LEFT JOIN memberships rm ON rm.id=r.reviewer_id LEFT JOIN users ru ON ru.id=rm.user_id WHERE r.company_id=$1 AND ($2::boolean OR m.id=$4::uuid) AND ($3::text IS NULL OR m.department_id::text=$3 OR m.id=$4::uuid) AND ($5='' OR r.kind=$5) ORDER BY r.created_at DESC LIMIT $6 OFFSET $7`, i.CompanyID, i.Can("team") && c.Query("mine") != "true", departmentScope(i), i.MembershipID, c.Query("kind"), limit, (p-1)*limit)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows, "page": p, "has_more": len(rows) == limit})
}
func (s *Server) createRequest(c *gin.Context) {
	var in struct {
		Kind   string     `json:"kind" binding:"required,oneof=correction leave"`
		Issue  string     `json:"issue" binding:"required,max=120"`
		Start  string     `json:"start_date" binding:"required,datetime=2006-01-02"`
		End    string     `json:"end_date" binding:"required,datetime=2006-01-02"`
		In     *time.Time `json:"requested_in"`
		Out    *time.Time `json:"requested_out"`
		Reason string     `json:"reason" binding:"required,min=10,max=2000"`
	}
	if !bind(c, &in) {
		return
	}
	i := who(c)
	if in.End < in.Start {
		fail(c, 400, "End date must follow start date")
		return
	}
	start, _ := time.Parse("2006-01-02", in.Start)
	end, _ := time.Parse("2006-01-02", in.End)
	if end.Sub(start) > 366*24*time.Hour {
		fail(c, 400, "Request cannot exceed one year")
		return
	}
	if in.Kind == "correction" {
		if in.Start != in.End || in.In == nil || in.Out == nil || !in.Out.After(*in.In) || in.Out.Sub(*in.In) > 24*time.Hour || in.Out.After(time.Now()) {
			fail(c, 400, "Corrections require a single date and a past check-in/check-out interval of at most 24 hours")
			return
		}
		var days int
		if e := s.DB.QueryRow(c.Request.Context(), `SELECT correction_days FROM companies WHERE id=$1`, i.CompanyID).Scan(&days); e != nil {
			problem(c, e)
			return
		}
		loc, _ := time.LoadLocation(i.Timezone)
		today, _ := time.Parse("2006-01-02", time.Now().In(loc).Format("2006-01-02"))
		if start.Before(today.AddDate(0, 0, -days)) || start.After(today) {
			fail(c, 422, "Date is outside the company correction deadline")
			return
		}
	}
	ctx := c.Request.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	var id string
	e = tx.QueryRow(ctx, `INSERT INTO requests(company_id,membership_id,kind,issue,start_date,end_date,requested_in,requested_out,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text`, i.CompanyID, i.MembershipID, in.Kind, in.Issue, in.Start, in.End, in.In, in.Out, in.Reason).Scan(&id)
	if e == nil {
		e = audit(ctx, tx, i.CompanyID, i.UserID, "request.submitted", id, in.Reason, c.GetString("request_id"))
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO notifications(company_id,membership_id,message) VALUES($1,$2,$3)`, i.CompanyID, i.MembershipID, "Your "+in.Kind+" request was submitted for review")
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(201, gin.H{"id": id})
}
func (s *Server) review(c *gin.Context) {
	if !resourceID(c) || !require(c, "review") {
		return
	}
	var in struct {
		Decision string `json:"decision" binding:"required,oneof=approved rejected"`
		Reason   string `json:"reason" binding:"required,min=10,max=2000"`
	}
	if !bind(c, &in) {
		return
	}
	i := who(c)
	ctx := c.Request.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	var member, kind, start, end, status string
	var requestedIn, requestedOut *time.Time
	e = tx.QueryRow(ctx, `SELECT r.membership_id::text,r.kind,r.start_date::text,r.end_date::text,r.status,r.requested_in,r.requested_out FROM requests r JOIN memberships m ON m.id=r.membership_id WHERE r.id=$1 AND r.company_id=$2 AND ($3::text IS NULL OR m.department_id::text=$3) FOR UPDATE OF r`, c.Param("id"), i.CompanyID, departmentScope(i)).Scan(&member, &kind, &start, &end, &status, &requestedIn, &requestedOut)
	if e != nil {
		fail(c, 404, "Request not found within your authorized scope")
		return
	}
	if member == i.MembershipID {
		fail(c, 403, "You cannot review your own request")
		return
	}
	if status != "pending" {
		fail(c, 409, "This request has already been decided")
		return
	}
	var locked string
	if e = tx.QueryRow(ctx, `SELECT id::text FROM memberships WHERE id=$1 FOR UPDATE`, member).Scan(&locked); e != nil {
		problem(c, e)
		return
	}
	if in.Decision == "approved" {
		if kind == "leave" {
			var conflict bool
			e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM attendance_sessions WHERE membership_id=$1 AND work_date BETWEEN $2::date AND $3::date) OR EXISTS(SELECT 1 FROM requests WHERE membership_id=$1 AND kind='leave' AND status='approved' AND start_date <= $3::date AND end_date >= $2::date)`, member, start, end).Scan(&conflict)
			if e != nil {
				problem(c, e)
				return
			}
			if conflict {
				fail(c, 409, "Leave overlaps attendance or approved leave; resolve the conflict first")
				return
			}
		} else {
			var id, state string
			var data []byte
			var breaks int64
			var breakStarted *time.Time
			var p attendance.Policy
			e = tx.QueryRow(ctx, `SELECT id::text,state,policy,break_seconds,break_started FROM attendance_sessions WHERE membership_id=$1 AND work_date=$2::date FOR UPDATE`, member, start).Scan(&id, &state, &data, &breaks, &breakStarted)
			if e == nil {
				e = json.Unmarshal(data, &p)
			} else if errors.Is(e, pgx.ErrNoRows) {
				employee := i
				employee.MembershipID = member
				p, e = schedulePolicy(ctx, tx, employee, start)
			}
			if e != nil || requestedIn == nil || requestedOut == nil {
				fail(c, 422, "Correction has no valid shift or requested interval")
				return
			}
			workDate, err := attendance.WorkDate(*requestedIn, p)
			if err != nil || workDate != start {
				fail(c, 422, "Requested check-in does not match the shift work date")
				return
			}
			if state == "on_break" && breakStarted != nil {
				if requestedOut.Before(*breakStarted) {
					fail(c, 422, "Requested checkout precedes the recorded open break")
					return
				}
				breaks += int64(requestedOut.Sub(*breakStarted).Seconds())
			}
			if requestedOut.Sub(*requestedIn).Seconds() < float64(breaks) {
				fail(c, 422, "Requested interval is shorter than recorded breaks")
				return
			}
			var onLeave bool
			e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM requests WHERE membership_id=$1 AND kind='leave' AND status='approved' AND $2::date BETWEEN start_date AND end_date)`, member, start).Scan(&onLeave)
			if e != nil {
				problem(c, e)
				return
			}
			if onLeave {
				fail(c, 409, "Approved leave conflicts with this correction")
				return
			}
			calc, err := attendance.Calculate(start, *requestedIn, requestedOut, breaks, p)
			if err != nil {
				problem(c, err)
				return
			}
			if id == "" {
				_, e = tx.Exec(ctx, `INSERT INTO attendance_sessions(company_id,membership_id,work_date,state,check_in,check_out,policy,worked_minutes,late_minutes,arrival_delay_minutes,early_minutes,overtime_minutes,status,adjusted) VALUES($1,$2,$3,'checked_out',$4,$5,$6,$7,$8,$9,$10,$11,$12,true)`, i.CompanyID, member, start, requestedIn, requestedOut, marshal(p), calc.Worked, calc.Late, calc.ArrivalDelay, calc.Early, calc.Overtime, calc.Status)
			} else {
				_, e = tx.Exec(ctx, `UPDATE attendance_sessions SET check_in=$1,check_out=$2,worked_minutes=$3,late_minutes=$4,arrival_delay_minutes=$5,early_minutes=$6,overtime_minutes=$7,status=$8,adjusted=true,state='checked_out',break_started=NULL,break_seconds=$10,updated_at=now() WHERE id=$9`, requestedIn, requestedOut, calc.Worked, calc.Late, calc.ArrivalDelay, calc.Early, calc.Overtime, calc.Status, id, breaks)
			}
			if e != nil {
				problem(c, e)
				return
			}
		}
	}
	_, e = tx.Exec(ctx, `UPDATE requests SET status=$1,reviewer_id=$2,review_reason=$3,reviewed_at=now() WHERE id=$4`, in.Decision, i.MembershipID, in.Reason, c.Param("id"))
	if e == nil {
		e = audit(ctx, tx, i.CompanyID, i.UserID, "request."+in.Decision, c.Param("id"), in.Reason, c.GetString("request_id"))
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO notifications(company_id,membership_id,message) VALUES($1,$2,$3)`, i.CompanyID, member, "Your "+kind+" request was "+in.Decision+": "+in.Reason)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (s *Server) cancelRequest(c *gin.Context) {
	if !resourceID(c) {
		return
	}
	i := who(c)
	s.transaction(c, func(ctx context.Context, tx pgx.Tx) error {
		var id string
		e := tx.QueryRow(ctx, `UPDATE requests SET status='cancelled' WHERE id=$1 AND company_id=$2 AND membership_id=$3 AND status='pending' RETURNING id::text`, c.Param("id"), i.CompanyID, i.MembershipID).Scan(&id)
		if e != nil {
			return e
		}
		return audit(ctx, tx, i.CompanyID, i.UserID, "request.cancelled", id, "Employee cancelled request", c.GetString("request_id"))
	})
}
