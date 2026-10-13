package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"devaraj-time/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
)

type passkeyUser struct {
	ID, Name    string
	Credentials []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte                         { return []byte(u.ID) }
func (u passkeyUser) WebAuthnName() string                       { return u.Name }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.Name }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }
func (s *Server) webAuthn() (*webauthn.WebAuthn, error) {
	origin, e := url.Parse(s.Config.Origin)
	if e != nil {
		return nil, e
	}
	return webauthn.New(&webauthn.Config{RPID: origin.Hostname(), RPDisplayName: "Devaraj Time", RPOrigins: []string{s.Config.Origin}, AuthenticatorSelection: protocol.AuthenticatorSelection{ResidentKey: protocol.ResidentKeyRequirementRequired, UserVerification: protocol.VerificationRequired}})
}
func (s *Server) devices(c *gin.Context) {
	if !require(c, "employees") {
		return
	}
	i := who(c)
	rows, e := s.jsonRows(c, `SELECT jsonb_build_object('id',cr.id,'membership_id',cr.membership_id,'name',u.name,'label',cr.label,'created_at',cr.created_at,'expires_at',cr.expires_at,'revoked_at',cr.revoked_at,'rp_id',cr.rp_id) FROM employee_credentials cr JOIN memberships m ON m.id=cr.membership_id JOIN users u ON u.id=m.user_id WHERE cr.company_id=$1 ORDER BY cr.created_at DESC LIMIT 500`, i.CompanyID)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"items": rows})
}
func (s *Server) issueInvitation(c *gin.Context) {
	if !require(c, "employees") {
		return
	}
	i := who(c)
	var in struct {
		Membership string `json:"membership_id" binding:"required,uuid"`
		Reason     string `json:"reason" binding:"required,min=3,max=1000"`
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
	var role string
	e = tx.QueryRow(ctx, `SELECT role FROM memberships WHERE id=$1 AND company_id=$2 AND active FOR UPDATE`, in.Membership, i.CompanyID).Scan(&role)
	if e != nil || (role != "employee" && role != "manager") {
		fail(c, 404, "Active employee not found")
		return
	}
	// A replacement invitation revokes all old devices and unfinished invitations atomically.
	_, e = tx.Exec(ctx, `UPDATE employee_credentials SET revoked_at=COALESCE(revoked_at,now()) WHERE membership_id=$1 AND company_id=$2`, in.Membership, i.CompanyID)
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE enrollment_invitations SET used_at=now() WHERE membership_id=$1 AND used_at IS NULL`, in.Membership)
	}
	token := auth.Token()
	var invitationHours int
	if e == nil {
		e = tx.QueryRow(ctx, `SELECT invitation_hours FROM platform_settings WHERE singleton`).Scan(&invitationHours)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO enrollment_invitations(token_hash,company_id,membership_id,issued_by,expires_at) VALUES($1,$2,$3,$4,now()+make_interval(hours=>$5))`, auth.Digest(token), i.CompanyID, in.Membership, i.UserID, invitationHours)
	}
	if e == nil {
		e = audit(ctx, tx, i.CompanyID, i.UserID, "device.invitation", in.Membership, in.Reason, c.GetString("request_id"))
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(201, gin.H{"url": s.Config.Origin + "/enroll#" + token, "expires_in_hours": invitationHours})
}
func (s *Server) revokeDevice(c *gin.Context) {
	if !require(c, "employees") {
		return
	}
	i := who(c)
	var in struct {
		Reason string `json:"reason" binding:"required,min=3,max=1000"`
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
	tag, e := tx.Exec(ctx, `UPDATE employee_credentials SET revoked_at=now() WHERE id=$1 AND company_id=$2 AND revoked_at IS NULL`, c.Param("id"), i.CompanyID)
	if e != nil {
		problem(c, e)
		return
	}
	if tag.RowsAffected() != 1 {
		fail(c, 404, "Device not found")
		return
	}
	e = audit(ctx, tx, i.CompanyID, i.UserID, "device.revoked", c.Param("id"), in.Reason, c.GetString("request_id"))
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (s *Server) saveCeremony(c *gin.Context, kind, member, invitation string, state *webauthn.SessionData) (string, error) {
	token := auth.Token()
	data, e := json.Marshal(state)
	if e != nil {
		return "", e
	}
	_, e = s.DB.Exec(c.Request.Context(), `INSERT INTO auth_ceremonies(token_hash,kind,membership_id,invitation_hash,state) VALUES($1,$2,NULLIF($3,'')::uuid,NULLIF($4,''),$5)`, auth.Digest(token), kind, member, invitation, data)
	return token, e
}
func (s *Server) beginEnrollment(c *gin.Context) {
	var in struct {
		Token string `json:"token" binding:"required,max=128"`
	}
	if !bind(c, &in) {
		return
	}
	var u passkeyUser
	e := s.DB.QueryRow(c.Request.Context(), `SELECT m.id::text,u.name FROM enrollment_invitations inv JOIN memberships m ON m.id=inv.membership_id JOIN users u ON u.id=m.user_id JOIN companies co ON co.id=m.company_id WHERE inv.token_hash=$1 AND inv.used_at IS NULL AND inv.expires_at>now() AND m.active AND co.active`, auth.Digest(in.Token)).Scan(&u.ID, &u.Name)
	if e != nil {
		fail(c, 401, "Invitation expired or already used; contact HR")
		return
	}
	wa, e := s.webAuthn()
	if e != nil {
		problem(c, e)
		return
	}
	options, state, e := wa.BeginRegistration(u)
	if e != nil {
		problem(c, e)
		return
	}
	token, e := s.saveCeremony(c, "enroll", u.ID, auth.Digest(in.Token), state)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"ceremony": token, "options": options})
}

