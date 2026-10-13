# Security model and review items

## Operational controls

- Argon2id with 64 MiB memory, two iterations, two lanes, 16 random salt bytes and a 32-byte key; constant-time comparison. The hash format currently represents one fixed parameter set. Parameter/version migration should precede tuning.
- Cryptographic 32-byte session/CSRF/QR tokens. Only SHA-256 session digests are stored; session tokens are never in QR codes or audit rows.
- HttpOnly, SameSite=Strict cookies; Secure enabled and HTTPS required in production. Twelve-hour absolute session expiry. Logout revokes the current session; changing a password revokes every session for that account. Membership activation/permissions are checked on every request.
- Exact PUBLIC_URL Origin and CSRF header verification for authenticated mutations. Login/reset/enrollment ceremonies require exact Origin. Public registration is disabled. No wildcard CORS. API/body size limit of 1 MiB, server input validation and parameterized SQL.
- In-process per-IP/per-route limits (20/minute for auth; 180/minute for other routes), sanitized error messages, request IDs and slog request logging. Request bodies, passwords and GPS are not emitted by the request logger.
- Tenant-derived authorization and composite company-resource FKs. Managers require matching departments; unassigned managers are restricted. HR cannot elevate a membership into a privileged role. Owners and one's own membership cannot be edited by employee administration. Self-approval is rejected.
- Membership row locking, one-open-session/one-work-date/unique-key constraints and immutable raw-event triggers. Critical audit entries are transactional with business changes. Raw events and audit rows cannot be updated/deleted through normal SQL without overriding database protection.
- Security headers on the API and Nginx; production Caddy HSTS/TLS. CSP disallows external scripts/frames and constrains connect/media/worker origins. React escapes text; CSV/XLSX exports protect text against formula execution.
- Precise location on geofenced attendance attempts, private evidence table and purge command, no coordinate exposure through employee/scanner endpoints and no sensitive service-worker/API caching. The QR entry flow submits after passkey/session and location verification. An unresolved request temporarily retains its exact payload in tab sessionStorage for safe reload retries.

## Remaining risks and missing controls

This is a tested release, not independently security-audited or automatically legally compliant. Browser location and permanent QR photographs can be fabricated. Geofence approval must not be treated as proof of presence or fraud detection. Employee identity now uses user-verified WebAuthn passkeys; a passkey may sync between devices and is not uniquely attested physical-phone proof.

Password reset, verified recovery emails, HR-issued enrollment invitations, credential revocation and explicit platform provisioning/activation are implemented; see [the system update](system-update.md). SMTP needs private operator configuration. Employees cannot use password login/reset. Administrative MFA and mandatory first-password change, primary email changes and existing-email membership onboarding remain incomplete. Public company signup is unavailable.

Rate limits are per-process and per-address, not distributed or durable. Expired limiter entries are pruned under load; there is no account-level lockout. Behind Nginx, Gin does not trust forwarded IPs by default, so limits share the proxy address. Configure a reviewed trusted-proxy policy before Internet traffic; never blindly trust client X-Forwarded-For.

Use a separate database owner/migration identity and least-privilege application login in production. Development uses a Docker database owner for convenience. Database administrators can change ordinary rows or disable immutability triggers. There is no audit hash chain or external WORM log. Login/logout audit failures are logged but do not block authentication. Failed-login attempts appear in request logs but are not company audit events. Administrative reasons are recorded, but full before/after JSON snapshots are not recorded yet.

Location erasure requires scheduling `purge-location`. It does not run as a background service automatically and does not erase backups. Account/request/audit retention and legal erasure tools are not implemented. Uploads and local/S3 private storage are not exposed. No claim is made about file security because uploads are disabled.

The demo/e2e data is fictional. Never load it into a production organization or use actual employee records in browser fixtures. End-to-end tests simulate GPS only in a disposable browser context and persist identifiable E2E tenants in development.

## Pre-release review

Review every new route's tenant and department scope; add tests for inactive users, policy edits, missing sessions and multi-membership switching. Perform an independent code/security review, dependency scans, real-device camera testing, and a jurisdiction-specific employment/privacy review. Add account recovery and full historical scheduling before making this system authoritative for payroll or disciplinary decisions.
