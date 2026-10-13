CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE companies (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL,
 timezone text NOT NULL DEFAULT 'Asia/Phnom_Penh', policy_version integer NOT NULL DEFAULT 1,
 breaks_enabled boolean NOT NULL DEFAULT true, correction_days integer NOT NULL DEFAULT 14,
 location_retention_days integer NOT NULL DEFAULT 30 CHECK(location_retention_days BETWEEN 1 AND 365),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE users(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text UNIQUE NOT NULL, name text NOT NULL, password_hash text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE departments(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, name text NOT NULL, UNIQUE(company_id,id), UNIQUE(company_id,name));
CREATE TABLE branches(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, name text NOT NULL, UNIQUE(company_id,id));
CREATE TABLE shifts(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, name text NOT NULL,
 start_time text NOT NULL, end_time text NOT NULL, grace_minutes integer NOT NULL DEFAULT 10 CHECK(grace_minutes BETWEEN 0 AND 120),
 expected_minutes integer NOT NULL DEFAULT 480 CHECK(expected_minutes BETWEEN 1 AND 1440),
 unpaid_break_minutes integer NOT NULL DEFAULT 0 CHECK(unpaid_break_minutes BETWEEN 0 AND 240),
 early_window_minutes integer NOT NULL DEFAULT 120 CHECK(early_window_minutes BETWEEN 0 AND 720),
 overtime_enabled boolean NOT NULL DEFAULT false, workdays integer[] NOT NULL DEFAULT ARRAY[1,2,3,4,5],
 UNIQUE(company_id,id)
);
CREATE TABLE memberships(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, user_id uuid NOT NULL REFERENCES users,
 role text NOT NULL CHECK(role IN ('owner','admin','hr','manager','employee')), department_id uuid,
 branch_id uuid, shift_id uuid, employee_code text NOT NULL, position text NOT NULL DEFAULT '', active boolean NOT NULL DEFAULT true,
 start_date date NOT NULL DEFAULT CURRENT_DATE, created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(company_id,user_id), UNIQUE(company_id,employee_code), UNIQUE(company_id,id),
 FOREIGN KEY(company_id,department_id) REFERENCES departments(company_id,id),
 FOREIGN KEY(company_id,branch_id) REFERENCES branches(company_id,id),
 FOREIGN KEY(company_id,shift_id) REFERENCES shifts(company_id,id)
);
CREATE TABLE user_sessions(token_hash text PRIMARY KEY, user_id uuid NOT NULL REFERENCES users, company_id uuid NOT NULL REFERENCES companies, csrf text NOT NULL, expires_at timestamptz NOT NULL);
CREATE TABLE workplaces(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, branch_id uuid NOT NULL,
 name text NOT NULL, address text NOT NULL DEFAULT '', latitude double precision NOT NULL CHECK(latitude BETWEEN -90 AND 90),
 longitude double precision NOT NULL CHECK(longitude BETWEEN -180 AND 180), radius_m double precision NOT NULL CHECK(radius_m BETWEEN 1 AND 10000),
 max_accuracy_m double precision NOT NULL DEFAULT 75 CHECK(max_accuracy_m BETWEEN 1 AND 1000), location_required boolean NOT NULL DEFAULT true,
 active boolean NOT NULL DEFAULT true, UNIQUE(company_id,id), FOREIGN KEY(company_id,branch_id) REFERENCES branches(company_id,id)
);
CREATE TABLE attendance_points(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, workplace_id uuid NOT NULL,
 name text NOT NULL, public_id text UNIQUE NOT NULL, active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), rotated_at timestamptz,
 FOREIGN KEY(company_id,workplace_id) REFERENCES workplaces(company_id,id), UNIQUE(company_id,id)
);
CREATE TABLE attendance_sessions(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, membership_id uuid NOT NULL,
 point_id uuid, work_date date NOT NULL, state text NOT NULL CHECK(state IN ('checked_in','on_break','checked_out')),
 check_in timestamptz NOT NULL, check_out timestamptz, break_started timestamptz, break_seconds bigint NOT NULL DEFAULT 0,
 policy jsonb NOT NULL, worked_minutes integer NOT NULL DEFAULT 0, late_minutes integer NOT NULL DEFAULT 0,
 arrival_delay_minutes integer NOT NULL DEFAULT 0, early_minutes integer NOT NULL DEFAULT 0, overtime_minutes integer NOT NULL DEFAULT 0,
 status text NOT NULL, adjusted boolean NOT NULL DEFAULT false, updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(membership_id,work_date), UNIQUE(company_id,id),
 FOREIGN KEY(company_id,membership_id) REFERENCES memberships(company_id,id), FOREIGN KEY(company_id,point_id) REFERENCES attendance_points(company_id,id)
);
CREATE UNIQUE INDEX one_open_session ON attendance_sessions(membership_id) WHERE state <> 'checked_out';
CREATE TABLE attendance_events(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, membership_id uuid NOT NULL,
 session_id uuid NOT NULL, point_id uuid NOT NULL, action text NOT NULL CHECK(action IN ('check_in','check_out','start_break','end_break')),
 recorded_at timestamptz NOT NULL, idempotency_key text NOT NULL, request_hash text NOT NULL,
 latitude double precision, longitude double precision, accuracy_m double precision, measured_at timestamptz,
 verification text NOT NULL, distance_m double precision,
 UNIQUE(membership_id,idempotency_key), FOREIGN KEY(company_id,session_id) REFERENCES attendance_sessions(company_id,id),
 FOREIGN KEY(company_id,membership_id) REFERENCES memberships(company_id,id), FOREIGN KEY(company_id,point_id) REFERENCES attendance_points(company_id,id)
);
CREATE TABLE requests(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, membership_id uuid NOT NULL,
 kind text NOT NULL CHECK(kind IN ('correction','leave')), issue text NOT NULL, start_date date NOT NULL, end_date date NOT NULL,
 requested_in timestamptz, requested_out timestamptz, reason text NOT NULL,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected','cancelled')),
 reviewer_id uuid REFERENCES memberships, review_reason text, reviewed_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(end_date >= start_date), FOREIGN KEY(company_id,membership_id) REFERENCES memberships(company_id,id)
);
CREATE TABLE audit_logs(
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, actor_id uuid REFERENCES users,
 action text NOT NULL, target text NOT NULL, reason text NOT NULL DEFAULT '', data jsonb NOT NULL DEFAULT '{}',
 request_id text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE notifications(id uuid PRIMARY KEY DEFAULT gen_random_uuid(), company_id uuid NOT NULL REFERENCES companies, membership_id uuid NOT NULL, message text NOT NULL, read_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), FOREIGN KEY(company_id,membership_id) REFERENCES memberships(company_id,id));
CREATE INDEX attendance_company_date ON attendance_sessions(company_id,work_date);
CREATE INDEX events_session ON attendance_events(session_id,recorded_at);
CREATE INDEX requests_tenant_status ON requests(company_id,status);
CREATE INDEX audit_tenant_time ON audit_logs(company_id,created_at DESC);
CREATE FUNCTION protect_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'immutable record'; END $$;
CREATE TRIGGER immutable_events BEFORE UPDATE OR DELETE ON attendance_events FOR EACH ROW EXECUTE FUNCTION protect_immutable();
CREATE TRIGGER immutable_audit BEFORE UPDATE OR DELETE ON audit_logs FOR EACH ROW EXECUTE FUNCTION protect_immutable();
CREATE FUNCTION protect_request_decision() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.status <> 'pending' THEN RAISE EXCEPTION 'decision is immutable'; END IF; RETURN NEW; END $$;
CREATE TRIGGER immutable_decisions BEFORE UPDATE ON requests FOR EACH ROW EXECUTE FUNCTION protect_request_decision();