type finishInput struct {
	Ceremony string          `json:"ceremony" binding:"required,max=128"`
	Response json.RawMessage `json:"response" binding:"required"`
}

func ceremonyRequest(c *gin.Context, data []byte) *http.Request {
	r := c.Request.Clone(c.Request.Context())
	r.Body = http.NoBody
	r.Body = ioBody{bytes.NewReader(data)}
	return r
}

type ioBody struct{ *bytes.Reader }

func (ioBody) Close() error { return nil }
func (s *Server) finishEnrollment(c *gin.Context) {
	var in finishInput
	if !bind(c, &in) {
		return
	}
	ctx := c.Request.Context()
	// Consume challenge before verification. Invalid responses cannot reuse it.
	var stateJSON []byte
	var member, invitation string
	e := s.DB.QueryRow(ctx, `DELETE FROM auth_ceremonies WHERE token_hash=$1 AND kind='enroll' AND expires_at>now() RETURNING state,membership_id::text,invitation_hash`, auth.Digest(in.Ceremony)).Scan(&stateJSON, &member, &invitation)
	if e != nil {
		fail(c, 401, "Enrollment expired; open the invitation again")
		return
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	var company, user, name string
	e = tx.QueryRow(ctx, `SELECT m.company_id::text,m.user_id::text,u.name FROM enrollment_invitations inv JOIN memberships m ON m.id=inv.membership_id JOIN users u ON u.id=m.user_id JOIN companies co ON co.id=m.company_id WHERE inv.token_hash=$1 AND inv.membership_id=$2 AND inv.used_at IS NULL AND inv.expires_at>now() AND m.active AND co.active FOR UPDATE OF inv,m`, invitation, member).Scan(&company, &user, &name)
	if e != nil {
		fail(c, 401, "Invitation expired or used")
		return
	}
	var state webauthn.SessionData
	if json.Unmarshal(stateJSON, &state) != nil {
		fail(c, 401, "Invalid ceremony")
		return
	}
	wa, e := s.webAuthn()
	if e != nil {
		problem(c, e)
		return
	}
	credential, e := wa.FinishRegistration(passkeyUser{ID: member, Name: name}, state, ceremonyRequest(c, in.Response))
	if e != nil {
		fail(c, 401, "Passkey enrollment failed; try again")
		return
	}
	data, e := json.Marshal(credential)
	if e != nil {
		problem(c, e)
		return
	}
	id := base64.RawURLEncoding.EncodeToString(credential.ID)
	_, e = tx.Exec(ctx, `INSERT INTO employee_credentials(id,company_id,membership_id,credential,rp_id,expires_at) SELECT $1,$2,$3,$4,$5,now()+make_interval(days=>credential_days) FROM platform_settings WHERE singleton`, id, company, member, data, wa.Config.RPID)
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE enrollment_invitations SET used_at=now() WHERE token_hash=$1`, invitation)
	}
	if e == nil {
		e = audit(ctx, tx, company, user, "device.enrolled", id, "HR invitation consumed", c.GetString("request_id"))
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	// Enrollment may occur on a phone previously signed in as HR/admin or
	// another employee. Do not silently reuse that identity on the next scan.
	s.cookie(c, "", -1)
	c.JSON(200, gin.H{"ok": true})
}
func (s *Server) beginPasskey(c *gin.Context) {
	wa, e := s.webAuthn()
	if e != nil {
		problem(c, e)
		return
	}
	options, state, e := wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if e != nil {
		problem(c, e)
		return
	}
	token, e := s.saveCeremony(c, "login", "", "", state)
	if e != nil {
		problem(c, e)
		return
	}
	c.JSON(200, gin.H{"ceremony": token, "options": options})
}
func (s *Server) finishPasskey(c *gin.Context) {
	var in finishInput
	if !bind(c, &in) {
		return
	}
	ctx := c.Request.Context()
	var data []byte
	e := s.DB.QueryRow(ctx, `DELETE FROM auth_ceremonies WHERE token_hash=$1 AND kind='login' AND expires_at>now() RETURNING state`, auth.Digest(in.Ceremony)).Scan(&data)
	if e != nil {
		fail(c, 401, "Authentication expired; try your passkey again")
		return
	}
	var state webauthn.SessionData
	if json.Unmarshal(data, &state) != nil {
		fail(c, 401, "Invalid ceremony")
		return
	}
	wa, e := s.webAuthn()
	if e != nil {
		problem(c, e)
		return
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		problem(c, e)
		return
	}
	defer tx.Rollback(ctx)
	var company, user, member, credID string
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		credID = base64.RawURLEncoding.EncodeToString(rawID)
		var raw []byte
		var name string
		e := tx.QueryRow(ctx, `SELECT cr.credential,cr.membership_id::text,m.user_id::text,m.company_id::text,u.name FROM employee_credentials cr JOIN memberships m ON m.id=cr.membership_id AND m.company_id=cr.company_id JOIN users u ON u.id=m.user_id JOIN companies co ON co.id=m.company_id WHERE cr.id=$1 AND cr.rp_id=$2 AND cr.revoked_at IS NULL AND cr.expires_at>now() AND m.active AND co.active FOR UPDATE OF cr`, credID, wa.Config.RPID).Scan(&raw, &member, &user, &company, &name)
		if e != nil || !bytes.Equal(userHandle, []byte(member)) {
			return nil, errors.New("unknown employee credential")
		}
		var credential webauthn.Credential
		if e = json.Unmarshal(raw, &credential); e != nil {
			return nil, e
		}
		return passkeyUser{ID: member, Name: name, Credentials: []webauthn.Credential{credential}}, nil
	}
	_, credential, e := wa.FinishPasskeyLogin(handler, state, ceremonyRequest(c, in.Response))
	if e != nil || credential.Authenticator.CloneWarning {
		fail(c, 401, "Device enrollment required; ask HR to enroll or replace your passkey")
		return
	}
	raw, _ := json.Marshal(credential)
	_, e = tx.Exec(ctx, `UPDATE employee_credentials SET credential=$1 WHERE id=$2`, raw, credID)
	token, csrf := auth.Token(), auth.Token()
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO user_sessions(token_hash,user_id,company_id,csrf,credential_id,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '12 hours')`, auth.Digest(token), user, company, csrf, credID)
	}
	if e == nil {
		e = audit(ctx, tx, company, user, "passkey.login", member, "User-verified passkey assertion", c.GetString("request_id"))
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		problem(c, e)
		return
	}
	s.cookie(c, token, int((12 * time.Hour).Seconds()))
	c.JSON(200, gin.H{"ok": true})
}

// Guard is also checked inside the attendance transaction to serialize revocation with writes.
func validAttendanceCredential(c *gin.Context, tx pgx.Tx, i Identity) bool {
	if i.Role != "employee" && i.Role != "manager" {
		return true
	}
	var id string
	e := tx.QueryRow(c.Request.Context(), `SELECT id FROM employee_credentials WHERE id=$1 AND membership_id=$2 AND company_id=$3 AND revoked_at IS NULL AND expires_at>now() FOR SHARE`, i.CredentialID, i.MembershipID, i.CompanyID).Scan(&id)
	if e != nil {
		fail(c, 401, "Device enrollment required")
		return false
	}
	return true
}
