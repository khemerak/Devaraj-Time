package server

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
)

func (s *Server) platformCompanies(c *gin.Context) {
	if who(c).Role != "system_admin" {
		fail(c, 403, "Platform administrator required")
		return
	}
	rows, e := s.jsonRows(c, `SELECT jsonb_build_object('id',id,'name',name,'timezone',timezone,'active',active,'subscription',subscription,'created_at',created_at) FROM companies ORDER BY name LIMIT 1000`)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func (s *Server) platformConfiguration(c *gin.Context) {
	if who(c).Role != "system_admin" {
		fail(c, 403, "Platform administrator required")
		return
	}
	rows, e := s.jsonRows(c, `SELECT to_jsonb(ps)-'singleton' FROM platform_settings ps WHERE singleton`)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, rows[0])
}
func (s *Server) updatePlatformConfiguration(c *gin.Context) {
	i := who(c)
	if i.Role != "system_admin" {
		fail(c, 403, "Platform administrator required")
		return
	}
	var in struct {
		InvitationHours int    `json:"invitation_hours" binding:"min=1,max=72"`
		CredentialDays  int    `json:"credential_days" binding:"min=7,max=365"`
		SupportEmail    string `json:"support_email" binding:"omitempty,email,max=254"`
		Reason          string `json:"reason" binding:"required,min=3,max=1000"`
	}
	if !bind(c, &in) {
		return
	}
	ctx := c.Request.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	var before []byte
	e = tx.QueryRow(ctx, `SELECT to_jsonb(ps) FROM platform_settings ps WHERE singleton FOR UPDATE`).Scan(&before)
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE platform_settings SET invitation_hours=$1,credential_days=$2,support_email=$3 WHERE singleton`, in.InvitationHours, in.CredentialDays, in.SupportEmail)
	}
	data, _ := json.Marshal(in)
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO platform_audit(actor_id,action,target,reason) VALUES($1,'platform.config',$2,$3)`, i.UserID, string(before)+" → "+string(data), in.Reason)
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
func (s *Server) platformUpdate(c *gin.Context) {
	i := who(c)
	if i.Role != "system_admin" {
		fail(c, 403, "Platform administrator required")
		return
	}
	if !resourceID(c) {
		return
	}
	var in struct {
		Active       bool   `json:"active"`
		Subscription string `json:"subscription" binding:"required,oneof=trial active suspended cancelled"`
		Reason       string `json:"reason" binding:"required,min=3,max=1000"`
	}
	if !bind(c, &in) {
		return
	}
	ctx := c.Request.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, `UPDATE companies SET active=$1,subscription=$2 WHERE id=$3`, in.Active, in.Subscription, c.Param("id"))
	if e != nil {
		problem(c, e)
		return
	}
	if tag.RowsAffected() != 1 {
		fail(c, 404, "Company not found")
		return
	}
	if !in.Active {
		_, e = tx.Exec(ctx, `DELETE FROM user_sessions WHERE company_id=$1`, c.Param("id"))
		if e != nil {
			problem(c, e)
			return
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO platform_audit(actor_id,action,target,reason) VALUES($1,'company.status',$2,$3)`, i.UserID, c.Param("id"), in.Reason)
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
