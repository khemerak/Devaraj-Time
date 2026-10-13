# Implementation checklist

- [x] PostgreSQL schema and two versioned migrations
- [x] Opaque sessions, Argon2id, CSRF, tenant and department authorization
- [x] Employee, department, branch, workplace, shift and QR administration
- [x] Transactional attendance, geofencing, idempotency and immutable events
- [x] Policy snapshots and deterministic calculations
- [x] Closed, open and missing-session corrections and immutable review decisions
- [x] Full-day leave requests, attendance conflicts, notifications, audit and CSV/XLSX reports
- [x] Separate private location evidence and explicit retention purge
- [x] Responsive React application, account controls, PWA manifest and offline notice
- [x] Unit and isolated-schema PostgreSQL integration tests
- [x] Docker builds, explicit migration/seed, running application and operating documentation
- [x] Browser workflow implemented; final execution result in docs/verification.md

The boxes refer to the original implemented core scope. The [system update](docs/system-update.md) adds invitation-only passkeys, platform provisioning/configuration, SMTP password recovery and effective-dated scheduling. Missing-checkout evaluation, uploads, leave balances, advanced reports and production review remain incomplete.

Scope gaps and verification limits are tracked in docs/limitations.md.

