package server

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"devaraj-time/internal/attendance"
	"devaraj-time/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

var resourceQueries = map[string]string{
	"employees":   `SELECT jsonb_build_object('id',m.id,'name',u.name,'email',u.email,'employee_code',m.employee_code,'role',m.role,'active',m.active,'department_id',m.department_id,'department',d.name,'branch_id',m.branch_id,'branch',b.name,'shift_id',m.shift_id,'shift',sh.name,'position',m.position,'start_date',m.start_date) FROM memberships m JOIN users u ON u.id=m.user_id LEFT JOIN departments d ON d.id=m.department_id LEFT JOIN branches b ON b.id=m.branch_id LEFT JOIN shifts sh ON sh.id=m.shift_id WHERE m.company_id=$1 AND ($2::text IS NULL OR m.department_id::text=$2) ORDER BY u.name LIMIT $3 OFFSET $4`,
	"departments": `SELECT to_jsonb(d) FROM departments d WHERE company_id=$1 AND ($2::text IS NULL OR id::text=$2) ORDER BY name LIMIT $3 OFFSET $4`,
	"branches":    `SELECT to_jsonb(b) FROM branches b WHERE company_id=$1 AND $2::text IS NULL ORDER BY name LIMIT $3 OFFSET $4`,
	"shifts":      `SELECT to_jsonb(sh) FROM shifts sh WHERE company_id=$1 AND $2::text IS NULL ORDER BY name LIMIT $3 OFFSET $4`,
	"workplaces":  `SELECT to_jsonb(w) FROM workplaces w WHERE company_id=$1 AND $2::text IS NULL ORDER BY name LIMIT $3 OFFSET $4`,
	"points":      `SELECT to_jsonb(p)||jsonb_build_object('workplace',w.name,'branch',b.name) FROM attendance_points p JOIN workplaces w ON w.id=p.workplace_id JOIN branches b ON b.id=w.branch_id WHERE p.company_id=$1 AND $2::text IS NULL ORDER BY p.created_at DESC LIMIT $3 OFFSET $4`,
}

func (s *Server) references(c *gin.Context) {
	if !require(c, "employees") {
		return
	}
	i := who(c)
	result := gin.H{}
	for _, r := range []string{"departments", "branches", "shifts", "workplaces"} {
		rows, e := s.jsonRows(c, resourceQueries[r], i.CompanyID, nil, 100, 0)
		if e != nil {
			problem(c, e)
			return
		}
		result[r] = rows
	}
	c.JSON(200, result)
}
func (s *Server) listResource(c *gin.Context) {
	resource := c.Param("resource")
	q, ok := resourceQueries[resource]
	if !ok {
		fail(c, 404, "Unknown resource")
		return
	}
	permission := "configure"
	if resource == "shifts" {
		permission = "schedules"
	}
	if resource == "employees" || resource == "departments" {
		permission = "team"
	}
	if !require(c, permission) {
		return
	}
	i := who(c)
	p, limit := page(c)
	rows, e := s.jsonRows(c, q, i.CompanyID, departmentScope(i), limit, (p-1)*limit)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows, "page": p, "has_more": len(rows) == limit})
}
func (s *Server) organization(c *gin.Context) {
	i := who(c)
	rows, e := s.jsonRows(c, `SELECT jsonb_build_object('company',co.name,'timezone',co.timezone,'policy_version',co.policy_version,'breaks_enabled',co.breaks_enabled,'correction_days',co.correction_days,'location_retention_days',co.location_retention_days,'employee_code',m.employee_code,'position',m.position,'department',d.name,'branch',b.name,'shift',to_jsonb(sh)) FROM memberships m JOIN companies co ON co.id=m.company_id LEFT JOIN shifts sh ON sh.id=m.shift_id LEFT JOIN departments d ON d.id=m.department_id LEFT JOIN branches b ON b.id=m.branch_id WHERE m.id=$1`, i.MembershipID)
	if e != nil {
		problem(c, e)
		return
	}
	var organization map[string]any
	if e = json.Unmarshal(rows[0], &organization); e != nil {
		problem(c, e)
		return
	}
	ctx := c.Request.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	p, date, e := effectivePolicy(ctx, tx, i, time.Now())
	if e != nil {
		problem(c, e)
		return
	}
	organization["shift"] = gin.H{"name": p.ShiftName, "start_time": p.Start, "end_time": p.End, "grace_minutes": p.Grace, "expected_minutes": p.Expected, "unpaid_break_minutes": p.UnpaidBreak, "early_window_minutes": p.EarlyWindow, "workdays": p.Workdays, "work_date": date}
	if p.ShiftName == "" {
		organization["shift"] = nil
	}
	c.JSON(200, organization)
}

