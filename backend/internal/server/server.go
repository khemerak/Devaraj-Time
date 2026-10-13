package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"devaraj-time/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Origin     string
	Secure     bool
	Production bool
	SendMail   func(to, subject, body string) error
	ResetTTL   time.Duration
}
type Server struct {
	DB       *pgxpool.Pool
	Config   Config
	mu       sync.Mutex
	attempts map[string]bucket
}
type bucket struct {
	Count int
	Until time.Time
}
type Identity struct {
	UserID       string  `json:"user_id"`
	CompanyID    string  `json:"company_id"`
	MembershipID string  `json:"membership_id"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	Company      string  `json:"company"`
	Timezone     string  `json:"timezone"`
	Role         string  `json:"role"`
	DepartmentID *string `json:"department_id"`
	CSRF         string  `json:"csrf"`
	CredentialID string  `json:"-"`
}

var permissions = map[string]map[string]bool{
	"owner":   {"configure": true, "schedules": true, "employees": true, "review": true, "reports": true, "audit": true, "team": true},
	"admin":   {"configure": true, "schedules": true, "employees": true, "review": true, "reports": true, "audit": true, "team": true},
	"hr":      {"schedules": true, "employees": true, "review": true, "reports": true, "team": true},
	"manager": {"review": true, "reports": true, "team": true},
}

func (i Identity) Can(p string) bool { return permissions[i.Role][p] }
func who(c *gin.Context) Identity    { return c.MustGet("identity").(Identity) }
func fail(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message, "request_id": c.GetString("request_id")})
}
func problem(c *gin.Context, e error) {
	slog.Error("request failed", "request_id", c.GetString("request_id"), "error", e)
	fail(c, 500, "Unable to complete the request")
}
func bind(c *gin.Context, v any) bool {
	if e := c.ShouldBindJSON(v); e != nil {
		fail(c, 400, "Invalid request fields")
		return false
	}
	return true
}
func New(db *pgxpool.Pool, cfg Config) *Server {
	return &Server{DB: db, Config: cfg, attempts: map[string]bucket{}}
}
func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	_ = r.SetTrustedProxies(nil)
	r.Use(func(c *gin.Context) {
		id := auth.Token()
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Cache-Control", "no-store")
		c.Header("Permissions-Policy", "camera=(self), geolocation=(self)")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		start := time.Now()
		c.Next()
		slog.Info("http", "method", c.Request.Method, "path", c.FullPath(), "status", c.Writer.Status(), "duration", time.Since(start), "request_id", id)
	})
	r.GET("/api/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if e := s.DB.Ping(ctx); e != nil {
			fail(c, 503, "Database unavailable")
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
	a := r.Group("/api")
	a.Use(s.rateLimit)
	a.POST("/auth/login", s.origin, s.login)
	a.POST("/auth/recovery", s.origin, s.requestReset)
	a.POST("/auth/reset", s.origin, s.resetPassword)
	a.POST("/auth/verify-recovery", s.origin, s.verifyRecovery)
	a.POST("/auth/enroll/begin", s.origin, s.beginEnrollment)
	a.POST("/auth/enroll/finish", s.origin, s.finishEnrollment)
	a.POST("/auth/passkey/begin", s.origin, s.beginPasskey)
	a.POST("/auth/passkey/finish", s.origin, s.finishPasskey)
	a.GET("/public/config", func(c *gin.Context) { c.JSON(200, gin.H{"origin": s.Config.Origin}) })
	a.Use(s.authenticate)
	a.GET("/platform/companies", s.platformCompanies)
	a.POST("/platform/companies", s.register)
	a.PATCH("/platform/companies/:id", s.platformUpdate)
	a.GET("/platform/config", s.platformConfiguration)
	a.PATCH("/platform/config", s.updatePlatformConfiguration)
	a.Use(func(c *gin.Context) {
		if who(c).Role == "system_admin" && !strings.HasPrefix(c.FullPath(), "/api/auth/") {
			fail(c, 403, "Use explicitly authorized platform functions")
			return
		}
		c.Next()
	})
	a.POST("/auth/recovery-email", s.requestRecoveryEmail)
	a.GET("/devices", s.devices)
	a.POST("/devices/invitations", s.issueInvitation)
	a.POST("/devices/:id/revoke", s.revokeDevice)
	a.GET("/schedules", s.schedules)
	a.POST("/schedules", s.assignSchedules)
	a.GET("/auth/me", s.me)
	a.POST("/auth/password", s.changePassword)
	a.PATCH("/auth/profile", s.updateProfile)
	a.POST("/auth/logout", s.logout)
	a.POST("/auth/company", s.switchCompany)
	a.GET("/organization", s.organization)
	a.GET("/references", s.references)
	a.GET("/dashboard", s.dashboard)
	a.GET("/attendance", s.history)
	a.GET("/attendance/:id", s.details)
	a.POST("/attendance", s.submit)
	a.GET("/points/:public", s.point)
	a.GET("/requests", s.requests)
	a.POST("/requests", s.createRequest)
	a.POST("/requests/:id/review", s.review)
	a.POST("/requests/:id/cancel", s.cancelRequest)
	a.GET("/notifications", s.notifications)
	a.POST("/notifications/read", s.readNotifications)
	a.GET("/reports", s.report)
	a.GET("/audit", s.auditLogs)
	a.GET("/admin/:resource", s.listResource)
	a.POST("/admin/:resource", s.createResource)
	a.PATCH("/admin/:resource/:id", s.updateResource)
	a.POST("/admin/points/:id/rotate", s.rotatePoint)
	a.PATCH("/settings", s.settings)
	return r
}
func (s *Server) origin(c *gin.Context) {
	if c.GetHeader("Origin") != s.Config.Origin {
		fail(c, 403, "Request origin is not permitted")
		return
	}
	c.Next()
}
func (s *Server) rateLimit(c *gin.Context) {
	key := c.ClientIP() + ":" + c.FullPath()
	limit := 180
	if strings.HasPrefix(c.FullPath(), "/api/auth/") {
		limit = 20
	}
	s.mu.Lock()
	now := time.Now()
	if len(s.attempts) > 10000 {
		for k, b := range s.attempts {
			if now.After(b.Until) {
				delete(s.attempts, k)
			}
		}
	}
	b := s.attempts[key]
	if now.After(b.Until) {
		b = bucket{Until: now.Add(time.Minute)}
	}
	b.Count++
	s.attempts[key] = b
	s.mu.Unlock()
	if b.Count > limit {
		c.Header("Retry-After", "60")
		fail(c, 429, "Too many requests; retry in a minute")
		return
	}
	c.Next()
}
func (s *Server) authenticate(c *gin.Context) {
	token, e := c.Cookie("devaraj_session")
	if e != nil {
		fail(c, 401, "Please sign in")
		return
	}
	var i Identity
	e = s.DB.QueryRow(c.Request.Context(), `SELECT u.id::text,m.company_id::text,m.id::text,u.name,u.email,co.name,co.timezone,m.role,m.department_id::text,se.csrf,COALESCE(se.credential_id,'') FROM user_sessions se JOIN users u ON u.id=se.user_id JOIN memberships m ON m.user_id=u.id AND m.company_id=se.company_id JOIN companies co ON co.id=m.company_id LEFT JOIN employee_credentials cr ON cr.id=se.credential_id AND cr.membership_id=m.id WHERE se.token_hash=$1 AND se.expires_at>now() AND m.active AND co.active AND ((se.credential_id IS NULL AND m.role IN ('owner','admin','hr')) OR (cr.revoked_at IS NULL AND cr.expires_at>now()))`, auth.Digest(token)).Scan(&i.UserID, &i.CompanyID, &i.MembershipID, &i.Name, &i.Email, &i.Company, &i.Timezone, &i.Role, &i.DepartmentID, &i.CSRF, &i.CredentialID)
	if e != nil {
		e = s.DB.QueryRow(c.Request.Context(), `SELECT u.id::text,u.name,u.email,se.csrf FROM user_sessions se JOIN users u ON u.id=se.user_id WHERE se.token_hash=$1 AND se.expires_at>now() AND se.company_id IS NULL AND u.platform_admin`, auth.Digest(token)).Scan(&i.UserID, &i.Name, &i.Email, &i.CSRF)
		if e == nil {
			i.Role = "system_admin"
		}
	}
	if e != nil {
		fail(c, 401, "Session expired; please sign in")
		return
	}
	if c.Request.Method != "GET" {
		if c.GetHeader("Origin") != s.Config.Origin || c.GetHeader("X-CSRF-Token") != i.CSRF {
			fail(c, 403, "Request verification failed; refresh and retry")
			return
		}
	}
	c.Set("identity", i)
	c.Next()
}
func require(c *gin.Context, p string) bool {
	if !who(c).Can(p) {
		fail(c, 403, "You do not have permission for this action")
		return false
	}
	return true
}
func (s *Server) cookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: "devaraj_session", Value: token, Path: "/", HttpOnly: true, Secure: s.Config.Secure, SameSite: http.SameSiteStrictMode, MaxAge: maxAge})
}
func (s *Server) startSession(c *gin.Context, user, company string) error {
	token, csrf := auth.Token(), auth.Token()
	_, e := s.DB.Exec(c.Request.Context(), `INSERT INTO user_sessions(token_hash,user_id,company_id,csrf,expires_at) VALUES($1,$2,NULLIF($3,'')::uuid,$4,now()+interval '12 hours')`, auth.Digest(token), user, company, csrf)
	if e == nil {
		s.cookie(c, token, 43200)
	}
	return e
}
func (s *Server) login(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email,max=254"`
		Password string `json:"password" binding:"required,max=128"`
	}
	if !bind(c, &input) {
		return
	}
	var id, hash, company string
	e := s.DB.QueryRow(c.Request.Context(), `SELECT u.id::text,u.password_hash,CASE WHEN u.platform_admin THEN '' ELSE m.company_id::text END FROM users u LEFT JOIN memberships m ON m.user_id=u.id AND m.active AND m.role IN ('owner','admin','hr') LEFT JOIN companies co ON co.id=m.company_id AND co.active WHERE u.email=$1 AND (u.platform_admin OR co.id IS NOT NULL) ORDER BY m.created_at LIMIT 1`, strings.ToLower(input.Email)).Scan(&id, &hash, &company)
	if e != nil || !auth.Verify(input.Password, hash) {
		if hash == "" {
			_ = auth.Verify(input.Password, auth.Hash("timing-only-dummy"))
		}
		fail(c, 401, "Email or password is incorrect")
		return
	}
	if e = s.startSession(c, id, company); e != nil {
		problem(c, e)
		return
	}
	if company != "" {
		s.log(c, company, id, "login", id, "")
	}
	c.JSON(200, gin.H{"ok": true})
}
func (s *Server) register(c *gin.Context) {
	if who(c).Role != "system_admin" {
		fail(c, 403, "Platform administrator required")
		return
	}
	var in struct {
		Company  string `json:"company" binding:"required,min=2,max=120"`
		Name     string `json:"name" binding:"required,min=2,max=120"`
		Email    string `json:"email" binding:"required,email,max=254"`
		Password string `json:"password" binding:"required,min=12,max=128"`
		Timezone string `json:"timezone" binding:"required,max=80"`
	}
	if !bind(c, &in) {
		return
	}
	if _, e := time.LoadLocation(in.Timezone); e != nil {
		fail(c, 400, "Invalid IANA timezone")
		return
	}
	ctx := c.Request.Context()
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	var company, user, branch, shift string
	if e = tx.QueryRow(ctx, `INSERT INTO companies(name,timezone) VALUES($1,$2) RETURNING id::text`, in.Company, in.Timezone).Scan(&company); e == nil {
		e = tx.QueryRow(ctx, `INSERT INTO users(email,name,password_hash) VALUES($1,$2,$3) RETURNING id::text`, strings.ToLower(in.Email), in.Name, auth.Hash(in.Password)).Scan(&user)
	}
	if e != nil {
		fail(c, 409, "Account cannot be created with these details")
		return
	}
	e = tx.QueryRow(ctx, `INSERT INTO branches(company_id,name) VALUES($1,'Head office') RETURNING id::text`, company).Scan(&branch)
	if e == nil {
		e = tx.QueryRow(ctx, `INSERT INTO shifts(company_id,name,start_time,end_time) VALUES($1,'Standard','08:00','17:00') RETURNING id::text`, company).Scan(&shift)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO memberships(company_id,user_id,role,employee_code,branch_id,shift_id) VALUES($1,$2,'owner','ADMIN-001',$3,$4)`, company, user, branch, shift)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO schedule_assignments(company_id,membership_id,effective_from,cycle,actor_id,reason) SELECT $1,m.id,CURRENT_DATE,jsonb_build_array(to_jsonb(sh)),$3,'Initial owner schedule' FROM memberships m JOIN shifts sh ON sh.id=m.shift_id WHERE m.company_id=$1 AND m.user_id=$2`, company, user, who(c).UserID)
	}
	if e == nil {
		e = audit(ctx, tx, company, who(c).UserID, "company.created", company, "Platform provisioned owner account", c.GetString("request_id"))
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO platform_audit(actor_id,action,target,reason) VALUES($1,'company.provisioned',$2,'Owner account provisioned')`, who(c).UserID, company)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(201, gin.H{"id": company})
}
func (s *Server) me(c *gin.Context) {
	i := who(c)
	rows, e := s.jsonRows(c, `SELECT jsonb_build_object('id',co.id,'name',co.name,'role',m.role) FROM memberships m JOIN companies co ON co.id=m.company_id WHERE m.user_id=$1 AND m.active ORDER BY co.name`, i.UserID)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"user": i, "companies": rows})
}
func (s *Server) logout(c *gin.Context) {
	token, _ := c.Cookie("devaraj_session")
	if _, e := s.DB.Exec(c.Request.Context(), `DELETE FROM user_sessions WHERE token_hash=$1`, auth.Digest(token)); e != nil {
		problem(c, e)
		return
	}
	i := who(c)
	if i.CompanyID != "" {
		s.log(c, i.CompanyID, i.UserID, "logout", i.UserID, "")
	}
	s.cookie(c, "", -1)
	c.JSON(200, gin.H{"ok": true})
}
func (s *Server) switchCompany(c *gin.Context) {
	var in struct {
		ID string `json:"id" binding:"required,uuid"`
	}
	if !bind(c, &in) {
		return
	}
	i := who(c)
	if i.CredentialID != "" || i.Role == "system_admin" {
		fail(c, 403, "This credential is scoped to its enrolled company")
		return
	}
	var exists bool
	e := s.DB.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM memberships m JOIN companies co ON co.id=m.company_id WHERE m.user_id=$1 AND m.company_id=$2 AND m.active AND co.active AND m.role IN ('owner','admin','hr'))`, i.UserID, in.ID).Scan(&exists)
	if e != nil {
		problem(c, e)
		return
	}
	if !exists {
		fail(c, 403, "No active membership in that company")
		return
	}
	token, _ := c.Cookie("devaraj_session")
	_, e = s.DB.Exec(c.Request.Context(), `UPDATE user_sessions SET company_id=$1,csrf=$2 WHERE token_hash=$3`, in.ID, auth.Token(), auth.Digest(token))
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func audit(ctx context.Context, tx pgx.Tx, company, user, action, target, reason, request string) error {
	_, e := tx.Exec(ctx, `INSERT INTO audit_logs(company_id,actor_id,action,target,reason,request_id) VALUES($1,$2,$3,$4,$5,$6)`, company, user, action, target, reason, request)
	return e
}
func (s *Server) log(c *gin.Context, company, user, action, target, reason string) {
	_, e := s.DB.Exec(c.Request.Context(), `INSERT INTO audit_logs(company_id,actor_id,action,target,reason,request_id) VALUES($1,$2,$3,$4,$5,$6)`, company, user, action, target, reason, c.GetString("request_id"))
	if e != nil {
		slog.Error("audit failed", "error", e)
	}
}
func (s *Server) jsonRows(c *gin.Context, query string, args ...any) ([]json.RawMessage, error) {
	rows, e := s.DB.Query(c.Request.Context(), query, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		result = append(result, json.RawMessage(b))
	}
	return result, rows.Err()
}
func validUUID(v string) bool {
	if len(v) != 36 {
		return false
	}
	for idx, r := range v {
		if idx == 8 || idx == 13 || idx == 18 || idx == 23 {
			if r != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}
func resourceID(c *gin.Context) bool {
	if !validUUID(c.Param("id")) {
		fail(c, 400, "Invalid resource identifier")
		return false
	}
	return true
}
func validOrigin(origin string) error {
	u, e := url.Parse(origin)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" {
		return errors.New("PUBLIC_URL must be a http(s) origin without a trailing slash")
	}
	return nil
}
func ValidateConfig(c Config) error {
	if e := validOrigin(c.Origin); e != nil {
		return e
	}
	if c.Production && (!c.Secure || !strings.HasPrefix(c.Origin, "https://")) {
		return errors.New("production requires HTTPS and secure cookies")
	}
	return nil
}
