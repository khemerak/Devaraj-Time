# Invitation-only attendance and scheduling update — 13 October 2026

This update extends the existing Go/Gin/pgx application and preserves original attendance events, corrections, leave, reports, dashboards, company memberships and geofencing. Public signup and employee password login are removed.

## Test the employee process

1. Use the configured HTTPS origin on the administrator's device. Sign in as `admin@lotus.example` or `hr@lotus.example` with the private `DEMO_PASSWORD` from `.env`.
2. Set the workplace to the actual entrance's latitude/longitude and verify its radius/accuracy threshold. `Use this device's location` fills coordinates; an administrator must save them with a reason. A readable address or Plus Code alone does not change the geofence.
3. Create an employee under Employees. Employee passwords are unused and cannot authenticate; only Admin/HR require an initial password. Assign the employee's branch and an initial shift.
4. Open Employee devices. Select that employee, enter an identity-verification/replacement reason and issue an invitation. Every invitation revokes prior credentials and previous unused invitations for that membership. Verify the recipient and share this confidential link privately.
5. On the employee's phone, open the invitation in the browser used for attendance. Create a passkey using the OS authentication prompt. Invitations expire in 24 hours by default (platform-configurable 1–72 hours) and are consumed by successful enrollment. The enrollment token is carried in a URL fragment and removed from the visible address after loading.

Enrollment clears any prior browser login, including an HR/admin session on a borrowed/testing phone. The employee QR page requires an employee/manager credential-backed identity; an administrative session redirects to the employee passkey prompt rather than recording attendance under the administrator's name.
6. In QR codes, choose View & print and Print sheet. The code embeds the canonical `PUBLIC_URL/scan/<opaque-id>`, even when the administrator accessed the app through localhost. Previously generated ID-only codes work in the optional in-app scanner, but must be reprinted to open the attendance page through the phone's normal Camera app.
7. Open the phone Camera, scan the paper, and follow its link. An existing credential-backed 12-hour session is reused; otherwise tap Continue with passkey and verify with the OS. Allow precise location when requested. The server records the event automatically after authentication and location checks.
8. The first eligible scan checks in; the next scan after the duplicate suppression window checks out. Automatic scans within two minutes return the existing result. Open breaks must be ended in optional self-service before automatic checkout. Explicit break/check-out actions remain available there.
9. To test lost-device recovery, HR revokes the credential or issues a new invitation after verifying the employee. Revocation makes sessions unusable immediately and prevents new assertions. There is no password-reset fallback for employee enrollment.

Precise location is requested for each attempted geofenced event. Browser/OS permissions and Face ID/Touch ID/PIN can require interaction; silent permission is not promised. A QR photograph identifies only a point and cannot authenticate an employee or satisfy GPS policy.

Location acquisition watches fresh high-accuracy readings and periodically requests another fix, for up to 45 seconds. Coarse first readings are not immediately submitted. The screen shows the reported accuracy and the workplace threshold; readings older than 15 seconds are rejected by the client without changing their provider timestamp. Monitoring stops on success, timeout, denied permission or screen exit. The server still independently enforces freshness, accuracy and the boundary. GPS hardware and OS permissions can prevent a qualifying fix; web code cannot override that. No accuracy threshold was increased by this update.

Passkeys do not specifically mandate Face ID, but the OS decides whether biometrics or a device passcode can unlock a credential. Apple Stolen Device Protection can deny passcode fallback for Keychain/passkeys. The application cannot bypass that policy. Lost/broken phones require HR credential revocation and verified re-enrollment on a replacement device. If access cannot be restored immediately, HR should retain an in-person-verified manual arrival/departure record and later approve the employee's correction after access is restored. A dedicated HR-assisted live attendance entry and reception kiosk are not yet implemented; an inaccessible phone must not be treated as proof of absence.

## Hostnames and credential limits

Use a stable HTTPS domain before enrolling real staff or permanently printing QR sheets. WebAuthn credentials are bound to the RP hostname (not the port). A temporary Cloudflare hostname changing requires new employee enrollment and reprinted link QR sheets. The old opaque point stays unchanged unless explicitly rotated.

Passkeys use cryptographic WebAuthn assertions with required user verification. Apple/Google can sync passkeys between devices; this release recognizes a credential, **not a uniquely attested physical phone**. Revoking a credential revokes every synced copy. Device-bound-only enforcement through authenticator attestation/backup-eligibility restrictions, hardware inventory, enforced MFA for administrative passwords and enterprise credential-provider policy are not implemented. No IP/GPS/browser-fingerprint/stored-ID authentication is used.

Credential expiry is one year by default (platform-configurable 7–365 days); credential-backed sessions expire after twelve hours. Configuration changes affect new credentials/invitations, preserving existing expiry dates. The database rechecks credential expiry/revocation, membership activity and company activation. Challenges expire after five minutes and are consumed before signature verification. Private keys remain with the authenticator; the server stores credential records/public keys.

## Provision companies and test tenant isolation

The development platform administrator created for this update is `platform@devaraj.example`, using local `.env` `DEMO_PASSWORD`. This is fictional development access and must not be reused in production. Owners/admins/HR use Administrative login. Employees/managers use enrolled passkeys.

For a fresh installation, set private `PLATFORM_ADMIN_EMAIL` / `PLATFORM_ADMIN_PASSWORD` process environment values and run:

```powershell
docker compose run --rm --no-deps -e PLATFORM_ADMIN_EMAIL -e PLATFORM_ADMIN_PASSWORD api -command bootstrap-admin
```

This explicit CLI creates a platform account and refuses an existing email; ordinary server startup never bootstraps one. The platform administrator can provision a company and owner, list companies, edit activation and record subscription status (`trial`, `active`, `suspended`, `cancelled`). Deactivation revokes company sessions. Platform configuration edits bounded invitation/credential lifetimes and support email with an audit record. Billing/payment collection and arbitrary runtime environment changes through the UI are absent; SMTP and deployment configuration are managed through private environment variables.

Test `admin@lotus.example` and `admin@riverstone.example` in separate browser profiles. Each sees only its own company. Platform provisioning supports Gmail/personal addresses; the test suite provisions a Gmail owner and verifies membership-based scoping. Email domains never determine tenants. Platform administrators use explicit platform endpoints and do not inherit unrestricted company-resource API access.

`POST /api/auth/register` is absent and returns 404. Company creation is available only at authenticated `POST /api/platform/companies`. A manipulated membership/company/resource ID does not override the authenticated scope. Cross-company foreign keys reject mixed assignments.

## HR and Admin scheduling

HR has the same shift-creation/editing and schedule-assignment permissions as company owners/admins, while company settings/workplaces/QR management remain owner/admin functions. HR cannot access platform functions or promote employees into privileged roles.

Shifts are templates: editing a template records before/after audit data and does not silently rewrite employees' snapshotted assignments. Use Scheduling to apply the updated template to one or multiple employees. Choose effective-from (today or later), optional inclusive effective-to, and a reason. An ordered cycle can contain 1–28 shifts or `rest` entries. Multiple entries repeat daily from effective-from; a single template follows its workdays. For a rotating cycle's every-day entries, configure those templates for all seven weekdays so rotation/rest entries control days off.

Assignment records are append-only and store affected membership, old assignment, new shift snapshots, actor, timestamp, effective dates and reason. Latest-created applicable assignment wins. A temporary override ending reveals the underlying assignment again. An ongoing replacement masks the older assignment from its effective date. Editing old assignment rows is prohibited by database triggers.

Attendance resolves the snapshot for its work date, including overnight sessions associated with yesterday. An open attendance session keeps its original policy snapshot; changes affect subsequent sessions. Missing-session correction reviews now resolve the historical assignment for their requested date. Existing history predating this migration keeps its original attendance policy; initial scheduling ledger entries preserve the shift present when the migration ran and do not reconstruct unrecorded past assignment changes.

Employee/department/branch changes still use current memberships; their historical effective-date administration is outside this update. Holidays, split/flexible shifts, schedule collision warnings and explicit DST ambiguous-wall-time business rules remain incomplete. Company-wide policy changes are versioned but are not a fully effective-dated company-policy ledger.

## Administrative password recovery

Configure in private `.env`, then recreate the API:

```dotenv
SMTP_HOST=smtp.your-provider.example
SMTP_PORT=587
SMTP_FROM=attendance@your-company.example
SMTP_USER=your-provider-user
SMTP_PASSWORD=your-private-provider-password
PASSWORD_RESET_TTL=15m
```

SMTP requires STARTTLS with certificate verification and bounded network timeouts. Reset-request delivery runs asynchronously so SMTP latency does not expose account existence. It is not a durable mail queue: process restarts or provider outages can interrupt delivery. Implicit TLS on port 465 and plaintext SMTP are not implemented. No real provider credentials were supplied or real mailbox delivery tested in this workspace.

Forgot password accepts the registered administrative email or an already verified personal recovery address. It returns the same generic response for eligible, missing and ineligible accounts. A reset request cannot supply another delivery destination. Random 32-byte tokens are stored as hashes, expire after configurable 1–60 minutes (default 15), can be consumed once, and invalidate every session and outstanding password reset token. The link carries its token as a fragment. Confirmation email follows a successful change. Auth routes are rate-limited; outstanding reset tokens also throttle by account.

Profile → Verify recovery email requires the current password. A separate short-lived verification link must be opened and consumed before that destination is eligible. An unverified recovery email never receives password reset mail. Verified destinations are unique. Primary email changes, existing-user cross-company invitation onboarding, mandatory first administrative-password change, distributed account lockout and bounce/delivery-status administration remain incomplete.

Recovery email destinations are not changed by enrollment recovery. Regular employee accounts cannot obtain password-reset links or use password login.

## Database migrations and deployment

Apply `docker compose run --rm --build migrate` before starting updated API/web containers. Do not delete the PostgreSQL volume. Migrations:

- `000003_enrollment_schedules`: activation/subscription status, platform-admin marker, verified recovery fields, nullable company for platform sessions, credential-linked sessions, credential/invitation/ceremony/recovery tables, platform audit, and immutable schedule assignments. It copies existing shift assignments into snapshots without overwriting users, memberships, events or reports.
- `000004_verified_recovery_email`: uniqueness of verified personal recovery destinations.
- `000005_platform_configuration`: bounded enrollment lifetime and support configuration editable only by the platform administrator.

The down migrations exist for development rollback, but dropping new auth/scheduling tables loses their added data; do not run down migrations against a deployed organization without a reviewed backup/restore procedure.

Use separate database migration and runtime identities in production. In addition to original event/audit protections, revoke UPDATE/DELETE on `schedule_assignments` and `platform_audit` from runtime credentials. The platform-admin CLI needs operator-level DB access and is an explicit privileged provisioning mechanism.

## Files changed

Backend: `cmd/api/main.go`; `internal/server/server.go`, `admin.go`, `attendance.go`, `requests.go`, `profile.go`; new `devices.go`, `recovery.go`, `platform.go`, `schedules.go`, `update_test.go`; updated integration tests, Go dependency manifests and migrations 000003–000005.

Frontend: `App.tsx`; new `pages/Entry.tsx`, `pages/Management.tsx`, `lib/passkey.ts`, `lib/passkey.test.ts`; updated `pages/Admin.tsx`, `pages/Scan.tsx`, `pages/Records.tsx`, `components/ProfileControls.tsx`, CSS and Playwright attendance suite. The obsolete public-registration E2E fixtures were replaced by platform provisioning and actual virtual-authenticator WebAuthn ceremonies.

Operations/docs: Docker Compose SMTP environment, `.env.example`, README, API schema generator/OpenAPI, system-update guide and revised security/deployment/limitations notes. Local private `.env` records the active test origin.

## Verification

The PostgreSQL suite runs all five migrations in a random disposable schema and drops only that schema. It tests existing attendance/geofence/transition/concurrency/correction/leave/export behavior plus cross-company and Gmail ownership, disabled registration, employee password refusal, HR/Admin schedule permissions, platform denial/configuration, future/temporary snapshots and immutable history, invitation/challenge expiry/reuse, revoked/expired credential-backed sessions, automatic check-in/out and duplicate suppression, and registered/verified recovery email with reset expiry/one-use protection.

Vitest tests API behavior/QR decoding and denied/fresh geolocation. Playwright uses a Chrome virtual CTAP2 resident, user-verified authenticator and actual backend signature validation for enrollment/passwordless login; clears cookies to prove passkey reauthentication; tests repeated scans, out-of-bound coordinates, lost-device revocation, and permission denial. A further lost-response/reload test checks exact-payload idempotent retry.

Physical iPhone Face ID/iCloud browser handling, printed paper decoding/lighting, real SMTP inbox delivery, production domain/TLS, restored backups and independent security/accessibility reviews still require operator/device validation. Test accounts are fictional and remain in the development database.

Automatic attendance has a two-minute repeated-scan window. A second scan after that window intentionally toggles an open checked-in session to check-out; there is no proof of the person's intent. For a committed request whose response is lost, the pending exact payload/key is kept in tab sessionStorage and reused on reload until success or a definitive 4xx rejection. This includes only that attempt's location and is cleared on resolution or tab closure; it is not an offline attendance record.
