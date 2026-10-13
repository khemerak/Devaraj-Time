package server

import (
	"context"
	"crypto/tls"
	"devaraj-time/internal/auth"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// SMTP is mandatory STARTTLS with certificate verification; development can use a local TLS-capable mail sink.
func SMTPMail(to, subject, body string) error {
	host, port, from := os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT"), os.Getenv("SMTP_FROM")
	if port == "" {
		port = "587"
	}
	if host == "" || from == "" {
		return errors.New("SMTP is not configured")
	}
	sender, e := mail.ParseAddress(from)
	if e != nil {
		return e
	}
	recipient, e := mail.ParseAddress(to)
	if e != nil {
		return e
	}
	conn, e := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
	if e != nil {
		return e
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	client, e := smtp.NewClient(conn, host)
	if e != nil {
		return e
	}
	defer client.Close()
	if e = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); e != nil {
		return e
	}
	if username := os.Getenv("SMTP_USER"); username != "" {
		if e = client.Auth(smtp.PlainAuth("", username, os.Getenv("SMTP_PASSWORD"), host)); e != nil {
			return e
		}
	}
	if e = client.Mail(sender.Address); e != nil {
		return e
	}
	if e = client.Rcpt(recipient.Address); e != nil {
		return e
	}
	w, e := client.Data()
	if e != nil {
		return e
	}
	_, e = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", sender.String(), recipient.String(), subject, body)
	if e != nil {
		return e
	}
	if e = w.Close(); e != nil {
		return e
	}
	return client.Quit()
}
func (s *Server) mail(to, subject, body string) error {
	if s.Config.SendMail != nil {
		return s.Config.SendMail(to, subject, body)
	}
	return SMTPMail(to, subject, body)
}
func (s *Server) requestReset(c *gin.Context) {
	var in struct {
		Email string `json:"email" binding:"required,email,max=254"`
	}
	if !bind(c, &in) {
		return
	}
	generic := gin.H{"message": "If an eligible account matches, recovery instructions will be sent to its registered or verified recovery email."}
	ctx := c.Request.Context()
	var id, destination string
	e := s.DB.QueryRow(ctx, `SELECT u.id::text,CASE WHEN lower(u.recovery_email)=$1 AND u.recovery_verified_at IS NOT NULL THEN u.recovery_email ELSE u.email END FROM users u WHERE (lower(u.email)=$1 OR (lower(u.recovery_email)=$1 AND u.recovery_verified_at IS NOT NULL)) AND (u.platform_admin OR EXISTS(SELECT 1 FROM memberships m JOIN companies co ON co.id=m.company_id WHERE m.user_id=u.id AND m.active AND co.active AND m.role IN ('owner','admin','hr'))) LIMIT 1`, strings.ToLower(in.Email)).Scan(&id, &destination)
	if e != nil {
		c.JSON(200, generic)
		return
	}
	// Account limit also covers requests from different IPs and different aliases.
	var recent bool
	e = s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM recovery_tokens WHERE user_id=$1 AND purpose='password' AND expires_at>now())`, id).Scan(&recent)
	if e != nil || recent {
		c.JSON(200, generic)
		return
	}
	ttl := s.Config.ResetTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	token := auth.Token()
	_, e = s.DB.Exec(ctx, `INSERT INTO recovery_tokens(token_hash,user_id,purpose,destination,expires_at) VALUES($1,$2,'password',$3,$4)`, auth.Digest(token), id, destination, time.Now().Add(ttl))
	if e == nil {
		deliver := func() {
			if err := s.mail(destination, "Reset your Devaraj Time password", "Open this single-use link within "+ttl.String()+":\n"+s.Config.Origin+"/reset#"+token+"\nIf you did not request this, ignore this email."); err != nil {
				cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, _ = s.DB.Exec(cleanup, `DELETE FROM recovery_tokens WHERE token_hash=$1`, auth.Digest(token))
			}
		}
		// Real SMTP latency must not reveal whether an account exists. The injected
		// test transport is synchronous so tests can assert destination and token behavior.
		if s.Config.SendMail != nil {
			deliver()
		} else {
			go deliver()
		}
	}
	// Do not log tokens, destinations or delivery errors that may contain the message.
	c.JSON(200, generic)
}
func (s *Server) resetPassword(c *gin.Context) {
	var in struct {
		Token    string `json:"token" binding:"required,max=128"`
		Password string `json:"password" binding:"required,min=12,max=128"`
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
	var user, destination string
	e = tx.QueryRow(ctx, `UPDATE recovery_tokens t SET used_at=now() FROM users u WHERE t.token_hash=$1 AND t.user_id=u.id AND t.purpose='password' AND t.used_at IS NULL AND t.expires_at>now() AND (t.destination=u.email OR (t.destination=u.recovery_email AND u.recovery_verified_at IS NOT NULL)) AND (u.platform_admin OR EXISTS(SELECT 1 FROM memberships m JOIN companies co ON co.id=m.company_id WHERE m.user_id=u.id AND m.active AND co.active AND m.role IN ('owner','admin','hr'))) RETURNING t.user_id::text,t.destination`, auth.Digest(in.Token)).Scan(&user, &destination)
	if e != nil {
		fail(c, 400, "Reset link is invalid, expired or already used")
		return
	}
	_, e = tx.Exec(ctx, `UPDATE users SET password_hash=$1 WHERE id=$2`, auth.Hash(in.Password), user)
	if e == nil {
		_, e = tx.Exec(ctx, `DELETE FROM user_sessions WHERE user_id=$1`, user)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE recovery_tokens SET used_at=now() WHERE user_id=$1 AND purpose='password' AND used_at IS NULL`, user)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO platform_audit(actor_id,action,target,reason) VALUES($1,'password.reset',$2,'Single-use verified reset link')`, user, user)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	_ = s.mail(destination, "Devaraj Time password changed", "Your password was changed and existing sessions were signed out. Contact your administrator if you did not make this change.")
	c.JSON(200, gin.H{"message": "Password changed. Sign in with your new password."})
}
func (s *Server) requestRecoveryEmail(c *gin.Context) {
	i := who(c)
	if i.Role != "system_admin" && !i.Can("employees") {
		fail(c, 403, "Administrative account required")
		return
	}
	var in struct {
		Email    string `json:"email" binding:"required,email,max=254"`
		Password string `json:"password" binding:"required,max=128"`
	}
	if !bind(c, &in) {
		return
	}
	var hash string
	e := s.DB.QueryRow(c.Request.Context(), `SELECT password_hash FROM users WHERE id=$1`, i.UserID).Scan(&hash)
	if e != nil || !auth.Verify(in.Password, hash) {
		fail(c, 403, "Current password is incorrect")
		return
	}
	token := auth.Token()
	_, e = s.DB.Exec(c.Request.Context(), `INSERT INTO recovery_tokens(token_hash,user_id,purpose,destination,expires_at) VALUES($1,$2,'verify_email',$3,now()+interval '15 minutes')`, auth.Digest(token), i.UserID, strings.ToLower(in.Email))
	if e == nil {
		e = s.mail(strings.ToLower(in.Email), "Verify your Devaraj Time recovery email", s.Config.Origin+"/verify-recovery#"+token)
	}
	if e != nil {
		fail(c, 503, "Email delivery unavailable; contact the platform administrator")
		return
	}
	c.JSON(200, gin.H{"message": "Check that email for a verification link. It cannot receive password resets until verified."})
}
func (s *Server) verifyRecovery(c *gin.Context) {
	var in struct {
		Token string `json:"token" binding:"required,max=128"`
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
	var user, destination string
	e = tx.QueryRow(ctx, `UPDATE recovery_tokens SET used_at=now() WHERE token_hash=$1 AND purpose='verify_email' AND used_at IS NULL AND expires_at>now() RETURNING user_id::text,destination`, auth.Digest(in.Token)).Scan(&user, &destination)
	if e != nil {
		fail(c, 400, "Verification link expired or used")
		return
	}
	_, e = tx.Exec(ctx, `UPDATE users SET recovery_email=$1,recovery_verified_at=now() WHERE id=$2`, destination, user)
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE recovery_tokens SET used_at=now() WHERE user_id=$1 AND purpose='verify_email' AND used_at IS NULL`, user)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"message": "Recovery email verified."})
}
