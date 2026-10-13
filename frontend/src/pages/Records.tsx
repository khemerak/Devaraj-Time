import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useParams } from 'react-router-dom';
import {
  Plus,
  Search,
  Download,
  CheckCircle2,
  XCircle,
  ArrowLeft,
  CalendarDays,
  ShieldCheck,
} from 'lucide-react';
import { api, write, label, time, hours } from '../lib/api';
import { Button, Modal, Badge, Loading, Empty, ErrorMessage } from '../components/ui';
import { RecordForm } from './Admin';
import { PageTitle, useSession } from '../App';

function monthStart() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-01`;
}
function today() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
export function Pagination({
  page,
  setPage,
  more,
}: {
  page: number;
  setPage: (p: number) => void;
  more: boolean;
}) {
  return (
    <div className="pagination">
      <Button variant="outline" disabled={page === 1} onClick={() => setPage(page - 1)}>
        Previous
      </Button>
      <span>Page {page}</span>
      <Button variant="outline" disabled={!more} onClick={() => setPage(page + 1)}>
        Next
      </Button>
    </div>
  );
}
export function HistoryPage() {
  const { id } = useParams(),
    { user } = useSession();
  const [page, setPage] = useState(1),
    [from, setFrom] = useState(monthStart()),
    [to, setTo] = useState(today()),
    [status, setStatus] = useState(''),
    [search, setSearch] = useState(''),
    [mine, setMine] = useState(false);
  const data = useQuery({
    queryKey: ['attendance', page, from, to, status, search, mine],
    queryFn: () =>
      api(
        `/attendance?${new URLSearchParams({ page: String(page), from, to, status, search, mine: String(mine) })}`
      ),
    enabled: !id,
  });
  const detail = useQuery({
    queryKey: ['attendance', id],
    queryFn: () => api(`/attendance/${id}`),
    enabled: !!id,
  });
  if (id) {
    if (detail.isPending) return <Loading />;
    if (detail.error) return <ErrorMessage error={detail.error} />;
    const r = detail.data.record;
    return (
      <>
        <Button variant="ghost" asChild>
          <Link to="/attendance">
            <ArrowLeft size={16} />
            Back to attendance
          </Link>
        </Button>
        <PageTitle
          title="Attendance details"
          description={`${r.name} · ${r.work_date} · ${user.timezone}`}
          action={
            <Button variant="outline" asChild>
              <Link to="/corrections">Request a correction</Link>
            </Button>
          }
        />
        <div className="dashboard-grid">
          <section className="card">
            <div className="card-heading">
              <h3>Effective attendance</h3>
              <Badge value={r.status} />
            </div>
            <div className="detail-grid">
              <div>
                <small>Check-in</small>
                <strong>{time(r.check_in, user.timezone)}</strong>
              </div>
              <div>
                <small>Check-out</small>
                <strong>{time(r.check_out, user.timezone)}</strong>
              </div>
              <div>
                <small>Worked</small>
                <strong>{hours(r.worked_minutes)}</strong>
              </div>
              <div>
                <small>Late beyond grace</small>
                <strong>{r.late_minutes} min</strong>
              </div>
              <div>
                <small>Actual arrival delay</small>
                <strong>{r.arrival_delay_minutes} min</strong>
              </div>
              <div>
                <small>Recorded breaks</small>
                <strong>{Math.floor(r.break_seconds / 60)} min</strong>
              </div>
              <div>
                <small>Early departure</small>
                <strong>{r.early_minutes} min</strong>
              </div>
              <div>
                <small>Overtime</small>
                <strong>{r.overtime_minutes} min</strong>
              </div>
            </div>
            {r.adjusted && (
              <div className="info-strip">
                This summary includes an approved correction. Original events remain below.
              </div>
            )}
          </section>
          <section className="card">
            <h3>Applied policy snapshot</h3>
            <div className="detail-grid">
              <div>
                <small>Policy version</small>
                <strong>{r.policy.version}</strong>
              </div>
              <div>
                <small>Shift</small>
                <strong>{r.policy.shift_name}</strong>
              </div>
              <div>
                <small>Scheduled hours</small>
                <strong>
                  {r.policy.start} — {r.policy.end}
                </strong>
              </div>
              <div>
                <small>Grace period</small>
                <strong>{r.policy.grace} min</strong>
              </div>
              <div>
                <small>Timezone</small>
                <strong>{r.policy.timezone}</strong>
              </div>
              <div>
                <small>Expected work</small>
                <strong>{r.policy.expected} min</strong>
              </div>
            </div>
            <p className="muted">
              Lateness is the arrival delay minus the grace period, floored at zero. Recorded and
              configured unpaid breaks are deducted separately.
            </p>
          </section>
        </div>
        <section className="card">
          <div className="card-heading">
            <h3>Original recorded events</h3>
            <ShieldCheck size={20} />
          </div>
          {detail.data.events.length ? (
            <div className="event-timeline">
              {detail.data.events.map((e: any) => (
                <div key={e.id}>
                  <span className="timeline-dot" />
                  <div>
                    <strong>{label(e.action)}</strong>
                    <p>
                      {new Intl.DateTimeFormat('en', {
                        dateStyle: 'medium',
                        timeStyle: 'medium',
                        timeZone: r.policy.timezone,
                      }).format(new Date(e.recorded_at))}
                    </p>
                    <Badge value={e.verification} />
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <Empty title="No original scans">
              This record was created through an approved correction.
            </Empty>
          )}
        </section>
        <section className="card">
          <h3>Correction history</h3>
          {detail.data.corrections.length ? (
            detail.data.corrections.map((c: any) => (
              <div className="request-history" key={c.id}>
                <Badge value={c.status} />
                <strong>{c.issue}</strong>
                <p>{c.reason}</p>
                <p>Reviewer decision: {c.review_reason || 'Pending review'}</p>
                <small>
                  {c.reviewed_at ? new Date(c.reviewed_at).toLocaleString() : 'No review yet'}
                </small>
              </div>
            ))
          ) : (
            <Empty title="No corrections requested" />
          )}
        </section>
      </>
    );
  }
  return (
    <>
      <PageTitle
        title="Attendance history"
        description="A transparent record of every workday. Original scans stay preserved."
      />
      <section className="card">
        <div className="filters">
          <div className="search-input">
            <Search size={17} />
            <input
              aria-label="Search employee"
              placeholder="Search employee or code…"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setPage(1);
              }}
            />
          </div>
          <label>
            From
            <input
              type="date"
              value={from}
              onChange={(e) => {
                setFrom(e.target.value);
                setPage(1);
              }}
            />
          </label>
          <label>
            To
            <input
              type="date"
              value={to}
              onChange={(e) => {
                setTo(e.target.value);
                setPage(1);
              }}
            />
          </label>
          <label>
            Status
            <select
              value={status}
              onChange={(e) => {
                setStatus(e.target.value);
                setPage(1);
              }}
            >
              <option value="">All statuses</option>
              {['on_time', 'within_grace', 'late'].map((v) => (
                <option key={v} value={v}>
                  {label(v)}
                </option>
              ))}
            </select>
          </label>
          {user.role !== 'employee' && (
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={mine}
                onChange={(e) => {
                  setMine(e.target.checked);
                  setPage(1);
                }}
              />
              Only my records
            </label>
          )}
        </div>
        <ErrorMessage error={data.error} />
        {data.isPending ? (
          <Loading />
        ) : !data.data?.items.length ? (
          <Empty title="No attendance in this date range" />
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Employee</th>
                  <th>Work date</th>
                  <th>Check-in</th>
                  <th>Check-out</th>
                  <th>Worked</th>
                  <th>Late</th>
                  <th>Status</th>
                  <th>State</th>
                </tr>
              </thead>
              <tbody>
                {data.data.items.map((r: any) => (
                  <tr key={r.id}>
                    <td>
                      <Link to={`/attendance/${r.id}`}>
                        <strong>{r.name}</strong>
                        <small>{r.employee_code}</small>
                      </Link>
                    </td>
                    <td>{r.work_date}</td>
                    <td>{time(r.check_in, user.timezone)}</td>
                    <td>{time(r.check_out, user.timezone)}</td>
                    <td>{hours(r.worked_minutes)}</td>
                    <td>{r.late_minutes} min</td>
                    <td>
                      <Badge value={r.status} />
                      {r.adjusted && <small>Adjusted</small>}
                    </td>
                    <td>{label(r.state)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <Pagination page={page} setPage={setPage} more={data.data?.has_more} />
      </section>
    </>
  );
}
export function RequestsPage({ kind }: { kind: 'correction' | 'leave' }) {
  const { user } = useSession(),
    client = useQueryClient();
  const [page, setPage] = useState(1),
    [open, setOpen] = useState(false),
    [review, setReview] = useState<any>(null),
    [error, setError] = useState<unknown>(null);
  const data = useQuery({
    queryKey: ['requests', kind, page],
    queryFn: () => api(`/requests?kind=${kind}&page=${page}`),
  });
  const baseFields: any[] = [
    {
      key: 'issue',
      name: kind === 'leave' ? 'Leave type' : 'Issue type',
      options: (kind === 'leave'
        ? ['Annual leave', 'Sick leave', 'Personal leave', 'Unpaid leave', 'Other']
        : [
            'Forgot check-in',
            'Forgot check-out',
            'Network unavailable',
            'QR not working',
            'Location verification failed',
            'Incorrect attendance calculation',
            'Other',
          ]
      ).map((v) => ({ id: v, name: v })),
    },
    { key: 'start_date', name: kind === 'leave' ? 'First day' : 'Work date', type: 'date' },
    ...(kind === 'leave'
      ? [{ key: 'end_date', name: 'Last day', type: 'date' }]
      : [
          {
            key: 'requested_in',
            name: 'Requested check-in',
            type: 'datetime-local',
            hint: 'Enter in your device timezone. This is a proposed time, not a server-recorded event.',
          },
          { key: 'requested_out', name: 'Requested check-out', type: 'datetime-local' },
        ]),
    { key: 'reason', name: 'Explain what happened', type: 'textarea', min: 10 },
  ];
  return (
    <>
      <PageTitle
        title={kind === 'leave' ? 'Leave requests' : 'Attendance corrections'}
        description={
          kind === 'leave'
            ? 'Time away, with a clear path to approval.'
            : 'Something doesn’t look right? Start a transparent review.'
        }
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus size={18} />
            New request
          </Button>
        }
      />
      <div className="info-strip">
        <ShieldCheck size={18} />
        {kind === 'correction'
          ? 'Proposed times are reviewed by HR. Approved corrections never erase original attendance events.'
          : 'Approved full-day leave prevents conflicting attendance. Balances and partial-day leave are not enabled.'}
      </div>
      <section className="card">
        <ErrorMessage error={error || data.error} />
        {data.isPending ? (
          <Loading />
        ) : !data.data?.items.length ? (
          <Empty title="No requests yet" />
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Employee</th>
                  <th>Request</th>
                  <th>Dates</th>
                  <th>Reason</th>
                  <th>Status</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {data.data.items.map((r: any) => (
                  <tr key={r.id}>
                    <td>
                      <strong>{r.name}</strong>
                    </td>
                    <td>{r.issue}</td>
                    <td>
                      {r.start_date}
                      {r.end_date !== r.start_date && <small>to {r.end_date}</small>}
                    </td>
                    <td className="reason-cell">
                      {r.reason}
                      {r.review_reason && (
                        <small>
                          Review: {r.review_reason} · {r.reviewer}
                        </small>
                      )}
                    </td>
                    <td>
                      <Badge value={r.status} />
                    </td>
                    <td>
                      {r.status === 'pending' &&
                        (r.membership_id === user.membership_id ? (
                          <Button
                            variant="ghost"
                            onClick={async () => {
                              if (!window.confirm('Cancel this request?')) return;
                              try {
                                await write(`/requests/${r.id}/cancel`, {});
                                await client.invalidateQueries();
                              } catch (e) {
                                setError(e);
                              }
                            }}
                          >
                            Cancel
                          </Button>
                        ) : (
                          ['owner', 'admin', 'hr', 'manager'].includes(user.role) && (
                            <Button variant="outline" onClick={() => setReview(r)}>
                              Review
                            </Button>
                          )
                        ))}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <Pagination page={page} setPage={setPage} more={data.data?.has_more} />
      </section>
      <Modal
        open={open}
        onOpenChange={setOpen}
        title={kind === 'leave' ? 'Request time off' : 'Request an attendance correction'}
        description="Your request is reviewed by an authorized colleague. You cannot approve your own request."
      >
        <RecordForm
          key={kind}
          fields={baseFields}
          initial={{ start_date: today(), end_date: today() }}
          submit="Submit for review"
          onSave={async (value) => {
            const payload = {
              ...value,
              kind,
              end_date: kind === 'correction' ? value.start_date : value.end_date,
            };
            if (kind === 'correction') {
              payload.requested_in = new Date(value.requested_in).toISOString();
              payload.requested_out = new Date(value.requested_out).toISOString();
            }
            await write('/requests', payload);
            setOpen(false);
            await client.invalidateQueries();
          }}
        />
      </Modal>
      <Modal
        open={!!review}
        onOpenChange={(v) => !v && setReview(null)}
        title="Review request"
        description={`${review?.name} · ${review?.start_date}`}
      >
        <p>{review?.reason}</p>
        {kind === 'correction' && (
          <div className="info-strip">
            Requested: {review?.requested_in && new Date(review.requested_in).toLocaleString()} —{' '}
            {review?.requested_out && new Date(review.requested_out).toLocaleString()}
          </div>
        )}
        <RecordForm
          fields={[
            {
              key: 'decision',
              name: 'Decision',
              options: [
                { id: 'approved', name: 'Approve' },
                { id: 'rejected', name: 'Reject' },
              ],
            },
            { key: 'reason', name: 'Reason for your decision', type: 'textarea', min: 10 },
          ]}
          submit="Record decision"
          onSave={async (value) => {
            await write(`/requests/${review.id}/review`, value);
            setReview(null);
            await client.invalidateQueries();
          }}
        />
      </Modal>
    </>
  );
}
export function ReportsPage() {
  const [from, setFrom] = useState(monthStart()),
    [to, setTo] = useState(today());
  return (
    <>
      <PageTitle
        title="Attendance reports"
        description="Clear, exportable records for your team’s next conversation."
      />
      <section className="card settings-card">
        <div className="card-heading">
          <div>
            <h3>Attendance summary export</h3>
            <p>Effective times, calculations, policy timezone and adjustment flags.</p>
          </div>
          <Download />
        </div>
        <div className="form-grid">
          <label>
            Start date
            <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
          </label>
          <label>
            End date
            <input type="date" value={to} onChange={(e) => setTo(e.target.value)} />
          </label>
        </div>
        <p>
          Reports follow your company and department access permissions. Worked, late, early
          departure and overtime values are expressed in minutes. Adjusted records are identified
          explicitly.
        </p>
        <div className="qr-downloads">
          <Button asChild>
            <a href={`/api/reports?${new URLSearchParams({ from, to, format: 'csv' })}`}>
              <Download size={17} />
              Download CSV
            </a>
          </Button>
          <Button variant="outline" asChild>
            <a href={`/api/reports?${new URLSearchParams({ from, to, format: 'xlsx' })}`}>
              Download Excel
            </a>
          </Button>
        </div>
        <div className="info-strip">
          Exports contain attendance summaries. Leave, unresolved missing scans and verification
          failure reports are not included in this release. No payroll amounts are calculated.
        </div>
      </section>
    </>
  );
}
export function AuditPage() {
  const [page, setPage] = useState(1);
  const data = useQuery({ queryKey: ['audit', page], queryFn: () => api(`/audit?page=${page}`) });
  return (
    <>
      <PageTitle
        title="Audit trail"
        description="Accountable decisions. A preserved history of company actions."
      />
      <section className="card">
        <ErrorMessage error={data.error} />
        {data.isPending ? (
          <Loading />
        ) : !data.data?.items.length ? (
          <Empty title="No audit events yet" />
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Timestamp</th>
                  <th>Actor</th>
                  <th>Action</th>
                  <th>Reason</th>
                  <th>Reference</th>
                </tr>
              </thead>
              <tbody>
                {data.data.items.map((r: any) => (
                  <tr key={r.id}>
                    <td>{new Date(r.created_at).toLocaleString()}</td>
                    <td>{r.actor || 'System'}</td>
                    <td>
                      <Badge value={r.action} />
                    </td>
                    <td>{r.reason || '—'}</td>
                    <td>
                      <code>{r.target.slice(0, 12)}</code>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <Pagination page={page} setPage={setPage} more={data.data?.has_more} />
      </section>
    </>
  );
}
export function SchedulePage() {
  const org = useQuery({ queryKey: ['organization'], queryFn: () => api('/organization') });
  const assignments = useQuery({ queryKey: ['my-schedules'], queryFn: () => api('/schedules') });
  if (org.isPending) return <Loading />;
  if (org.error) return <ErrorMessage error={org.error} />;
  const o = org.data,
    s = o.shift;
  return (
    <>
      <PageTitle
        title="My schedule"
        description="Know what’s expected before your workday begins."
      />
      <section className="card settings-card">
        <div className="card-heading">
          <div>
            <h3>{s?.name || 'No shift assigned'}</h3>
            <p>
              {o.branch || 'No branch assigned'} · {o.timezone}
            </p>
          </div>
          <CalendarDays size={25} />
        </div>
        {s ? (
          <>
            <div className="schedule-time">
              {s.start_time}
              <span>to</span>
              {s.end_time}
            </div>
            <div className="weekday-row">
              {['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].map((d, n) => (
                <span key={d} className={s.workdays.includes(n) ? 'scheduled' : ''}>
                  {d}
                </span>
              ))}
            </div>
            <div className="detail-grid">
              <div>
                <small>Grace period</small>
                <strong>{s.grace_minutes} min</strong>
              </div>
              <div>
                <small>Expected work</small>
                <strong>{hours(s.expected_minutes)}</strong>
              </div>
              <div>
                <small>Early check-in window</small>
                <strong>{s.early_window_minutes} min</strong>
              </div>
              <div>
                <small>Correction deadline</small>
                <strong>{o.correction_days} days</strong>
              </div>
            </div>
            <p className="muted">
              A shift ending at or before its start time crosses midnight. The work date is anchored
              to its scheduled start. Effective-dated rotations and temporary overrides are listed
              below.
            </p>
          </>
        ) : (
          <Empty title="Rest day or no active shift" />
        )}
      </section>
      <section className="card">
        <h3>Schedule history and upcoming assignments</h3>
        <ErrorMessage error={assignments.error} />
        {assignments.data?.items.map((a: any) => (
          <p key={a.id}>
            <strong>
              {a.effective_from} – {a.effective_to || 'ongoing'}
            </strong>
            <br />
            {a.cycle
              .map((s: any) => (s ? `${s.name}: ${s.start_time}–${s.end_time}` : 'Rest'))
              .join(' → ')}
            <br />
            {a.reason}
          </p>
        ))}
      </section>
    </>
  );
}