type employeeInput struct {
	Name       string  `json:"name" binding:"required,min=2,max=120"`
	Email      string  `json:"email" binding:"required,email,max=254"`
	Password   string  `json:"password"`
	Code       string  `json:"employee_code" binding:"required,max=40"`
	Role       string  `json:"role" binding:"required,oneof=admin hr manager employee"`
	Department *string `json:"department_id"`
	Branch     *string `json:"branch_id" binding:"required,uuid"`
	Shift      *string `json:"shift_id" binding:"required,uuid"`
	Position   string  `json:"position" binding:"max=120"`
	Active     bool    `json:"active"`
	StartDate  string  `json:"start_date" binding:"required,datetime=2006-01-02"`
	Reason     string  `json:"reason" binding:"required,min=3,max=1000"`
}

func (s *Server) createResource(c *gin.Context) { s.mutateResource(c, false) }
func (s *Server) updateResource(c *gin.Context) {
	if !resourceID(c) {
		return
	}
	s.mutateResource(c, true)
}
func (s *Server) mutateResource(c *gin.Context, update bool) {
	resource := c.Param("resource")
	permission := "configure"
	if resource == "shifts" {
		permission = "schedules"
	}
	if resource == "employees" {
		permission = "employees"
	}
	if !require(c, permission) {
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
	id := c.Param("id")
	reason := ""
	action := "created"
	if update {
		action = "updated"
	}
	switch resource {
	case "employees":
		var in employeeInput
		if !bind(c, &in) {
			return
		}
		reason = in.Reason
		if in.Department != nil && !validUUID(*in.Department) {
			fail(c, 400, "Invalid department")
			return
		}
		if !i.Can("configure") && in.Role != "employee" {
			fail(c, 403, "Only company administrators can assign privileged roles")
			return
		}
		if update {
			var oldRole, user string
			var oldShift *string
			e = tx.QueryRow(ctx, `SELECT role,user_id::text,shift_id::text FROM memberships WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, i.CompanyID).Scan(&oldRole, &user, &oldShift)
			if e != nil {
				fail(c, 404, "Employee not found")
				return
			}
			if oldRole == "owner" || id == i.MembershipID || (!i.Can("configure") && oldRole != "employee") {
				fail(c, 403, "This membership cannot be changed through employee administration")
				return
			}
			if oldShift != nil && in.Shift != nil && *oldShift != *in.Shift {
				fail(c, 422, "Use Scheduling for effective-dated shift changes")
				return
			}
			var existingEmail, existingName string
			e = tx.QueryRow(ctx, `SELECT email,name FROM users WHERE id=$1`, user).Scan(&existingEmail, &existingName)
			if e != nil {
				problem(c, e)
				return
			}
			if strings.ToLower(in.Email) != existingEmail {
				fail(c, 422, "Account email changes require an identity recovery workflow")
				return
			}
			if in.Name != existingName {
				fail(c, 422, "Account display names can only be changed by the account holder")
				return
			}
			_, e = tx.Exec(ctx, `UPDATE memberships SET employee_code=$1,role=$2,department_id=$3,branch_id=$4,shift_id=$5,position=$6,active=$7,start_date=$8 WHERE id=$9 AND company_id=$10`, in.Code, in.Role, in.Department, in.Branch, in.Shift, in.Position, in.Active, in.StartDate, id, i.CompanyID)
		} else {
			if in.Role == "employee" || in.Role == "manager" {
				in.Password = auth.Token()
			}
			if len(in.Password) < 12 || len(in.Password) > 128 {
				fail(c, 400, "Temporary password must contain 12–128 characters")
				return
			}
			var user string
			e = tx.QueryRow(ctx, `INSERT INTO users(email,name,password_hash) VALUES($1,$2,$3) RETURNING id::text`, strings.ToLower(in.Email), in.Name, auth.Hash(in.Password)).Scan(&user)
			if e == nil {
				e = tx.QueryRow(ctx, `INSERT INTO memberships(company_id,user_id,employee_code,role,department_id,branch_id,shift_id,position,active,start_date) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id::text`, i.CompanyID, user, in.Code, in.Role, in.Department, in.Branch, in.Shift, in.Position, in.Active, in.StartDate).Scan(&id)
			}
		}
		if e == nil {
			// Preserve the initial assignment. Later changes use the effective-dated scheduling endpoint.
			_, e = tx.Exec(ctx, `INSERT INTO schedule_assignments(company_id,membership_id,effective_from,cycle,actor_id,reason) SELECT $1,$2,$3::date,jsonb_build_array(to_jsonb(sh)),$4,$5 FROM shifts sh WHERE sh.id=$6 AND sh.company_id=$1 AND NOT EXISTS(SELECT 1 FROM schedule_assignments WHERE membership_id=$2)`, i.CompanyID, id, in.StartDate, i.UserID, in.Reason, in.Shift)
		}
	case "departments", "branches":
		var in struct {
			Name   string `json:"name" binding:"required,min=2,max=120"`
			Reason string `json:"reason" binding:"required,min=3,max=1000"`
		}
		if !bind(c, &in) {
			return
		}
		reason = in.Reason
		table := resource
		if update {
			e = tx.QueryRow(ctx, `UPDATE `+table+` SET name=$1 WHERE id=$2 AND company_id=$3 RETURNING id::text`, in.Name, id, i.CompanyID).Scan(&id)
		} else {
			e = tx.QueryRow(ctx, `INSERT INTO `+table+`(company_id,name) VALUES($1,$2) RETURNING id::text`, i.CompanyID, in.Name).Scan(&id)
		}
	case "shifts":
		var before []byte
		if update {
			e = tx.QueryRow(ctx, `SELECT to_jsonb(sh) FROM shifts sh WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, i.CompanyID).Scan(&before)
			if e != nil {
				fail(c, 404, "Shift not found")
				return
			}
		}
		var in struct {
			Name     string `json:"name" binding:"required,max=120"`
			Start    string `json:"start_time" binding:"required"`
			End      string `json:"end_time" binding:"required"`
			Grace    int    `json:"grace_minutes" binding:"min=0,max=120"`
			Expected int    `json:"expected_minutes" binding:"min=1,max=1440"`
			Unpaid   int    `json:"unpaid_break_minutes" binding:"min=0,max=240"`
			Early    int    `json:"early_window_minutes" binding:"min=0,max=720"`
			Overtime bool   `json:"overtime_enabled"`
			Workdays []int  `json:"workdays" binding:"required,min=1,max=7,dive,min=0,max=6"`
			Reason   string `json:"reason" binding:"required,min=3,max=1000"`
		}
		if !bind(c, &in) {
			return
		}
		reason = in.Reason
		if _, _, err := attendance.Bounds("2026-01-01", attendance.Policy{Start: in.Start, End: in.End, Timezone: i.Timezone}); err != nil {
			fail(c, 400, "Use HH:mm shift times")
			return
		}
		if update {
			e = tx.QueryRow(ctx, `UPDATE shifts SET name=$1,start_time=$2,end_time=$3,grace_minutes=$4,expected_minutes=$5,unpaid_break_minutes=$6,early_window_minutes=$7,overtime_enabled=$8,workdays=$9 WHERE id=$10 AND company_id=$11 RETURNING id::text`, in.Name, in.Start, in.End, in.Grace, in.Expected, in.Unpaid, in.Early, in.Overtime, in.Workdays, id, i.CompanyID).Scan(&id)
		} else {
			e = tx.QueryRow(ctx, `INSERT INTO shifts(company_id,name,start_time,end_time,grace_minutes,expected_minutes,unpaid_break_minutes,early_window_minutes,overtime_enabled,workdays) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id::text`, i.CompanyID, in.Name, in.Start, in.End, in.Grace, in.Expected, in.Unpaid, in.Early, in.Overtime, in.Workdays).Scan(&id)
		}
		if e == nil {
			_, e = tx.Exec(ctx, `UPDATE companies SET policy_version=policy_version+1 WHERE id=$1`, i.CompanyID)
		}
		if e == nil {
			_, e = tx.Exec(ctx, `INSERT INTO audit_logs(company_id,actor_id,action,target,reason,request_id,data) SELECT $1,$2,'shift.template',$3,$4,$5,jsonb_build_object('before',$6::jsonb,'after',to_jsonb(sh),'effective','Apply through schedule assignments') FROM shifts sh WHERE sh.id::text=$3`, i.CompanyID, i.UserID, id, reason, c.GetString("request_id"), before)
		}
	case "workplaces":
		var in struct {
			Name     string  `json:"name" binding:"required,max=120"`
			Branch   string  `json:"branch_id" binding:"required,uuid"`
			Address  string  `json:"address" binding:"max=500"`
			Lat      float64 `json:"latitude" binding:"min=-90,max=90"`
			Lon      float64 `json:"longitude" binding:"min=-180,max=180"`
			Radius   float64 `json:"radius_m" binding:"min=1,max=10000"`
			Accuracy float64 `json:"max_accuracy_m" binding:"min=1,max=1000"`
			Required bool    `json:"location_required"`
			Active   bool    `json:"active"`
			Reason   string  `json:"reason" binding:"required,min=3,max=1000"`
		}
		if !bind(c, &in) {
			return
		}
		reason = in.Reason
		if update {
			e = tx.QueryRow(ctx, `UPDATE workplaces SET name=$1,branch_id=$2,address=$3,latitude=$4,longitude=$5,radius_m=$6,max_accuracy_m=$7,location_required=$8,active=$9 WHERE id=$10 AND company_id=$11 RETURNING id::text`, in.Name, in.Branch, in.Address, in.Lat, in.Lon, in.Radius, in.Accuracy, in.Required, in.Active, id, i.CompanyID).Scan(&id)
		} else {
			e = tx.QueryRow(ctx, `INSERT INTO workplaces(company_id,name,branch_id,address,latitude,longitude,radius_m,max_accuracy_m,location_required,active) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id::text`, i.CompanyID, in.Name, in.Branch, in.Address, in.Lat, in.Lon, in.Radius, in.Accuracy, in.Required, in.Active).Scan(&id)
		}
	case "points":
		var in struct {
			Name      string `json:"name" binding:"required,max=120"`
			Workplace string `json:"workplace_id" binding:"required,uuid"`
			Active    bool   `json:"active"`
			Reason    string `json:"reason" binding:"required,min=3,max=1000"`
		}
		if !bind(c, &in) {
			return
		}
		reason = in.Reason
		if update {
			e = tx.QueryRow(ctx, `UPDATE attendance_points SET name=$1,workplace_id=$2,active=$3 WHERE id=$4 AND company_id=$5 RETURNING id::text`, in.Name, in.Workplace, in.Active, id, i.CompanyID).Scan(&id)
		} else {
			e = tx.QueryRow(ctx, `INSERT INTO attendance_points(company_id,workplace_id,name,public_id,active) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, i.CompanyID, in.Workplace, in.Name, auth.Token(), in.Active).Scan(&id)
		}
	default:
		fail(c, 404, "Unknown resource")
		return
	}
	if e != nil {
		fail(c, 409, "Save failed: check unique values and company resource assignments")
		return
	}
	e = audit(ctx, tx, i.CompanyID, i.UserID, resource+"."+action, id, reason, c.GetString("request_id"))
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"id": id})
}
func (s *Server) rotatePoint(c *gin.Context) {
	if !require(c, "configure") || !resourceID(c) {
		return
	}
	var in struct {
		Reason string `json:"reason" binding:"required,min=3,max=1000"`
	}
	if !bind(c, &in) {
		return
	}
	i := who(c)
	s.transaction(c, func(ctx context.Context, tx pgx.Tx) error {
		var id string
		e := tx.QueryRow(ctx, `UPDATE attendance_points SET public_id=$1,rotated_at=now() WHERE id=$2 AND company_id=$3 RETURNING id::text`, auth.Token(), c.Param("id"), i.CompanyID).Scan(&id)
		if e != nil {
			return e
		}
		return audit(ctx, tx, i.CompanyID, i.UserID, "points.rotated", id, in.Reason, c.GetString("request_id"))
	})
}
func (s *Server) transaction(c *gin.Context, fn func(context.Context, pgx.Tx) error) {
	ctx := c.Request.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	e = fn(ctx, tx)
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		fail(c, 409, "Unable to save; resource is missing or conflicts with current data")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (s *Server) settings(c *gin.Context) {
	if !require(c, "configure") {
		return
	}
	var in struct {
		Name      string `json:"name" binding:"required,max=120"`
		Timezone  string `json:"timezone" binding:"required,max=80"`
		Breaks    bool   `json:"breaks_enabled"`
		Days      int    `json:"correction_days" binding:"min=1,max=90"`
		Retention int    `json:"location_retention_days" binding:"min=1,max=365"`
		Reason    string `json:"reason" binding:"required,min=3,max=1000"`
	}
	if !bind(c, &in) {
		return
	}
	if _, e := time.LoadLocation(in.Timezone); e != nil {
		fail(c, 400, "Invalid IANA timezone")
		return
	}
	i := who(c)
	s.transaction(c, func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE companies SET name=$1,timezone=$2,breaks_enabled=$3,correction_days=$4,location_retention_days=$5,policy_version=policy_version+1 WHERE id=$6`, in.Name, in.Timezone, in.Breaks, in.Days, in.Retention, i.CompanyID)
		if e != nil {
			return e
		}
		return audit(ctx, tx, i.CompanyID, i.UserID, "settings.updated", i.CompanyID, in.Reason, c.GetString("request_id"))
	})
}
func (s *Server) auditLogs(c *gin.Context) {
	if !require(c, "audit") {
		return
	}
	p, limit := page(c)
	rows, e := s.jsonRows(c, `SELECT to_jsonb(a)||jsonb_build_object('actor',u.name) FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_id WHERE a.company_id=$1 ORDER BY a.created_at DESC LIMIT $2 OFFSET $3`, who(c).CompanyID, limit, (p-1)*limit)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows, "page": p, "has_more": len(rows) == limit})
}
func (s *Server) notifications(c *gin.Context) {
	i := who(c)
	rows, e := s.jsonRows(c, `SELECT to_jsonb(n) FROM notifications n WHERE membership_id=$1 AND company_id=$2 ORDER BY created_at DESC LIMIT 50`, i.MembershipID, i.CompanyID)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, rows)
}
func (s *Server) readNotifications(c *gin.Context) {
	i := who(c)
	_, e := s.DB.Exec(c.Request.Context(), `UPDATE notifications SET read_at=now() WHERE membership_id=$1 AND company_id=$2 AND read_at IS NULL`, i.MembershipID, i.CompanyID)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func marshal(v any) []byte { b, _ := json.Marshal(v); return b }
