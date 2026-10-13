# Architecture

The system has one Go API, one PostgreSQL database and a static React client served through a same-origin Nginx proxy. Caddy adds public HTTPS for deployment. No Redis, paid map service or external identity provider is required.

`internal/auth` owns password hashing and secure token generation. `internal/attendance` owns pure deterministic location/state/time calculations. `internal/server` contains separated HTTP modules for identity, administration, attendance, requests and reports; parameterized pgx SQL is explicit and typed where decisions/calculations occur. PostgreSQL enforces foreign-resource tenant consistency, unique attendance work dates and one open session. `internal/database` manages the separate location retention lifecycle. This version does not use sqlc-generated queries; JSON projection queries support list views.

Identity comes from a SHA-256 digest of a random session token stored server-side. A session selects a company but only active membership proves authorization. Identity is looked up on every request, so deactivation and role changes apply immediately. Read queries derive employee/team/department scope from the identity. Membership locks serialize attendance and reviewer corrections/leave conflicts. All critical mutations and corresponding audit events commit together. Login/logout audit writes are best effort and failures are logged.

```mermaid
sequenceDiagram
 participant Employee
 participant React
 participant API
 participant PostgreSQL
 Employee->>React: Explicitly scan printed QR
 React->>API: Resolve opaque attendance point
 API->>PostgreSQL: Match active company / assigned branch
 API-->>React: Workplace and location policy
 React->>Employee: Request camera and fresh location
 Employee->>React: Confirm action
 React->>API: Point + action + fresh GPS + idempotency key + CSRF
 API->>PostgreSQL: Begin transaction; lock membership
 API->>API: Verify location and policy; validate state
 API->>PostgreSQL: Insert event/evidence; update summary; append audit/notification
 PostgreSQL-->>API: Commit
 API-->>React: Confirm authoritative timestamp
 React-->>Employee: Success and attendance details
```

The browser never calculates authoritative attendance results. A network retry reuses the exact confirmed payload and key. Reusing a key with a different payload fails. Raw events remain preserved while summaries reflect approved corrections. Full-day leave approval rejects overlaps with attendance instead of creating contradictory records.

The UI polls dashboards every 15 seconds and notifications every 30 seconds. It uses TanStack Query invalidation after writes and lazy-loaded administration, scanner and records modules. English interface copy is currently embedded in components; extraction into locale dictionaries is a documented next step. It is not a complete internationalization implementation.

SMTP recovery, invitation-only WebAuthn credentials, platform provisioning/configuration and immutable effective-dated schedules now extend this architecture; see [the update](system-update.md). Storage/uploads, scheduled absence evaluation and external append-only audit exports remain absent.
