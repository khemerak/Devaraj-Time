# Executed verification — 13 October 2026

These results describe commands actually executed in this workspace, not production certification.

## Invitation-only system update

Updated PostgreSQL tests pass with migrations 000001–000005 in an isolated schema. New coverage includes Gmail membership scoping, denied cross-company reads/writes, removed signup and employee password login, HR/Admin scheduling and platform denial, future/temporary/immutable snapshots, invitation/challenge reuse and expiry, revoked/expired credential sessions, automatic check-in/out and rapid repeats, reset expiry/one-use/destination restrictions, and platform enrollment configuration. Existing correction/leave/export/concurrency tests continue passing. Go vet passes.

Frontend build passes with no chunk-size warning; Vitest has 6 passing tests. The replacement Chrome/Playwright suite has 2 passing tests using real signed ceremonies from a virtual CTAP2 resident, user-verified authenticator: one-time enrollment, passwordless reauthentication after cookie clearing, duplicate suppression, a response lost after commit followed by safe reload/replay, out-of-bound rejection, device revocation, location denial, and signup absence.

Updated govulncheck: 0 reachable vulnerabilities, 0 vulnerabilities in imported packages; one unused dependency-module advisory remains. Real iPhone/printed-camera behavior and real SMTP inbox delivery remain untested. SMTP recovery transport/destinations/token consumption are verified through injected test mail delivery.

Location-acquisition update: frontend build and 10 Vitest tests pass. New tests cover coarse-to-precise fixes, persistent low accuracy, stale precise readings, location permission denial and watch cleanup/cancellation. The backend thresholds remain unchanged. Real iPhone GPS/precision behavior still needs physical-device verification.

The table below records the original release baseline; its registration/password employee flows were subsequently replaced by the invitation-only workflow described above. See [the current update guide](system-update.md).

| Check | Result |
| --- | --- |
| Go unit tests | Passed: geofence, stale/future GPS, accuracy, Haversine/boundary, states, grace, break/early/overtime, overnight/DST, workdays, snapshots, Argon2id/tokens, cookies, Origin/session gate, RBAC and safe exports. |
| PostgreSQL integration | Passed against a running PostgreSQL 17 container, in a disposable random schema: registration/login/expiry, roles, CSRF, tenant and manager department boundaries, employee/resource creation, QR/geofence, idempotency replay/body mismatch, invalid transitions, eight concurrent check-ins with one commit, correction decisions/immutability, forgotten-checkout resolution without fabricated scans, leave review/conflicts, CSV/XLSX, QR rotation, repeatable seed, location erasure preserving events, profile update and password change revoking every session. |
| `go vet ./...` | Passed. |
| React TypeScript + Vite production build | Passed. Final source splits records, administration, scanner and charts; no chunk-size warning. |
| Vitest | 4 tests passed: CSRF/rejection behavior, offline failure, own-installation QR parsing and duration display. |
| Playwright in installed Chrome | **2 tests passed.** Full workflow: disposable admin company → UI shift/employee/workplace/QR → employee login → browser GPS fixture → committed check-in/check-out → HR visibility → correction submission/approval → adjusted employee details with original scans → audit trail. Phone test: 390px navigation, Escape dismissal, schedule/scanner access and confirmation disabled before point resolution. |
| Native migration and seed | Passed; seed initially exposed a SQL parameter-type bug, corrected and regression-tested. |
| Docker builds | Go 1.27.2 backend and Node 24 frontend multi-stage images built and ran successfully. API, db and web reported healthy. |
| Documented Docker commands | Migration replay, repeatable seed and location purge all returned success. |
| Production Compose | Both compose files together passed `config --quiet`; no secrets printed. |
| HTTP readiness/security headers | Web root returned 200 with CSP, frame protection and nosniff headers; proxied API readiness returned ok. |
| npm production dependency audit | 0 vulnerabilities reported. Dependency installation also reported 0 vulnerabilities across installed packages. |
| Go vulnerability scan | Initial scan found standard-library and QUIC issues. Go was raised to 1.27.2 and quic-go to 0.59.1. Final govulncheck returned 0 reachable vulnerabilities and 0 vulnerable imported packages. It reports GO-2026-5932 for the unused, unmaintained `golang.org/x/crypto/openpgp` package inside a required module. The application imports Argon2id, not OpenPGP; no fix is available for that unrelated package. |
| Browser visual QA | Administrator sign-in, database dashboard, phone-width navigation/scanner and dark-theme rendering inspected in the in-app browser. Original light-theme desktop dashboard also inspected. |

Windows sandbox application control initially blocked a Go test executable; the later permitted runs completed successfully, including after upgrading the native project toolchain to Go 1.27.2. Network sandboxing initially blocked dependency/audit downloads; approved network runs completed. Docker Desktop was initially stopped and was started for database/deployment verification.

Playwright initially found ambiguous accessible field labels caused by help text. Explicit field names fixed them. A later test selector matched both the employee account link and attendance link; the test was scoped to the attendance table. Vitest was explicitly scoped to `src` to avoid executing Playwright suites. Final successful executions are the reported results.

## Test mechanisms

The Go test server uses actual Gin/pgx/PostgreSQL logic through httptest, not a mocked store. Its random schema is dropped on completion. The Playwright test uses real HTTP writes in a visibly named fictional E2E company on the local development database; those E2E companies remain for inspection. Simulated browser geolocation and printed-URL QR input are test-only fixtures. No server GPS bypass, mock clock or production simulation endpoint was added.

## Unrun or limited verification

- Physical iOS/Android camera decoding, camera-denied OS prompts, torch hardware, real GPS reception and PWA installation.
- Production domain/certificate issuance, hardened separate database users, encrypted off-host backup and destructive restore.
- Load/stress testing, independent penetration testing and exhaustive authorization matrices for every resource.
- Formal screen-reader/accessibility audit and all timezone/DST ambiguous-wall-time rules.
- Features explicitly absent in [limitations](limitations.md).

Browser/API GPS is spoofable. Tests prove the implemented calculations and authorization/transaction behaviors under tested inputs; they do not prove physical employee presence or legal compliance.
