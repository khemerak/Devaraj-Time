DROP TABLE schedule_assignments,platform_audit,recovery_tokens,auth_ceremonies,enrollment_invitations;
ALTER TABLE user_sessions DROP COLUMN credential_id;
DROP TABLE employee_credentials;
DELETE FROM user_sessions WHERE company_id IS NULL;
ALTER TABLE user_sessions ALTER COLUMN company_id SET NOT NULL;
ALTER TABLE users DROP COLUMN platform_admin, DROP COLUMN recovery_email, DROP COLUMN recovery_verified_at;
ALTER TABLE companies DROP COLUMN active, DROP COLUMN subscription;
