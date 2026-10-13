# Deployment and operations

## Environment

| Variable | Purpose |
| --- | --- |
| DATABASE_URL | PostgreSQL connection URI; required by API/CLI. URI-encode special password characters. Docker Compose builds its internal URI from POSTGRES_* variables. |
| POSTGRES_USER / POSTGRES_DB / POSTGRES_PASSWORD | Development database bootstrap; strong random password required. |
| PUBLIC_URL | Exact browser origin with scheme, host and optional port; no trailing slash/path. Origin/CSRF checks depend on this. |
| APP_ENV | development or production; production rejects insecure origin/cookies and disables demo seeding. |
| COOKIE_SECURE | true in production, false only for local HTTP development. |
| DEMO_PASSWORD / DEMO_TIMEZONE | Explicit development-only provisioning, minimum 12-character password, IANA timezone. |
| HTTP_ADDR | Native API listener; default :8080. |
| MIGRATIONS_PATH | CLI migration folder; default migrations. |
| DOMAIN / ACME_EMAIL | Caddy TLS domain and certificate contact. |
| TEST_DATABASE_URL | Disposable local test database; tests create/drop an isolated schema. |
| E2E_URL | Optional Playwright web origin; default http://localhost:5173. |

Opaque sessions do not require a shared signing secret. SMTP password recovery uses `SMTP_HOST`, `SMTP_PORT` (587), `SMTP_FROM`, `SMTP_USER`, `SMTP_PASSWORD` and `PASSWORD_RESET_TTL` (15m). STARTTLS and valid certificates are mandatory. A stable HTTPS `PUBLIC_URL` is needed for employee passkeys and permanent printed link QR codes. See [the system update](system-update.md).

## Docker

Development: start db, run explicit migrations/seed, then start api/web as in README. Backend/frontend multi-stage builds run as non-root users. Health checks cover PostgreSQL readiness, API DB ping and frontend HTTP availability. The database volume persists across restarts. Never use `docker compose down -v` unless deliberately destroying development data.

Production requires a real DNS domain resolving to the server, reachable TCP 80/443, strong credentials, backups and a reviewed deployment. Set APP_ENV=production, COOKIE_SECURE=true, DOMAIN and ACME_EMAIL. The production override hides database/API/web host ports and adds Caddy. Use both compose files for **all** production commands. Do not run the development seed. Production HTTPS issuance has not been exercised against a real domain in this workspace.

```sh
docker compose -f docker-compose.yml -f docker-compose.prod.yml up --build -d db
docker compose -f docker-compose.yml -f docker-compose.prod.yml run --rm migrate
docker compose -f docker-compose.yml -f docker-compose.prod.yml up --build -d api web caddy
docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
docker compose -f docker-compose.yml -f docker-compose.prod.yml logs --tail=100 api
```

Create the first platform administrator with the explicit `bootstrap-admin` CLI using private environment credentials. Provision companies/owners through Platform administration. Public registration is absent. HR issues one-time employee passkey invitations; share those confidential links after verifying the employee. Initial passwords are used only by company owners/admins/HR. See [provisioning and enrollment instructions](system-update.md).

## Database privileges

For production, use distinct migration owner and runtime user. Provision the runtime login manually using your database secret manager or interactive `psql \password`; never paste a password into a committed SQL script. After migrations, use the same schema as the API:

```sql
CREATE ROLE devaraj_app LOGIN;
-- Set its private password using your approved database administration process.
GRANT CONNECT ON DATABASE devaraj TO devaraj_app;
GRANT USAGE ON SCHEMA public TO devaraj_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO devaraj_app;
REVOKE UPDATE, DELETE ON attendance_events, audit_logs, schedule_assignments, platform_audit FROM devaraj_app;
REVOKE DELETE ON requests, attendance_sessions FROM devaraj_app;
REVOKE ALL ON schema_migrations FROM devaraj_app;
```

The schema uses random UUIDs, so there are no serial sequences to grant. Give the API/retention CLI the runtime DATABASE_URL. Give only the migration job a separate owner DATABASE_URL through a deployment-specific override or secret injection. Run every future migration with the owner and review/reapply least privilege afterward. The bundled development Compose uses one DB identity; changing to separate production credentials is an operator task and has not been tested here.

## Backups and restore

Take encrypted off-host backups with PostgreSQL `pg_dump -Fc`, retain according to policy, and test restores into a separate database. On Windows, avoid binary shell redirection; create the dump inside the container and copy it out:

```sh
docker compose exec db sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc -f /tmp/devaraj.backup'
docker compose cp db:/tmp/devaraj.backup ./devaraj.backup
```

For restore, provision an empty separate database, copy the backup into the DB container, and run `pg_restore` using explicitly reviewed target database credentials. Do not use `--clean` against a live system. Restrict access to backups; they contain employee identities and request reasons. Exact retention erasure from backups requires a separate policy. No destructive restore was run during this task.

## Migrations and retention

Apply migrations explicitly before starting a changed API. Back up first. `-command migrate` uses golang-migrate version tracking and treats no-change as success. A failed migration requires reviewing database state before repairing its dirty flag. Never force a version blindly.

Schedule this daily on the deployment host using its normal job scheduler (no scheduler installed by this repository):

```sh
docker compose -f docker-compose.yml -f docker-compose.prod.yml run --rm api -command purge-location
```

It deletes private evidence older than each company's configured location-retention days and appends an audit count. It preserves immutable raw clock events, timestamps and verification result. Monitor nonzero exit codes. For native development, run `./scripts/backend.ps1 purge-location`.

## Monitoring and review

Use API readiness, structured container logs, database metrics, backup alerts and resource limits suited to your host. Restrict database networks, update pinned dependencies/images deliberately, test migration/rollback plans, configure a reviewed proxy IP policy, and complete independent security/accessibility/legal reviews. No production autoscaling, deployment rollback orchestration, SMTP, S3 or distributed throttling is included.
