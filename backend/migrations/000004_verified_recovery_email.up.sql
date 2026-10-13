CREATE UNIQUE INDEX verified_recovery_email_unique ON users(lower(recovery_email)) WHERE recovery_verified_at IS NOT NULL;
