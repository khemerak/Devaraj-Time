package server

import (
	"context"
	"devaraj-time/internal/attendance"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"time"
)

type shiftSnapshot struct {
	Name     string `json:"name"`
	Start    string `json:"start_time"`
	End      string `json:"end_time"`
	Grace    int    `json:"grace_minutes"`
	Expected int    `json:"expected_minutes"`
	Unpaid   int    `json:"unpaid_break_minutes"`
	Early    int    `json:"early_window_minutes"`
	Overtime bool   `json:"overtime_enabled"`
	Workdays []int  `json:"workdays"`
}

func (s *Server) schedules(c *gin.Context) {
	i := who(c)
	member := c.Query("membership_id")
	if !i.Can("schedules") {
		member = i.MembershipID
	}
	if member != "" && !validUUID(member) {
		fail(c, 400, "Invalid membership")
		return
	}
	rows, e := s.jsonRows(c, `SELECT to_jsonb(sa)||jsonb_build_object('employee',u.name,'actor',a.name) FROM schedule_assignments sa JOIN memberships m ON m.id=sa.membership_id JOIN users u ON u.id=m.user_id LEFT JOIN users a ON a.id=sa.actor_id WHERE sa.company_id=$1 AND ($2='' OR sa.membership_id::text=$2) ORDER BY sa.created_at DESC LIMIT 500`, i.CompanyID, member)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows})
}
func (s *Server) assignSchedules(c *gin.Context) {
	if !require(c, "schedules") {
		return
	}
	i := who(c)
	var in struct {
		Members []string `json:"membership_ids" binding:"required,min=1,max=200,dive,uuid"`
		Shifts  []string `json:"shift_ids" binding:"required,min=1,max=28"`
		From    string   `json:"effective_from" binding:"required,datetime=2006-01-02"`
		To      *string  `json:"effective_to"`
		Reason  string   `json:"reason" binding:"required,min=3,max=1000"`
	}
	if !bind(c, &in) {
		return
	}
	loc, _ := time.LoadLocation(i.Timezone)
	today := time.Now().In(loc).Format("2006-01-02")
	if in.From < today {
		fail(c, 422, "Schedule changes must take effect today or later")
		return
	}
	if in.To != nil {
		if _, e := time.Parse("2006-01-02", *in.To); e != nil || *in.To < in.From {
			fail(c, 400, "Invalid end date")
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
	cycle := []json.RawMessage{}
	for _, id := range in.Shifts {
		if id == "rest" {
			cycle = append(cycle, json.RawMessage(`null`))
			continue
		}
		if !validUUID(id) {
			fail(c, 400, "Invalid shift")
			return
		}
		var raw []byte
		e = tx.QueryRow(ctx, `SELECT to_jsonb(sh) FROM shifts sh WHERE id=$1 AND company_id=$2`, id, i.CompanyID).Scan(&raw)
		if e != nil {
			fail(c, 404, "Shift not found in this company")
			return
		}
		cycle = append(cycle, raw)
	}
	raw, _ := json.Marshal(cycle)
	// Sort membership locks to avoid deadlocks between overlapping bulk assignments.
	members, e := tx.Query(ctx, `SELECT id::text FROM memberships WHERE company_id=$1 AND id=ANY($2::uuid[]) AND active ORDER BY id FOR UPDATE`, i.CompanyID, in.Members)
	if e != nil {
		problem(c, e)
		return
	}
	ids := []string{}
	for members.Next() {
		var id string
		_ = members.Scan(&id)
		ids = append(ids, id)
	}
	members.Close()
	if len(ids) != len(in.Members) {
		fail(c, 404, "One or more employees are unavailable in this company")
		return
	}
	for _, member := range ids {
		_, e = tx.Exec(ctx, `INSERT INTO schedule_assignments(company_id,membership_id,effective_from,effective_to,cycle,previous_schedule,actor_id,reason) VALUES($1,$2,$3,$4,$5,COALESCE((SELECT to_jsonb(sa) FROM schedule_assignments sa WHERE membership_id=$2 AND effective_from<=$3::date AND (effective_to IS NULL OR effective_to>=$3::date) ORDER BY created_at DESC LIMIT 1),'{}'),$6,$7)`, i.CompanyID, member, in.From, in.To, raw, i.UserID, in.Reason)
		if e != nil {
			problem(c, e)
			return
		}
	}
	e = audit(ctx, tx, i.CompanyID, i.UserID, "schedules.assigned", string(raw), in.Reason, c.GetString("request_id"))
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(201, gin.H{"count": len(ids)})
}

// Resolve snapshots by work date; an expired temporary override reveals the underlying assignment.
func schedulePolicy(ctx context.Context, tx pgx.Tx, i Identity, date string) (attendance.Policy, error) {
	var raw []byte
	var from time.Time
	e := tx.QueryRow(ctx, `SELECT cycle,effective_from FROM schedule_assignments WHERE company_id=$1 AND membership_id=$2 AND effective_from<=$3::date AND (effective_to IS NULL OR effective_to>=$3::date) ORDER BY created_at DESC LIMIT 1`, i.CompanyID, i.MembershipID, date).Scan(&raw, &from)
	if errors.Is(e, pgx.ErrNoRows) {
		e = tx.QueryRow(ctx, `SELECT jsonb_build_array(to_jsonb(sh)) FROM memberships m JOIN shifts sh ON sh.id=m.shift_id AND sh.company_id=m.company_id WHERE m.id=$1 AND m.company_id=$2`, i.MembershipID, i.CompanyID).Scan(&raw)
		from, _ = time.Parse("2006-01-02", date)
	}
	if e != nil {
		return attendance.Policy{}, e
	}
	var cycle []*shiftSnapshot
	if e = json.Unmarshal(raw, &cycle); e != nil {
		return attendance.Policy{}, e
	}
	if len(cycle) == 0 {
		return attendance.Policy{}, errors.New("empty schedule")
	}
	day, _ := time.Parse("2006-01-02", date)
	offset := int(day.Sub(from).Hours()/24) % len(cycle)
	if offset < 0 {
		offset += len(cycle)
	}
	sh := cycle[offset]
	p := attendance.Policy{Timezone: i.Timezone, Start: "00:00", End: "23:59", Expected: 480, Workdays: []int{}}
	e = tx.QueryRow(ctx, `SELECT policy_version,breaks_enabled FROM companies WHERE id=$1`, i.CompanyID).Scan(&p.Version, &p.Breaks)
	if e != nil {
		return p, e
	}
	if sh != nil {
		p.ShiftName = sh.Name
		p.Start = sh.Start
		p.End = sh.End
		p.Grace = sh.Grace
		p.Expected = sh.Expected
		p.UnpaidBreak = sh.Unpaid
		p.EarlyWindow = sh.Early
		p.Overtime = sh.Overtime
		p.Workdays = sh.Workdays
	}
	return p, nil
}
func effectivePolicy(ctx context.Context, tx pgx.Tx, i Identity, now time.Time) (attendance.Policy, string, error) {
	loc, e := time.LoadLocation(i.Timezone)
	if e != nil {
		return attendance.Policy{}, "", e
	}
	today := now.In(loc).Format("2006-01-02")
	yesterday := now.In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	prev, err := schedulePolicy(ctx, tx, i, yesterday)
	if err == nil && prev.End <= prev.Start && attendance.Scheduled(yesterday, prev) {
		start, end, err := attendance.Bounds(yesterday, prev)
		if err == nil && !now.Before(start) && now.Before(end) {
			return prev, yesterday, nil
		}
	}
	p, e := schedulePolicy(ctx, tx, i, today)
	return p, today, e
}
