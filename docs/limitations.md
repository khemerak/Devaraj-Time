# Release scope, limitations and roadmap

This release completes a working foundation and core attendance workflow with selected transparency/management features. It does **not** satisfy every item in the master specification. The following facilities are intentionally absent or limited, and must not be represented as complete.

## High-priority gaps

1. **Identity lifecycle:** passkeys, enrollment invitations/revocation, verified recovery email, password reset and platform provisioning/activation are now implemented. Remaining: existing-user company invitation onboarding, mandatory initial administrative-password change, account lockout/administrative MFA and configurable custom roles. Synced passkeys do not prove a uniquely attested physical phone. SMTP needs operator configuration. See [the current system update](system-update.md).
2. **Unresolved sessions:** no automatic missing-checkout evaluator/reminder. An authorized correction approval can close a forgotten-checkout session while preserving original scans; there is no direct force-close endpoint. Automatic absence and evaluation cutoffs are absent. “No record yet” is explicitly not absence.
3. **Scheduling:** effective-dated shift snapshots, bulk assignments, rotating daily cycles, temporary overrides and immutable history are implemented; missing-session corrections use their date's assignment. Branch/department remain current assignments. Holiday calendars, split/flexible schedules, conflicts and scheduled-staff denominators remain absent. DST elapsed durations are tested, but ambiguous/nonexistent wall times lack a reviewed business policy.
4. **Privacy/legal:** location erasure needs an external schedule for the implemented CLI. No account/company data deletion/export administration, raw-record retention, backup erasure, policy-consent/notice acceptance or independent legal/security assessment. No automatic employment-law compliance is claimed.
5. **Production authorization hardening:** test coverage is meaningful but not exhaustive for every route/resource/department/multi-membership case. Development uses a DB owner; production must provision least privilege. Audit triggers can be overridden by database administrators. No hash-chain/external immutable export exists; full before/after snapshots and failed-login tenant audit events are incomplete.

## Product gaps

- Employee model currently includes name, email, code, role, position, status, start date and current department/branch/shift. No separate first/last name, phone, photos, reporting-manager assignment, employment-status timeline, employee CSV import/export or invitation flow. Changing shared account name/email through employee administration is not supported.
- No evidence uploads, private local/S3 object storage adapter, logo upload, file MIME/size validation or attachments. Upload controls are not exposed.
- Full-day leave only. No partial days, leave-type/entitlement administration, balances, adjustments or department calendar. No country-specific entitlements or payroll amount calculations.
- Only attendance-summary CSV/XLSX reports. No separate leave, unresolved discrepancy, correction or verification-failure exports, department comparison chart or PDF export. 366-day/50,000-row reporting caps.
- Dashboard uses actual records but current-calendar-date team cards can omit an open overnight session associated with the previous work date. Late and live-state metrics intentionally overlap. No-record includes unscheduled people and must never be read as absence. Worked minutes are calculated at checkout, not a live accruing estimate. Trend dates without records are omitted rather than manufactured.
- Employee directory search is page-local; attendance search is server-side. List `has_more` is inferred from a full page and can return a final empty page. Reference selectors and QR management UI show at most 100 resources and need searchable pagination for larger companies.
- Notifications cover attendance and request submission/decision, not schedule-change alerts, late notices, automatic missing-checkout reminders, SMTP delivery or preference controls.
- PWA assets and offline informational screen exist. No offline app shell/data caching, stored exception drafts, background submission or trusted offline attendance. Physical mobile installability has not been tested.
- English copy is currently embedded; no translated locale dictionaries or language switching yet. Shadcn-style Button/Dialog primitives use Radix/CVA, not a fully generated shadcn component suite. Advanced table widgets/maps are not implemented.
- Routes and charts load lazily, and the final build has no chunk-size warning. The scanner and main application bundles still merit further optimization; typed frontend response models would also improve maintainability.
- The Go HTTP layer is separated by module files but is not the complete suggested clean-architecture directory layout. List projections use JSON from explicit pgx SQL rather than sqlc-generated types. Permissions are code-defined, not persisted roles/permissions tables.

## Security/operating limits

Browser GPS and permanent QR photographs can be fabricated. There is no device-bound proof of location, QR photograph detector, continuous monitoring or misconduct inference. Rate limiting is in-process/IP-based and behind Nginx uses the proxy address unless reviewed proxy trust is configured; it is not distributed/durable. Password recovery is implemented with SMTP configuration, single-use reset links, generic responses and verified destinations.

Public registration is disabled; platform administrators provision companies/owners. Owner membership changes are blocked instead of ownership transfer. No owner/company deletion flow exists. Production Compose/Caddy configuration is provided; live-domain HTTPS, backup/restore, zero-downtime migrations and least-privilege credentials need production validation.

## Next implementation order

1. Mandatory first administrative-password change and MFA; independently audit the new passkey/recovery/access controls and proxy throttling.
2. Authorized missing-checkout resolution, request evidence review, calendar/absence cutoffs and historical effective assignments/policies.
3. Holiday/split/flexible scheduling and explicit DST wall-time policies.
4. Private upload storage and file validation; leave balances/partial days; full retention/data-rights administration.
5. Expand report coverage, filters, searchable references and notifications; extract i18n copy and split chart bundles.
6. Real iOS/Android camera/location/PWA QA, accessibility audit, independent security assessment and jurisdiction-specific legal review before production adoption.


