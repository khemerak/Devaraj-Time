import { mkdirSync, writeFileSync } from "node:fs";
const str = (extra = {}) => ({ type: "string", ...extra }),
  integer = (min, max) => ({ type: "integer", minimum: min, maximum: max }),
  boolean = { type: "boolean" },
  uuid = str({ format: "uuid" }),
  date = str({ format: "date" }),
  timestamp = str({ format: "date-time" });
const object = (properties, required = Object.keys(properties)) => ({
  type: "object",
  properties,
  required,
});
const reason = str({ minLength: 3, maxLength: 1000 }),
  longReason = str({ minLength: 10, maxLength: 2000 }),
  name = str({ minLength: 2, maxLength: 120 });
const schemas = {
  Error: object({ error: str(), request_id: str() }),
  Login: object({
    email: str({ format: "email", maxLength: 254 }),
    password: str({ minLength: 1, maxLength: 128 }),
  }),
  Register: object({
    company: name,
    name,
    email: str({ format: "email", maxLength: 254 }),
    password: str({ minLength: 12, maxLength: 128 }),
    timezone: str({ description: "IANA timezone", maxLength: 80 }),
  }),
  Location: object({
    latitude: { type: "number", minimum: -90, maximum: 90 },
    longitude: { type: "number", minimum: -180, maximum: 180 },
    accuracy: { type: "number", exclusiveMinimum: 0 },
    measured_at: timestamp,
  }),
  Clock: object(
    {
      point: str({ maxLength: 128 }),
      action: str({
        enum: ["auto", "check_in", "check_out", "start_break", "end_break"],
      }),
      idempotency_key: str({ minLength: 16, maxLength: 128 }),
      location: {
        anyOf: [{ $ref: "#/components/schemas/Location" }, { type: "null" }],
      },
    },
    ["point", "action", "idempotency_key"],
  ),
  ClockResult: object(
    {
      event_id: uuid,
      session_id: uuid,
      recorded_at: timestamp,
      action: str(),
      verification: str(),
      state: str(),
      status: str(),
      late_minutes: integer(0, 1440),
      workplace: str(),
    },
    ["event_id", "session_id", "recorded_at", "action", "verification"],
  ),
  Employee: object(
    {
      name,
      email: str({ format: "email", maxLength: 254 }),
      password: str({
        minLength: 12,
        maxLength: 128,
        description:
          "Required only on creation; existing identity fields cannot be changed here.",
      }),
      employee_code: str({ maxLength: 40 }),
      role: str({ enum: ["admin", "hr", "manager", "employee"] }),
      department_id: { anyOf: [uuid, { type: "null" }] },
      branch_id: uuid,
      shift_id: uuid,
      position: str({ maxLength: 120 }),
      active: boolean,
      start_date: date,
      reason,
    },
    [
      "name",
      "email",
      "employee_code",
      "role",
      "branch_id",
      "shift_id",
      "start_date",
      "reason",
    ],
  ),
  NamedResource: object({ name, reason }),
  Shift: object({
    name,
    start_time: str({ pattern: "^([01][0-9]|2[0-3]):[0-5][0-9]$" }),
    end_time: str({ pattern: "^([01][0-9]|2[0-3]):[0-5][0-9]$" }),
    grace_minutes: integer(0, 120),
    expected_minutes: integer(1, 1440),
    unpaid_break_minutes: integer(0, 240),
    early_window_minutes: integer(0, 720),
    overtime_enabled: boolean,
    workdays: { type: "array", minItems: 1, maxItems: 7, items: integer(0, 6) },
    reason,
  }),
  Workplace: object(
    {
      name,
      branch_id: uuid,
      address: str({ maxLength: 500 }),
      latitude: { type: "number", minimum: -90, maximum: 90 },
      longitude: { type: "number", minimum: -180, maximum: 180 },
      radius_m: { type: "number", minimum: 1, maximum: 10000 },
      max_accuracy_m: { type: "number", minimum: 1, maximum: 1000 },
      location_required: boolean,
      active: boolean,
      reason,
    },
    ["name", "branch_id", "radius_m", "max_accuracy_m", "reason"],
  ),
  Point: object({ name, workplace_id: uuid, active: boolean, reason }),
  Request: object(
    {
      kind: str({ enum: ["correction", "leave"] }),
      issue: str({ maxLength: 120 }),
      start_date: date,
      end_date: date,
      requested_in: timestamp,
      requested_out: timestamp,
      reason: longReason,
    },
    ["kind", "issue", "start_date", "end_date", "reason"],
  ),
  Review: object({
    decision: str({ enum: ["approved", "rejected"] }),
    reason: longReason,
  }),
  Settings: object({
    name,
    timezone: str({ maxLength: 80 }),
    breaks_enabled: boolean,
    correction_days: integer(1, 90),
    location_retention_days: integer(1, 365),
    reason,
  }),
  Identity: object({
    user_id: uuid,
    company_id: uuid,
    membership_id: uuid,
    name,
    email: str({ format: "email" }),
    company: str(),
    timezone: str(),
    role: str(),
    department_id: { anyOf: [uuid, { type: "null" }] },
    csrf: str(),
  }),
  Me: object({
    user: { $ref: "#/components/schemas/Identity" },
    companies: {
      type: "array",
      items: object({ id: uuid, name, role: str() }),
    },
  }),
  Page: object(
    {
      items: { type: "array", items: { type: "object" } },
      page: integer(1, 1000000),
      has_more: boolean,
    },
    ["items", "page", "has_more"],
  ),
};
const spec = {
  openapi: "3.1.0",
  info: {
    title: "Devaraj Time API",
    version: "0.1.0",
    description:
      "Same-origin cookie API. All tenant context comes from an authenticated active membership. Mutations require an exact PUBLIC_URL Origin and X-CSRF-Token returned by /auth/me. QR and browser GPS are layered evidence, not proof of presence.",
  },
  servers: [{ url: "/api" }],
  security: [{ session: [] }],
  paths: {},
  components: {
    securitySchemes: {
      session: { type: "apiKey", in: "cookie", name: "devaraj_session" },
    },
    schemas,
  },
};
const json = (s) => ({
  "application/json": {
    schema: s ? { $ref: "#/components/schemas/" + s } : { type: "object" },
  },
});
const param = (name, where = "query", schema = str(), required = false) => ({
  name,
  in: where,
  required,
  schema,
});
const pageParams = [
  param("page", "query", integer(1, 1000000)),
  param("limit", "query", integer(1, 100)),
];
const dates = [param("from", "query", date), param("to", "query", date)];
function add(
  path,
  method,
  summary,
  {
    body,
    response,
    success = "200",
    publicAccess = false,
    params = [],
    description = "",
  } = {},
) {
  const p = [...params];
  for (const match of path.matchAll(/\{([^}]+)\}/g))
    p.push(
      param(
        match[1],
        "path",
        ["public", "credential"].includes(match[1]) ? str() : uuid,
        true,
      ),
    );
  if (method !== "get") {
    p.push(param("Origin", "header", str(), true));
    if (!publicAccess) p.push(param("X-CSRF-Token", "header", str(), true));
  }
  const op = {
    summary,
    description,
    parameters: p,
    responses: {
      [success]: {
        description: "Successful response",
        content: json(response),
      },
      400: { description: "Invalid input", content: json("Error") },
      401: {
        description: "Sign in or session expired",
        content: json("Error"),
      },
      403: {
        description: "Authorization, Origin or CSRF denied",
        content: json("Error"),
      },
      404: {
        description: "Not found within authorized tenant scope",
        content: json("Error"),
      },
      409: {
        description: "State, replay, assignment or review conflict",
        content: json("Error"),
      },
      422: {
        description: "Location or attendance policy rejection",
        content: json("Error"),
      },
      429: { description: "Rate limit exceeded", content: json("Error") },
      500: {
        description: "Internal failure with correlation ID",
        content: json("Error"),
      },
    },
  };
  if (publicAccess) op.security = [];
  if (body)
    op.requestBody = {
      required: true,
      content:
        typeof body === "string"
          ? json(body)
          : { "application/json": { schema: body } },
    };
  spec.paths[path] ??= {};
  spec.paths[path][method] = op;
}
add("/health", "get", "Database readiness", { publicAccess: true });
add("/auth/login", "post", "Sign in and set HttpOnly session cookie", {
  body: "Login",
  publicAccess: true,
});
add(
  "/platform/companies",
  "post",
  "Platform administrator provisions company and owner",
  {
    body: "Register",
    success: "201",
  },
);
add(
  "/platform/companies",
  "get",
  "Platform administrator lists company activation and subscription status",
);
add(
  "/platform/companies/{id}",
  "patch",
  "Platform administrator updates activation and subscription",
  {
    body: object({
      active: boolean,
      subscription: str({
        enum: ["trial", "active", "suspended", "cancelled"],
      }),
      reason,
    }),
  },
);
add(
  "/public/config",
  "get",
  "Canonical attendance origin for printed QR URLs",
  { publicAccess: true },
);
add(
  "/auth/recovery",
  "post",
  "Generic password reset request to registered or verified destination",
  { publicAccess: true, body: object({ email: str({ format: "email" }) }) },
);
add(
  "/auth/reset",
  "post",
  "Consume expiring one-time reset token and revoke sessions",
  {
    publicAccess: true,
    body: object({
      token: str(),
      password: str({ minLength: 12, maxLength: 128 }),
    }),
  },
);
add(
  "/auth/verify-recovery",
  "post",
  "Consume recovery email verification token",
  { publicAccess: true, body: object({ token: str() }) },
);
add(
  "/auth/recovery-email",
  "post",
  "Password reauthentication and recovery email verification request",
  { body: object({ email: str({ format: "email" }), password: str() }) },
);
add(
  "/auth/enroll/begin",
  "post",
  "Begin passkey enrollment with authorized single-use invitation",
  { publicAccess: true, body: object({ token: str() }) },
);
add(
  "/auth/enroll/finish",
  "post",
  "Validate signed WebAuthn registration and consume invitation",
  {
    publicAccess: true,
    body: object({ ceremony: str(), response: { type: "object" } }),
  },
);
add(
  "/auth/passkey/begin",
  "post",
  "Begin user-verified discoverable passkey authentication",
  { publicAccess: true },
);
add(
  "/auth/passkey/finish",
  "post",
  "Verify signed passkey assertion and issue company-bound session",
  {
    publicAccess: true,
    body: object({ ceremony: str(), response: { type: "object" } }),
  },
);
add("/devices", "get", "List authorized company employee credentials");
add(
  "/devices/invitations",
  "post",
  "HR/Admin replace enrollment; revoke prior credentials",
  { success: "201", body: object({ membership_id: uuid, reason }) },
);
add(
  "/devices/{credential}/revoke",
  "post",
  "Revoke company employee credential",
  { body: object({ reason }) },
);
add("/schedules", "get", "Effective-dated immutable schedule assignments", {
  params: [param("membership_id", "query", uuid)],
});
add(
  "/schedules",
  "post",
  "HR/Admin assign snapshots or rotations individually or in bulk",
  {
    success: "201",
    body: object(
      {
        membership_ids: {
          type: "array",
          items: uuid,
          minItems: 1,
          maxItems: 200,
        },
        shift_ids: { type: "array", items: str(), minItems: 1, maxItems: 28 },
        effective_from: date,
        effective_to: { anyOf: [date, { type: "null" }] },
        reason,
      },
      ["membership_ids", "shift_ids", "effective_from", "reason"],
    ),
  },
);
add("/auth/me", "get", "Active identity and company memberships", {
  response: "Me",
});
add(
  "/platform/config",
  "get",
  "Platform enrollment lifetimes and support configuration",
);
add(
  "/platform/config",
  "patch",
  "Platform-only bounded enrollment configuration with audit",
  {
    body: object({
      invitation_hours: integer(1, 72),
      credential_days: integer(7, 365),
      support_email: str(),
      reason,
    }),
  },
);
add("/auth/logout", "post", "Revoke current session");
add("/auth/password", "post", "Change own password and revoke all sessions", {
  body: object({
    current_password: str({ maxLength: 128 }),
    new_password: str({ minLength: 12, maxLength: 128 }),
  }),
});
add("/auth/profile", "patch", "Update own display name in all memberships", {
  body: object({ name }),
});
add("/auth/company", "post", "Select an active company membership", {
  body: object({ id: uuid }),
});
add(
  "/organization",
  "get",
  "Own assignment and current company attendance policy",
);
add("/references", "get", "Assignment reference data", {
  description:
    "Company admin or HR only; returns up to 100 departments, branches, shifts and workplaces.",
});
add("/dashboard", "get", "Scoped metrics and seven-day trend", {
  description:
    "Managers see their department. Employees see themselves. No-record counts are not verified absences.",
});
add(
  "/points/{public}",
  "get",
  "Resolve an active QR point in the employee assigned branch",
);
add("/attendance", "post", "Confirm and commit a clock event", {
  body: "Clock",
  response: "ClockResult",
  success: "201",
  description:
    "An identical retry returns 200 with the same event. A changed payload with the same key is rejected. Server PostgreSQL clock is authoritative. Row locking serializes attendance per membership.",
});
spec.paths["/attendance"].post.responses["200"] = {
  description: "Idempotent replay of the original event",
  content: json("ClockResult"),
};
add("/attendance", "get", "Scoped effective attendance history", {
  response: "Page",
  params: [
    ...pageParams,
    ...dates,
    param("search"),
    param(
      "status",
      "query",
      str({ enum: ["on_time", "within_grace", "late"] }),
    ),
    param("mine", "query", boolean),
  ],
});
add(
  "/attendance/{id}",
  "get",
  "Summary, original events and correction decisions",
);
add("/requests", "get", "Scoped correction or leave requests", {
  response: "Page",
  params: [
    ...pageParams,
    param("kind", "query", str({ enum: ["correction", "leave"] })),
    param("mine", "query", boolean),
  ],
});
add(
  "/requests",
  "post",
  "Submit a personal correction or full-day leave request",
  {
    body: "Request",
    success: "201",
    description:
      "Correction requires requested_in and requested_out, a past interval of at most 24 hours and a single date within the company deadline. Proposed timestamps are never raw events.",
  },
);
add(
  "/requests/{id}/review",
  "post",
  "Approve or reject an authorized request",
  {
    body: "Review",
    description:
      "Company admin, HR or same-department manager; self-approval denied. Decisions are immutable. Approved corrections may resolve an open session without creating a fabricated scan. Leave conflicts with existing attendance or approved leave are rejected.",
  },
);
add("/requests/{id}/cancel", "post", "Cancel own pending request");
for (const [resource, body] of Object.entries({
  employees: "Employee",
  departments: "NamedResource",
  branches: "NamedResource",
  shifts: "Shift",
  workplaces: "Workplace",
  points: "Point",
})) {
  add("/admin/" + resource, "get", "List " + resource, {
    response: "Page",
    params: pageParams,
    description:
      "Configuration resources require owner/admin; employees/departments permit scoped team access.",
  });
  add("/admin/" + resource, "post", "Create " + resource, {
    body,
    description:
      resource === "employees"
        ? "Admin/HR; HR can only create employee-role accounts. New emails only."
        : "Company owner/admin.",
  });
  add("/admin/" + resource + "/{id}", "patch", "Update " + resource, {
    body,
    description:
      "Reason required. Company-scoped foreign assignments enforced. Owners and your own membership cannot be edited through employee administration.",
  });
}
add(
  "/admin/points/{id}/rotate",
  "post",
  "Invalidate old QR identifier and replace it",
  { body: object({ reason }) },
);
add("/settings", "patch", "Version the company attendance policy", {
  body: "Settings",
});
add("/notifications", "get", "Personal in-app notifications");
spec.paths["/notifications"].get.responses["200"].content[
  "application/json"
].schema = { type: "array", items: { type: "object" } };
add("/notifications/read", "post", "Mark own notifications as read");
add("/audit", "get", "Company audit trail", {
  response: "Page",
  params: pageParams,
  description: "Owner/admin only. Append-only at database trigger level.",
});
add("/reports", "get", "Export scoped effective attendance summaries", {
  params: [...dates, param("format", "query", str({ enum: ["csv", "xlsx"] }))],
  description:
    "Team roles only; managers department scoped. At most 366 days and 50,000 rows. Cells protected against formula injection. Date range and timezone included.",
});
spec.paths["/reports"].get.responses["200"] = {
  description: "CSV or Excel download",
  content: {
    "text/csv": { schema: str() },
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {
      schema: str({ contentEncoding: "binary" }),
    },
  },
};
mkdirSync(new URL("../docs", import.meta.url), { recursive: true });
writeFileSync(
  new URL("../docs/openapi.json", import.meta.url),
  JSON.stringify(spec, null, 2) + "\n",
);
