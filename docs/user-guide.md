# User guide

## Employee

Enroll once using HR's private invitation link and create an attendance passkey on your phone. Each day, scan the printed link QR with the phone's normal Camera app. Open its attendance link, verify your passkey if the secure session has expired, and permit precise location. The server automatically selects check-in/out and shows success only after committing. Rapid repeats return the original event. End a break in self-service before automatic checkout. Review [the enrollment guide](system-update.md) for hostname and device-replacement requirements.

If camera is unavailable, open the printed attendance URL or enter its identifier; geofence checks still apply. If location is rejected/denied, follow the correction link. No scan is fabricated and no automatic absence accusation is created.

Attendance History filters dates and lateness outcomes. Open a record to see effective times, applied historical policy, original scans and correction decisions. Corrections require proposed past times and a clear reason. When submitting datetime fields, use the device timezone; the form converts to UTC and the server validates shift work date. Request full-day leave in Leave Requests. Pending personal requests can be cancelled.

Profile shows identity, branch, department and privacy information. Update your own display name or change your password; password changes revoke every session and require signing in again. Notifications show attendance confirmations and review decisions. Offline pages explain that attendance was not recorded; prepare notes externally and submit for review after reconnecting.

## HR and department manager

Overview shows current-calendar-date scoped membership/event counts and seven days of actual recorded summaries. No record yet is not an absence count and includes unscheduled employees. Pending requests combines correction and leave counts. Manager access is limited to the assigned department plus personal records; unassigned managers have no team-wide access.

Use Attendance to inspect employee records and applied policy. In Corrections or Leave Requests, review requests belonging to others and enter an approval/rejection reason. Approved corrections preserve original scans. Leave overlapping attendance or existing approved leave is rejected so the conflict can be resolved. Managers cannot review unrelated departments or their own requests.

HR can create/update employee-role records, deactivate memberships, issue/revoke passkey enrollment, manage shift templates and effective-dated individual/bulk schedules, and download scoped reports. Employee passwords cannot authenticate. Existing email/name changes are account-holder operations. Privileged role escalation is restricted to administrators.

## Company owner/administrator

Configure Settings, Branches, Departments and Shifts. Workday numbers are Sunday 0 to Saturday 6. Ending at/before shift start denotes overnight. Recorded and fixed unpaid-break deductions both apply; use a fixed deduction of zero when tracking actual breaks.

Create a workplace with real coordinates, radius, branch, maximum acceptable location error and location requirement. Create a QR point, preview/download/print it and display it at the workplace. Disabling a point prevents use. Rotation immediately invalidates the old printed identifier; replace its printed sheet.

Create employees with unique work emails and company employee codes, appropriate roles, department/branch/shift and employment start. Deactivation preserves historical records. Owner/current-user membership edits are intentionally blocked through employee administration. Profile identity updates apply to the account holder across memberships.

Audit Trail records actor, reason, time and target reference; schedule assignments preserve immutable before/after snapshots and effective dates. Administrative password recovery is on the login page and requires configured SMTP; personal recovery emails must be verified. Schedule the location retention CLI in deployment operations. Uploads, leave balances and automatic absence evaluation remain unavailable.
