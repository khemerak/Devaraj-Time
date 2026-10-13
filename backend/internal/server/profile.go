package server

import (
	"devaraj-time/internal/auth"
	"github.com/gin-gonic/gin"
)

func (s *Server) changePassword(c *gin.Context) {
	var in struct {
		Current string `json:"current_password" binding:"required,max=128"`
		New     string `json:"new_password" binding:"required,min=12,max=128"`
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
	var hash string
	e = tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1 FOR UPDATE`, i.UserID).Scan(&hash)
	if e != nil {
		problem(c, e)
		return
	}
	if !auth.Verify(in.Current, hash) {
		fail(c, 403, "Current password is incorrect")
		return
	}
	if in.New == in.Current {
		fail(c, 400, "Choose a different password")
		return
	}
	_, e = tx.Exec(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2`, auth.Hash(in.New), i.UserID)
	if e == nil {
		_, e = tx.Exec(ctx, `DELETE FROM user_sessions WHERE user_id=$1`, i.UserID)
	}
	if e == nil {
		if i.Role == "system_admin" {
			_, e = tx.Exec(ctx, `INSERT INTO platform_audit(actor_id,action,target,reason) VALUES($1,'password.changed',$2,'All sessions revoked')`, i.UserID, i.UserID)
		} else {
			e = audit(ctx, tx, i.CompanyID, i.UserID, "password.changed", i.UserID, "User changed their own password; all sessions revoked", c.GetString("request_id"))
		}
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	s.cookie(c, "", -1)
	c.JSON(200, gin.H{"ok": true, "message": "Password changed. Sign in with your new password."})
}
func (s *Server) updateProfile(c *gin.Context) {
	var in struct {
		Name string `json:"name" binding:"required,min=2,max=120"`
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
	_, e = tx.Exec(ctx, `UPDATE users SET name=$1 WHERE id=$2`, in.Name, i.UserID)
	if e == nil {
		e = audit(ctx, tx, i.CompanyID, i.UserID, "profile.updated", i.UserID, "Account holder updated their display name", c.GetString("request_id"))
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
