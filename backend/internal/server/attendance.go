package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"devaraj-time/internal/attendance"
	"devaraj-time/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type clockInput struct {
	Point    string `json:"point" binding:"required,max=128"`
	Action   string `json:"action" binding:"required,oneof=auto check_in check_out start_break end_break"`
	Key      string `json:"idempotency_key" binding:"required,min=16,max=128"`
	Location *struct {
		Latitude  float64   `json:"latitude"`
		Longitude float64   `json:"longitude"`
		Accuracy  float64   `json:"accuracy"`
		Measured  time.Time `json:"measured_at"`
	} `json:"location"`
}

func (s *Server) point(c *gin.Context) {
	i := who(c)
	data, e := s.jsonRows(c, `SELECT jsonb_build_object('public_id',p.public_id,'name',p.name,'workplace',w.name,'address',w.address,'location_required',w.location_required,'radius_m',w.radius_m,'max_accuracy_m',w.max_accuracy_m) FROM attendance_points p JOIN workplaces w ON w.id=p.workplace_id JOIN memberships m ON m.id=$2 WHERE p.public_id=$1 AND p.company_id=$3 AND p.active AND w.active AND w.branch_id=m.branch_id`, c.Param("public"), i.MembershipID, i.CompanyID)
	if e != nil {
		problem(c, e)
		return
	}
	if len(data) == 0 {
		fail(c, 404, "QR is invalid, disabled, or outside your assigned branch")
		return
	}
	c.JSON(200, data[0])
}
func (s *Server) submit(c *gin.Context) {
	var in clockInput
	if !bind(c, &in) {
		return
	}
	i := who(c)
	ctx := c.Request.Context()
	payload, _ := json.Marshal(in)
	digest := auth.Digest(string(payload))
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	// Lock the membership before checking replay or state: all concurrent attendance actions serialize.
	var branch *string
	var active bool
	e = tx.QueryRow(ctx, `SELECT branch_id::text,active FROM memberships WHERE id=$1 AND company_id=$2 FOR UPDATE`, i.MembershipID, i.CompanyID).Scan(&branch, &active)
	if e != nil || !active {
		fail(c, 403, "Membership is inactive")
		return
	}
	if !validAttendanceCredential(c, tx, i) {
		return
	}
	var companyActive bool
	e = tx.QueryRow(ctx, `SELECT active FROM companies WHERE id=$1 FOR SHARE`, i.CompanyID).Scan(&companyActive)
	if e != nil || !companyActive {
		fail(c, 403, "Company access is disabled")
		return
	}
	var saved []byte
	var oldHash string
	e = tx.QueryRow(ctx, `SELECT jsonb_build_object('event_id',id,'session_id',session_id,'recorded_at',recorded_at,'action',action,'verification',verification),request_hash FROM attendance_events WHERE membership_id=$1 AND idempotency_key=$2`, i.MembershipID, in.Key).Scan(&saved, &oldHash)
	if e == nil {
		if oldHash != digest {
			fail(c, 409, "Idempotency key was already used for a different request")
			return
		}
		c.JSON(200, json.RawMessage(saved))
		return
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		problem(c, e)
		return
	}
	var point, workplace string
	var lat, lon, radius, accuracy float64
	var required bool
	e = tx.QueryRow(ctx, `SELECT p.id::text,w.name,w.latitude,w.longitude,w.radius_m,w.max_accuracy_m,w.location_required FROM attendance_points p JOIN workplaces w ON w.id=p.workplace_id WHERE p.public_id=$1 AND p.company_id=$2 AND p.active AND w.active AND w.branch_id=$3`, in.Point, i.CompanyID, branch).Scan(&point, &workplace, &lat, &lon, &radius, &accuracy, &required)
	if e != nil {
		fail(c, 404, "QR is invalid, disabled, or outside your assigned branch")
		return
	}
	var now time.Time
	if e = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); e != nil {
		problem(c, e)
		return
	}
	if in.Action == "auto" {
		var recent []byte
		err := tx.QueryRow(ctx, `SELECT jsonb_build_object('event_id',id,'session_id',session_id,'recorded_at',recorded_at,'action',action,'verification',verification,'already_recorded',true) FROM attendance_events WHERE membership_id=$1 AND point_id=$3 AND recorded_at>$2::timestamptz-interval '2 minutes' ORDER BY recorded_at DESC LIMIT 1`, i.MembershipID, now, point).Scan(&recent)
		if err == nil {
			c.JSON(200, json.RawMessage(recent))
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			problem(c, err)
			return
		}
	}
	verification := "location_not_required"
	var distance *float64
	if required {
		if in.Location == nil {
			fail(c, 422, "Location is required; allow access or submit an exception request")
			s.log(c, i.CompanyID, i.UserID, "attendance.rejected", point, "location unavailable")
			return
		}
		d, err := attendance.VerifyLocation(now, in.Location.Measured, in.Location.Latitude, in.Location.Longitude, in.Location.Accuracy, lat, lon, radius, accuracy)
		if err != nil {
			message := err.Error()
			if message == "outside the workplace boundary" {
				message = fmt.Sprintf("outside the workplace boundary (%d m away; allowed radius %d m)", int(d+0.5), int(radius+0.5))
			}
			fail(c, 422, message)
			s.log(c, i.CompanyID, i.UserID, "attendance.rejected", point, err.Error())
			return
		}
		distance = &d
		verification = "verification_passed"
	}
	var id, date, state string
	var checkIn time.Time
	var checkOut, breakStart *time.Time
	var breakSeconds int64
	var policyJSON []byte
	var p attendance.Policy
	e = tx.QueryRow(ctx, `SELECT id::text,work_date::text,state,check_in,check_out,break_started,break_seconds,policy FROM attendance_sessions WHERE membership_id=$1 AND state<>'checked_out' FOR UPDATE`, i.MembershipID).Scan(&id, &date, &state, &checkIn, &checkOut, &breakStart, &breakSeconds, &policyJSON)
	if errors.Is(e, pgx.ErrNoRows) {
		p, date, e = effectivePolicy(ctx, tx, i, now)
		if e != nil {
			fail(c, 422, "A shift assignment is required")
			return
		}
		state = ""
		checkIn = now
		policyJSON, _ = json.Marshal(p)
	} else if e == nil {
		e = json.Unmarshal(policyJSON, &p)
	}
	if e != nil {
		problem(c, e)
		return
	}
	if in.Action == "auto" {
		switch state {
		case "":
			in.Action = "check_in"
		case "checked_in":
			in.Action = "check_out"
		case "on_break":
			fail(c, 409, "End your break in self-service before checking out")
			return
		}
	}
	next, e := attendance.Transition(state, in.Action, p.Breaks)
	if e != nil {
		fail(c, 409, e.Error())
		return
	}
	if state == "" {
		if !attendance.Scheduled(date, p) {
			fail(c, 422, "This is a rest day; request an attendance exception")
			return
		}
		start, shiftEnd, err := attendance.Bounds(date, p)
		if err != nil {
			problem(c, err)
			return
		}
		if now.Before(start.Add(-time.Duration(p.EarlyWindow) * time.Minute)) {
			fail(c, 422, "The early check-in window has not opened")
			return
		}
		if now.After(shiftEnd) {
			fail(c, 422, "The assigned shift has ended; submit an attendance exception")
			return
		}
		var leave bool
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM requests WHERE membership_id=$1 AND kind='leave' AND status='approved' AND $2::date BETWEEN start_date AND end_date)`, i.MembershipID, date).Scan(&leave)
		if e != nil {
			problem(c, e)
			return
		}
		if leave {
			fail(c, 409, "Approved leave covers this date; contact HR to resolve the conflict")
			return
		}
		e = tx.QueryRow(ctx, `INSERT INTO attendance_sessions(company_id,membership_id,point_id,work_date,state,check_in,policy,status) VALUES($1,$2,$3,$4,'checked_in',$5,$6,'on_time') ON CONFLICT(membership_id,work_date) DO NOTHING RETURNING id::text`, i.CompanyID, i.MembershipID, point, date, now, policyJSON).Scan(&id)
		if errors.Is(e, pgx.ErrNoRows) {
			fail(c, 409, "Attendance already exists for this work date")
			return
		}
		if e != nil {
			problem(c, e)
			return
		}
	}
	switch in.Action {
	case "check_out":
		checkOut = &now
	case "start_break":
		breakStart = &now
	case "end_break":
		if breakStart == nil {
			problem(c, errors.New("break integrity failure"))
			return
		}
		breakSeconds += int64(now.Sub(*breakStart).Seconds())
		breakStart = nil
	}
	calc, e := attendance.Calculate(date, checkIn, checkOut, breakSeconds, p)
	if e != nil {
		problem(c, e)
		return
	}
	_, e = tx.Exec(ctx, `UPDATE attendance_sessions SET state=$1,check_out=$2,break_started=$3,break_seconds=$4,worked_minutes=$5,late_minutes=$6,arrival_delay_minutes=$7,early_minutes=$8,overtime_minutes=$9,status=$10,updated_at=$11 WHERE id=$12`, next, checkOut, breakStart, breakSeconds, calc.Worked, calc.Late, calc.ArrivalDelay, calc.Early, calc.Overtime, calc.Status, now, id)
	if e != nil {
		problem(c, e)
		return
	}
	var event string
	e = tx.QueryRow(ctx, `INSERT INTO attendance_events(company_id,membership_id,session_id,point_id,action,recorded_at,idempotency_key,request_hash,verification) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text`, i.CompanyID, i.MembershipID, id, point, in.Action, now, in.Key, digest, verification).Scan(&event)
	if e == nil && required {
		l := in.Location
		_, e = tx.Exec(ctx, `INSERT INTO event_locations(event_id,company_id,recorded_at,latitude,longitude,accuracy_m,measured_at,distance_m) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, event, i.CompanyID, now, l.Latitude, l.Longitude, l.Accuracy, l.Measured, distance)
	}
	if e == nil {
		e = audit(ctx, tx, i.CompanyID, i.UserID, "attendance."+in.Action, event, verification, c.GetString("request_id"))
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO notifications(company_id,membership_id,message) VALUES($1,$2,$3)`, i.CompanyID, i.MembershipID, "Attendance recorded: "+in.Action+" at "+workplace)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(201, gin.H{"event_id": event, "session_id": id, "recorded_at": now, "action": in.Action, "state": next, "status": calc.Status, "late_minutes": calc.Late, "verification": verification, "workplace": workplace})
}

// Scope is derived exclusively from the authenticated membership. Manager access is department limited.
func scope(i Identity) (bool, *string, string) { return i.Can("team"), i.DepartmentID, i.MembershipID }

const visibleSessions = ` FROM attendance_sessions a JOIN memberships m ON m.id=a.membership_id JOIN users u ON u.id=m.user_id WHERE a.company_id=$1 AND ($2::boolean OR m.id=$4::uuid) AND ($3::text IS NULL OR m.department_id::text=$3 OR m.id=$4::uuid)`

func departmentScope(i Identity) *string {
	if i.Role == "manager" {
		if i.DepartmentID == nil {
			v := "none"
			return &v
		}
		return i.DepartmentID
	}
	return nil
}
func dateRange(c *gin.Context) (string, string, bool) {
	from, to := c.DefaultQuery("from", time.Now().AddDate(0, -1, 0).Format("2006-01-02")), c.DefaultQuery("to", time.Now().Format("2006-01-02"))
	f, e1 := time.Parse("2006-01-02", from)
	t, e2 := time.Parse("2006-01-02", to)
	if e1 != nil || e2 != nil || t.Before(f) || t.Sub(f) > 366*24*time.Hour {
		fail(c, 400, "Use a valid date range of at most 366 days")
		return "", "", false
	}
	return from, to, true
}
func page(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
	return max(1, p), min(100, max(1, limit))
}
func (s *Server) history(c *gin.Context) {
	i := who(c)
	from, to, ok := dateRange(c)
	if !ok {
		return
	}
	p, limit := page(c)
	team := i.Can("team") && c.Query("mine") != "true"
	args := []any{i.CompanyID, team, departmentScope(i), i.MembershipID, from, to}
	query := `SELECT jsonb_build_object('id',a.id,'name',u.name,'employee_code',m.employee_code,'work_date',a.work_date,'state',a.state,'check_in',a.check_in,'check_out',a.check_out,'worked_minutes',a.worked_minutes,'late_minutes',a.late_minutes,'early_minutes',a.early_minutes,'overtime_minutes',a.overtime_minutes,'status',a.status,'adjusted',a.adjusted,'policy',a.policy)` + visibleSessions + ` AND a.work_date BETWEEN $5::date AND $6::date AND ($7='' OR a.status=$7) AND ($8='' OR u.name ILIKE '%'||$8||'%' OR m.employee_code ILIKE '%'||$8||'%') ORDER BY a.work_date DESC,u.name LIMIT $9 OFFSET $10`
	args = append(args, c.Query("status"), c.Query("search"), limit, (p-1)*limit)
	rows, e := s.jsonRows(c, query, args...)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows, "page": p, "limit": limit, "has_more": len(rows) == limit})
}
func (s *Server) details(c *gin.Context) {
	if !resourceID(c) {
		return
	}
	i := who(c)
	rows, e := s.jsonRows(c, `SELECT to_jsonb(a)||jsonb_build_object('name',u.name)`+visibleSessions+` AND a.id=$5`, i.CompanyID, i.Can("team"), departmentScope(i), i.MembershipID, c.Param("id"))
	if e != nil {
		problem(c, e)
		return
	}
	if len(rows) == 0 {
		fail(c, 404, "Attendance record not found")
		return
	}
	events, e := s.jsonRows(c, `SELECT jsonb_build_object('id',id,'action',action,'recorded_at',recorded_at,'verification',verification) FROM attendance_events WHERE session_id=$1 AND company_id=$2 ORDER BY recorded_at`, c.Param("id"), i.CompanyID)
	if e != nil {
		problem(c, e)
		return
	}
	corrections, e := s.jsonRows(c, `SELECT to_jsonb(r) FROM requests r JOIN attendance_sessions a ON a.membership_id=r.membership_id AND a.work_date=r.start_date WHERE a.id=$1 AND r.company_id=$2 AND r.kind='correction' ORDER BY r.created_at`, c.Param("id"), i.CompanyID)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"record": rows[0], "events": events, "corrections": corrections})
}
func (s *Server) dashboard(c *gin.Context) {
	i := who(c)
	loc, _ := time.LoadLocation(i.Timezone)
	today := time.Now().In(loc).Format("2006-01-02")
	rows, e := s.jsonRows(c, `SELECT jsonb_build_object('active',count(*),'checked_in',count(*) FILTER(WHERE a.state='checked_in'),'on_break',count(*) FILTER(WHERE a.state='on_break'),'checked_out',count(*) FILTER(WHERE a.state='checked_out'),'late',count(*) FILTER(WHERE a.late_minutes>0),'not_recorded',count(*) FILTER(WHERE a.id IS NULL AND NOT EXISTS(SELECT 1 FROM requests r WHERE r.membership_id=m.id AND r.kind='leave' AND r.status='approved' AND $5::date BETWEEN r.start_date AND r.end_date)),'on_leave',count(*) FILTER(WHERE EXISTS(SELECT 1 FROM requests r WHERE r.membership_id=m.id AND r.kind='leave' AND r.status='approved' AND $5::date BETWEEN r.start_date AND r.end_date))) FROM memberships m LEFT JOIN attendance_sessions a ON a.membership_id=m.id AND a.work_date=$5::date WHERE m.company_id=$1 AND m.active AND ($2::boolean OR m.id=$4::uuid) AND ($3::text IS NULL OR m.department_id::text=$3 OR m.id=$4::uuid)`, i.CompanyID, i.Can("team"), departmentScope(i), i.MembershipID, today)
	if e != nil {
		problem(c, e)
		return
	}
	trend, e := s.jsonRows(c, `SELECT jsonb_build_object('date',a.work_date,'present',count(*),'late',count(*) FILTER(WHERE a.late_minutes>0),'hours',round(sum(a.worked_minutes)::numeric/60,1))`+visibleSessions+` AND a.work_date >= $5::date-interval '6 days' GROUP BY a.work_date ORDER BY a.work_date`, i.CompanyID, i.Can("team"), departmentScope(i), i.MembershipID, today)
	if e != nil {
		problem(c, e)
		return
	}
	own, e := s.jsonRows(c, `SELECT to_jsonb(a) FROM attendance_sessions a WHERE membership_id=$1 AND (state<>'checked_out' OR work_date=$2::date) ORDER BY check_in DESC LIMIT 1`, i.MembershipID, today)
	if e != nil {
		problem(c, e)
		return
	}
	var pending int
	e = s.DB.QueryRow(c.Request.Context(), `SELECT count(*) FROM requests r JOIN memberships m ON m.id=r.membership_id WHERE r.company_id=$1 AND r.status='pending' AND ($2::boolean OR r.membership_id=$4::uuid) AND ($3::text IS NULL OR m.department_id::text=$3 OR m.id=$4::uuid)`, i.CompanyID, i.Can("team"), departmentScope(i), i.MembershipID).Scan(&pending)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"date": today, "metrics": rows[0], "trend": trend, "own": own, "pending": pending})
}
