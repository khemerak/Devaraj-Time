# API guide

The exact implemented routes, input structures, statuses and cookie security scheme are in [OpenAPI 3.1](openapi.json), generated reproducibly by `node scripts/generate-openapi.mjs`. The Go HTTP implementation is authoritative when changing behavior; update the generator/contract with it.

All routes are under `/api`. Login/reset/enrollment require `Origin: <PUBLIC_URL>`. A successful administrative login or signed passkey assertion sets an opaque HttpOnly session cookie; read `/auth/me` for identity/company/CSRF. Authenticated POST/PATCH operations also require `X-CSRF-Token`. Public registration returns 404. See [the system update](system-update.md) for new security workflows.

| Area | Routes |
| --- | --- |
| Readiness | GET /health |
| Identity | POST /auth/login, /auth/logout, /auth/company, /auth/password; GET /auth/me; PATCH /auth/profile |
| Employee passkeys | POST /auth/enroll/begin, /auth/enroll/finish, /auth/passkey/begin, /auth/passkey/finish; GET /devices; POST /devices/invitations, /devices/{credential}/revoke |
| Recovery | POST /auth/recovery, /auth/reset, /auth/recovery-email, /auth/verify-recovery |
| Platform | GET/POST /platform/companies; PATCH /platform/companies/{id}; GET/PATCH /platform/config |
| Scheduling | GET/POST /schedules |
| Personal configuration | GET /organization |
| Admin assignment choices | GET /references (admin/HR) |
| Dashboard | GET /dashboard |
| Attendance | GET /points/{public}; GET/POST /attendance; GET /attendance/{id} |
| Requests | GET/POST /requests; POST /requests/{id}/review, /requests/{id}/cancel |
| Organization | GET/POST /admin/employees, departments, branches, shifts, workplaces, points; PATCH corresponding /{id} |
| QR rotation | POST /admin/points/{id}/rotate |
| Settings | PATCH /settings |
| Reporting | GET /reports?from=YYYY-MM-DD&to=YYYY-MM-DD&format=csv or xlsx |
| Transparency | GET /audit, /notifications; POST /notifications/read |

List responses include `items`, `page`, `has_more`; default page size 25, maximum 100. A full final page can produce a false-positive `has_more`, followed by an empty page. Reference/QR UI choices currently stop at 100 records. Attendance date range is limited to 366 days and supports `status`, `search`, `mine=true`. Managers' department filters and employee self-only filtering are applied server-side.

Clock payload example (timestamps are **location evidence only**):

```json
{
 "point": "43-character-public-identifier-from-your-printed-code",
 "action": "check_in",
 "idempotency_key": "a-new-random-key-for-this-confirmed-action",
 "location": {
  "latitude": 11.5564,
  "longitude": 104.9282,
  "accuracy": 10,
  "measured_at": "2026-10-13T06:00:00Z"
 }
}
```

Do not reuse the illustrative location reading: server checks freshness. `location` is required for geofenced workplaces. Accepted submissions return 201 with the committed event, session and server timestamp. Identical retries return 200 with the same event. Replays with a changed body return 409. Coordinate evidence is not returned through ordinary detail APIs.

Errors return `error` and `request_id`. 401 means sign in; 403 means permission/CSRF/origin failure; 404 means no resource inside authorized scope; 409 means state/replay/review/resource conflict; 422 means location or policy rejection; 429 means rate limiting. A 500 does not prove a write failed before commit: retry attendance with the **same** confirmed body/key. The UI retains that body in memory while a submission outcome is uncertain.

Attendance `state` differs from lateness `status`. Requests are pending/approved/rejected/cancelled. Automatic attendance uses `action: auto`; the server selects check-in/out under lock and suppresses repeated scans within two minutes. Simulated clock/GPS and direct raw-scan editing are not exposed. Recovery and platform endpoints have their separate authorization rules.
