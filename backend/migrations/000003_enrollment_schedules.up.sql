ALTER TABLE companies ADD COLUMN active boolean NOT NULL DEFAULT true;
ALTER TABLE companies ADD COLUMN subscription text NOT NULL DEFAULT 'trial';
ALTER TABLE users ADD COLUMN platform_admin boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN recovery_email text;
ALTER TABLE users ADD COLUMN recovery_verified_at timestamptz;
ALTER TABLE user_sessions ALTER COLUMN company_id DROP NOT NULL;
CREATE TABLE employee_credentials (
 id text PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies, membership_id uuid NOT NULL,
 credential jsonb NOT NULL, rp_id text NOT NULL, label text NOT NULL DEFAULT 'Employee passkey',
 created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL DEFAULT now()+interval '365 days', revoked_at timestamptz,
 FOREIGN KEY(company_id,membership_id) REFERENCES memberships(company_id,id)
);
ALTER TABLE user_sessions ADD COLUMN credential_id text REFERENCES employee_credentials;
CREATE TABLE enrollment_invitations (
 token_hash text PRIMARY KEY, company_id uuid NOT NULL REFERENCES companies, membership_id uuid NOT NULL,
 issued_by uuid NOT NULL REFERENCES users, expires_at timestamptz NOT NULL, used_at timestamptz,
 FOREIGN KEY(company_id,membership_id) REFERENCES memberships(company_id,id)
);
CREATE TABLE auth_ceremonies (
 token_hash text PRIMARY KEY, kind text NOT NULL, membership_id uuid REFERENCES memberships,
 invitation_hash text REFERENCES enrollment_invitations, state jsonb NOT NULL,
 expires_at timestamptz NOT NULL DEFAULT now()+interval '5 minutes'
);
CREATE TABLE recovery_tokens (
 token_hash text PRIMARY KEY, user_id uuid NOT NULL REFERENCES users, purpose text NOT NULL CHECK(purpose IN ('password','verify_email')),
 destination text NOT NULL, expires_at timestamptz NOT NULL, used_at timestamptz
);
CREATE TABLE platform_audit (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), actor_id uuid REFERENCES users, action text NOT NULL,
 target text NOT NULL, reason text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER immutable_platform_audit BEFORE UPDATE OR DELETE ON platform_audit FOR EACH ROW EXECUTE FUNCTION protect_immutable();
CREATE TABLE schedule_assignments (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies,
 membership_id uuid NOT NULL, effective_from date NOT NULL, effective_to date,
 cycle jsonb NOT NULL CHECK(jsonb_typeof(cycle)='array' AND jsonb_array_length(cycle)>0),
 previous_schedule jsonb NOT NULL DEFAULT '[]', actor_id uuid REFERENCES users, reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), CHECK(effective_to IS NULL OR effective_to>=effective_from),
 FOREIGN KEY(company_id,membership_id) REFERENCES memberships(company_id,id)
);
CREATE INDEX schedule_member_dates ON schedule_assignments(company_id,membership_id,effective_from,created_at DESC);
CREATE TRIGGER immutable_schedules BEFORE UPDATE OR DELETE ON schedule_assignments FOR EACH ROW EXECUTE FUNCTION protect_immutable();
INSERT INTO schedule_assignments(company_id,membership_id,effective_from,cycle,reason)
 SELECT m.company_id,m.id,'1900-01-01',jsonb_build_array(to_jsonb(sh)),'Preserved assignment at scheduling migration'
 FROM memberships m JOIN shifts sh ON sh.id=m.shift_id AND sh.company_id=m.company_id;
