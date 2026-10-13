import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, Pencil, QrCode, Download, Printer, RotateCw, Search, MapPin } from 'lucide-react';
import QRCode from 'qrcode';
import { api, write, label } from '../lib/api';
import { Button, Modal, ErrorMessage, Empty, Badge, Loading } from '../components/ui';
import { PageTitle, useSession } from '../App';

type Field = {
  key: string;
  name: string;
  type?: string;
  options?: any[];
  min?: number;
  max?: number;
  optional?: boolean;
  hint?: string;
  readOnly?: boolean;
};
export function RecordForm({
  fields,
  initial,
  onSave,
  submit = 'Save changes',
}: {
  fields: Field[];
  initial?: any;
  onSave: (data: any) => Promise<any>;
  submit?: string;
}) {
  const [error, setError] = useState<unknown>(null),
    [busy, setBusy] = useState(false),
    [locating, setLocating] = useState(false),
    [locationNote, setLocationNote] = useState('');
  const hasCoordinates = fields.some((field) => field.key === 'latitude');
  return (
    <form
      className="record-form"
      onSubmit={async (e) => {
        e.preventDefault();
        setError(null);
        const raw = new FormData(e.currentTarget);
        const data: any = {};
        for (const f of fields) {
          const v = raw.get(f.key);
          data[f.key] =
            f.type === 'checkbox'
              ? v === 'on'
              : f.type === 'number'
                ? Number(v)
                : f.type === 'workdays'
                  ? String(v)
                      .split(',')
                      .map((v) => Number(v.trim()))
                  : v === '' && f.optional
                    ? null
                    : v;
        }
        setBusy(true);
        try {
          await onSave(data);
        } catch (err) {
          setError(err);
        } finally {
          setBusy(false);
        }
      }}
    >
      <div className="form-grid">
        {fields.map((f) => (
          <label key={f.key} className={f.type === 'textarea' ? 'full' : ''}>
            {f.type === 'checkbox' ? (
              <span className="checkbox-label">
                <input type="checkbox" name={f.key} defaultChecked={initial?.[f.key] ?? false} />
                {f.name}
              </span>
            ) : (
              <>
                {f.name}
                {f.options ? (
                  <>
                    {f.readOnly && (
                      <input type="hidden" name={f.key} value={initial?.[f.key] ?? ''} />
                    )}
                    <select
                      aria-label={f.name}
                      name={f.key}
                      defaultValue={initial?.[f.key] ?? ''}
                      required={!f.optional}
                      disabled={f.readOnly}
                    >
                      <option value="">{f.optional ? 'None' : 'Select…'}</option>
                      {f.options.map((o) => (
                        <option key={o.id} value={o.id}>
                          {o.name}
                        </option>
                      ))}
                    </select>
                  </>
                ) : f.type === 'textarea' ? (
                  <textarea
                    aria-label={f.name}
                    name={f.key}
                    defaultValue={initial?.[f.key] ?? ''}
                    required={!f.optional}
                    minLength={f.min}
                    maxLength={f.max || 2000}
                    rows={3}
                  />
                ) : (
                  <input
                    aria-label={f.name}
                    readOnly={f.readOnly || (f.key === 'email' && !!initial?.id)}
                    name={f.key}
                    type={f.type === 'workdays' ? 'text' : f.type || 'text'}
                    defaultValue={
                      Array.isArray(initial?.[f.key])
                        ? initial[f.key].join(',')
                        : (initial?.[f.key] ?? '')
                    }
                    required={!f.optional}
                    min={f.min}
                    max={f.max}
                    step={f.type === 'number' ? 'any' : undefined}
                    minLength={f.type === 'password' ? 12 : undefined}
                    maxLength={f.type === 'password' ? 128 : undefined}
                  />
                )}
              </>
            )}
            {f.hint && <small>{f.hint}</small>}
          </label>
        ))}
      </div>
      {hasCoordinates && (
        <div className="location-setup">
          <Button
            type="button"
            variant="outline"
            disabled={busy || locating}
            onClick={(event) => {
              setError(null);
              setLocationNote('');
              if (!navigator.geolocation) {
                setError(new Error('Location is not supported by this browser.'));
                return;
              }
              setLocating(true);
              const form = event.currentTarget.form!;
              navigator.geolocation.getCurrentPosition(
                (position) => {
                  const latitude = form.elements.namedItem('latitude') as HTMLInputElement;
                  const longitude = form.elements.namedItem('longitude') as HTMLInputElement;
                  latitude.value = position.coords.latitude.toFixed(7);
                  longitude.value = position.coords.longitude.toFixed(7);
                  setLocationNote(
                    `Coordinates filled from this device (reported accuracy ±${Math.round(position.coords.accuracy)} m). Save changes to apply them.`
                  );
                  setLocating(false);
                },
                (locationError) => {
                  setError(
                    new Error(
                      locationError.code === 1
                        ? 'Location permission was denied. Allow location access and try again.'
                        : 'A reliable position could not be obtained. Move near a window or outdoors and retry.'
                    )
                  );
                  setLocating(false);
                },
                { enableHighAccuracy: true, maximumAge: 0, timeout: 20000 }
              );
            }}
          >
            <MapPin size={16} />
            {locating ? 'Getting location…' : 'Use this device’s location'}
          </Button>
          {locationNote && <small>{locationNote}</small>}
        </div>
      )}
      <ErrorMessage error={error} />
      <div className="form-actions">
        <small>Changes are recorded in your audit trail.</small>
        <Button disabled={busy}>{busy ? 'Saving…' : submit}</Button>
      </div>
    </form>
  );
}
const reason: Field = {
  key: 'reason',
  name: 'Reason for this change',
  type: 'textarea',
  min: 3,
  max: 1000,
};
function fieldsFor(resource: string, refs: any, edit: boolean, configure: boolean): Field[] {
  const name = { key: 'name', name: 'Name', readOnly: resource === 'employees' && edit };
  if (resource === 'employees')
    return [
      name,
      { key: 'email', name: 'Work email', type: 'email' },
      ...(!edit
        ? [
            {
              key: 'password',
              name: 'Initial password',
              type: 'password',
              optional: true,
              hint: 'Required only for Admin or HR. Employees enroll through an invitation in Employee devices.',
            },
          ]
        : []),
      { key: 'employee_code', name: 'Employee code' },
      { key: 'position', name: 'Position', optional: true },
      {
        key: 'role',
        name: 'Role',
        options: (configure ? ['admin', 'hr', 'manager', 'employee'] : ['employee']).map((v) => ({
          id: v,
          name: label(v),
        })),
      },
      { key: 'department_id', name: 'Department', options: refs.departments, optional: true },
      { key: 'branch_id', name: 'Branch', options: refs.branches },
      {
        key: 'shift_id',
        name: 'Initial shift template',
        options: refs.shifts,
        readOnly: edit,
        hint: 'Use Scheduling to apply effective-dated changes to existing employees.',
      },
      { key: 'start_date', name: 'Employment start', type: 'date' },
      { key: 'active', name: 'Active membership', type: 'checkbox' },
      reason,
    ];
  if (resource === 'workplaces')
    return [
      name,
      { key: 'branch_id', name: 'Branch', options: refs.branches },
      { key: 'address', name: 'Address', optional: true },
      { key: 'latitude', name: 'Latitude', type: 'number', min: -90, max: 90 },
      { key: 'longitude', name: 'Longitude', type: 'number', min: -180, max: 180 },
      { key: 'radius_m', name: 'Allowed radius (m)', type: 'number', min: 1, max: 10000 },
      {
        key: 'max_accuracy_m',
        name: 'Maximum reported error (m)',
        type: 'number',
        min: 1,
        max: 1000,
      },
      { key: 'location_required', name: 'Require location verification', type: 'checkbox' },
      { key: 'active', name: 'Workplace active', type: 'checkbox' },
      reason,
    ];
  if (resource === 'shifts')
    return [
      name,
      { key: 'start_time', name: 'Start time', type: 'time' },
      { key: 'end_time', name: 'End time', type: 'time' },
      { key: 'grace_minutes', name: 'Grace period (min)', type: 'number', min: 0, max: 120 },
      { key: 'expected_minutes', name: 'Expected work (min)', type: 'number', min: 1, max: 1440 },
      {
        key: 'unpaid_break_minutes',
        name: 'Additional fixed unpaid deduction (min)',
        type: 'number',
        min: 0,
        max: 240,
        hint: 'Recorded break time is also deducted. Set 0 to avoid double deductions.',
      },
      {
        key: 'early_window_minutes',
        name: 'Early check-in window (min)',
        type: 'number',
        min: 0,
        max: 720,
      },
      {
        key: 'workdays',
        name: 'Working weekdays',
        type: 'workdays',
        hint: 'Comma-separated: 0 Sunday, 1 Monday … 6 Saturday.',
      },
      {
        key: 'overtime_enabled',
        name: 'Calculate overtime beyond expected minutes',
        type: 'checkbox',
      },
      reason,
    ];
  if (resource === 'points')
    return [
      name,
      { key: 'workplace_id', name: 'Workplace', options: refs.workplaces },
      { key: 'active', name: 'QR point active', type: 'checkbox' },
      reason,
    ];
  return [name, reason];
}
export function useReferences(enabled: boolean) {
  return useQuery({ queryKey: ['references'], enabled, queryFn: () => api('/references') });
}
export function AdminPage({ resource }: { resource: string }) {
  const { user } = useSession(),
    configure = ['owner', 'admin'].includes(user.role),
    canEdit = resource !== 'employees' || ['owner', 'admin', 'hr'].includes(user.role);
  const [page, setPage] = useState(1),
    [search, setSearch] = useState(''),
    [edit, setEdit] = useState<any>(null);
  const data = useQuery({
    queryKey: ['admin', resource, page],
    queryFn: () => api(`/admin/${resource}?page=${page}`),
  });
  const refs = useReferences(canEdit);
  const client = useQueryClient();
  // HR receives organization reference data through a dedicated scoped endpoint.
  const rows =
    data.data?.items?.filter((r: any) =>
      [r.name, r.email, r.employee_code].some((v) =>
        String(v || '')
          .toLowerCase()
          .includes(search.toLowerCase())
      )
    ) || [];
  const defaults: any = {
    active: true,
    location_required: true,
    radius_m: 150,
    max_accuracy_m: 75,
    start_time: '08:00',
    end_time: '17:00',
    grace_minutes: 10,
    expected_minutes: 480,
    unpaid_break_minutes: 0,
    early_window_minutes: 120,
    workdays: [1, 2, 3, 4, 5],
    role: 'employee',
    start_date: new Date().toISOString().slice(0, 10),
  };
  return (
    <>
      <PageTitle
        title={label(resource)}
        description={
          {
            employees: 'The people behind your workplace. Manage assignments and access.',
            shifts: 'Clear schedules and consistent attendance rules.',
            workplaces: 'Define where attendance can be recorded.',
            departments: 'Keep your organization connected.',
            branches: 'Manage your company locations.',
          }[resource] || 'Manage company resources.'
        }
        action={
          canEdit && (
            <Button onClick={() => setEdit({})}>
              <Plus size={18} />
              Add {resource === 'employees' ? 'employee' : resource.slice(0, -1)}
            </Button>
          )
        }
      />
      <section className="card">
        <div className="table-toolbar">
          <div className="search-input">
            <Search size={17} />
            <input
              aria-label="Search current page"
              placeholder={`Search ${resource} on this page…`}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <span className="muted">Page {page}</span>
        </div>
        <ErrorMessage error={data.error} />
        {data.isPending ? (
          <Loading />
        ) : !rows.length ? (
          <Empty title={`No ${resource} to show`} />
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  {resource === 'employees' ? (
                    <>
                      <th>Department</th>
                      <th>Shift</th>
                      <th>Role</th>
                      <th>Status</th>
                    </>
                  ) : resource === 'workplaces' ? (
                    <>
                      <th>Coordinates</th>
                      <th>Boundary</th>
                      <th>Verification</th>
                    </>
                  ) : resource === 'shifts' ? (
                    <>
                      <th>Schedule</th>
                      <th>Grace</th>
                      <th>Expected</th>
                    </>
                  ) : (
                    <th>Company resource</th>
                  )}
                  {canEdit && <th>Actions</th>}
                </tr>
              </thead>
              <tbody>
                {rows.map((r: any) => (
                  <tr key={r.id}>
                    <td>
                      <strong>{r.name}</strong>
                      <small>{r.email || r.address || ''}</small>
                    </td>
                    {resource === 'employees' ? (
                      <>
                        <td>{r.department || '—'}</td>
                        <td>{r.shift || '—'}</td>
                        <td>
                          <Badge value={r.role} />
                        </td>
                        <td>
                          <Badge value={r.active ? 'active' : 'inactive'} />
                        </td>
                      </>
                    ) : resource === 'workplaces' ? (
                      <>
                        <td>
                          {r.latitude.toFixed(5)}, {r.longitude.toFixed(5)}
                        </td>
                        <td>{r.radius_m} m</td>
                        <td>
                          <Badge value={r.location_required ? 'required' : 'optional'} />
                        </td>
                      </>
                    ) : resource === 'shifts' ? (
                      <>
                        <td>
                          {r.start_time} — {r.end_time}
                        </td>
                        <td>{r.grace_minutes} min</td>
                        <td>{r.expected_minutes} min</td>
                      </>
                    ) : (
                      <td>Available for assignment</td>
                    )}
                    {canEdit && (
                      <td>
                        {r.role !== 'owner' && r.id !== user.membership_id && (
                          <Button
                            variant="ghost"
                            aria-label={`Edit ${r.name}`}
                            onClick={() => setEdit(r)}
                          >
                            <Pencil size={16} />
                          </Button>
                        )}
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="pagination">
          <Button variant="outline" disabled={page === 1} onClick={() => setPage(page - 1)}>
            Previous
          </Button>
          <span>Page {page}</span>
          <Button
            variant="outline"
            disabled={!data.data?.has_more}
            onClick={() => setPage(page + 1)}
          >
            Next
          </Button>
        </div>
      </section>
      <Modal
        open={!!edit}
        onOpenChange={(v) => !v && setEdit(null)}
        title={`${edit?.id ? 'Edit' : 'Add'} ${resource === 'employees' ? 'employee' : resource.slice(0, -1)}`}
      >
        {refs.isPending ? (
          <Loading />
        ) : refs.error ? (
          <ErrorMessage error={refs.error} />
        ) : (
          edit && (
            <RecordForm
              key={edit.id || 'new'}
              fields={fieldsFor(resource, refs.data || {}, !!edit.id, configure)}
              initial={{ ...defaults, ...edit }}
              onSave={async (value) => {
                await write(
                  `/admin/${resource}${edit.id ? `/${edit.id}` : ''}`,
                  value,
                  edit.id ? 'PATCH' : 'POST'
                );
                setEdit(null);
                await client.invalidateQueries();
              }}
            />
          )
        )}
      </Modal>
    </>
  );
}
export function QRPage() {
  const config = useQuery({ queryKey: ['public-config'], queryFn: () => api('/public/config') });
  const [edit, setEdit] = useState<any>(null),
    [preview, setPreview] = useState<any>(null),
    [error, setError] = useState<unknown>(null),
    [png, setPng] = useState('');
  const data = useQuery({ queryKey: ['points'], queryFn: () => api('/admin/points?limit=100') }),
    refs = useReferences(true),
    client = useQueryClient();
  const [rotation, setRotation] = useState<any>(null);
  async function show(point: any) {
    try {
      setPng(
        await QRCode.toDataURL(
          `${config.data?.origin || location.origin}/scan/${point.public_id}`,
          {
            width: 800,
            margin: 4,
            errorCorrectionLevel: 'M',
          }
        )
      );
      setPreview(point);
    } catch (e) {
      setError(e);
    }
  }
  async function svg() {
    if (!preview) return;
    const text = await QRCode.toString(
      `${config.data?.origin || location.origin}/scan/${preview.public_id}`,
      {
        type: 'svg',
        margin: 4,
      }
    );
    const url = URL.createObjectURL(new Blob([text], { type: 'image/svg+xml' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = 'devaraj-attendance.svg';
    a.click();
    URL.revokeObjectURL(url);
  }
  return (
    <>
      <PageTitle
        title="QR code management"
        description="A permanent point of connection. Print once and keep attendance simple."
        action={
          <Button onClick={() => setEdit({})}>
            <Plus size={18} />
            Create QR point
          </Button>
        }
      />
      <div className="info-strip">
        <ShieldIcon />
        Public QR codes contain only an opaque attendance-point identifier. Authentication and
        workplace verification are still required.
      </div>
      <ErrorMessage error={error} />
      <ErrorMessage error={data.error} />
      {data.isPending ? (
        <Loading />
      ) : data.data?.items.length ? (
        <div className="qr-grid">
          {data.data.items.map((p: any) => (
            <section className="card qr-card" key={p.id}>
              <div className="qr-symbol">
                <QrCode size={60} />
              </div>
              <Badge value={p.active ? 'active' : 'disabled'} />
              <h3>{p.name}</h3>
              <p>
                {p.workplace} · {p.branch}
              </p>
              <small>Created {new Date(p.created_at).toLocaleDateString()}</small>
              <div className="qr-actions">
                <Button onClick={() => show(p)} variant="outline">
                  <QrCode size={16} />
                  View & print
                </Button>
                <Button variant="ghost" onClick={() => setEdit(p)} aria-label={`Edit ${p.name}`}>
                  <Pencil size={16} />
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => setRotation(p)}
                  aria-label={`Rotate ${p.name}`}
                >
                  <RotateCw size={16} />
                </Button>
              </div>
            </section>
          ))}
        </div>
      ) : (
        <Empty title="Create your first attendance point">
          Choose a workplace, generate its QR, and print it for your team.
        </Empty>
      )}
      <Modal
        open={!!edit}
        onOpenChange={(v) => !v && setEdit(null)}
        title={edit?.id ? 'Edit attendance point' : 'Create attendance point'}
      >
        {refs.isPending ? (
          <Loading />
        ) : refs.error ? (
          <ErrorMessage error={refs.error} />
        ) : (
          edit && (
            <RecordForm
              fields={fieldsFor('points', refs.data, !!edit.id, true)}
              initial={{ active: true, ...edit }}
              onSave={async (value) => {
                await write(
                  `/admin/points${edit.id ? `/${edit.id}` : ''}`,
                  value,
                  edit.id ? 'PATCH' : 'POST'
                );
                setEdit(null);
                await client.invalidateQueries();
              }}
            />
          )
        )}
      </Modal>
      <Modal
        open={!!preview}
        onOpenChange={(v) => !v && setPreview(null)}
        title="Your printable attendance QR"
      >
        <div className="print-sheet">
          <h2>Devaraj Time</h2>
          <p>
            {preview?.name} · {preview?.workplace}
          </p>
          <img src={png} alt="Attendance QR code" />
          <h3>Scan. Verify. Get on with your day.</h3>
          <p>Open your phone’s Camera app and scan. Follow the link to record attendance.</p>
          <small>
            {config.data?.origin || location.origin}/scan/{preview?.public_id}
          </small>
        </div>
        <div className="qr-downloads">
          <Button asChild variant="outline">
            <a href={png} download="devaraj-attendance.png">
              <Download size={16} />
              PNG
            </a>
          </Button>
          <Button variant="outline" onClick={svg}>
            SVG
          </Button>
          <Button onClick={() => window.print()}>
            <Printer size={16} />
            Print sheet
          </Button>
        </div>
      </Modal>
      <Modal
        open={!!rotation}
        onOpenChange={(v) => !v && setRotation(null)}
        title="Replace this QR identifier"
        description="The old printed code will stop working immediately. Print and display the replacement."
      >
        {rotation && (
          <RecordForm
            fields={[reason]}
            submit="Rotate QR identifier"
            onSave={async (value) => {
              await write(`/admin/points/${rotation.id}/rotate`, value);
              setRotation(null);
              await client.invalidateQueries();
            }}
          />
        )}
      </Modal>
    </>
  );
}
function ShieldIcon() {
  return <QrCode size={19} />;
}
export function SettingsPage() {
  const { user } = useSession();
  const org = useQuery({ queryKey: ['organization'], queryFn: () => api('/organization') });
  const client = useQueryClient();
  const [saved, setSaved] = useState(false);
  return (
    <>
      <PageTitle
        title="Company settings"
        description="Clear policies help build a fairer workplace."
      />
      {org.isPending ? (
        <Loading />
      ) : org.error ? (
        <ErrorMessage error={org.error} />
      ) : (
        <section className="card settings-card">
          <h3>Organization & attendance policy</h3>
          <p>Existing attendance keeps its applied policy snapshot.</p>
          {saved && (
            <div className="success" role="status">
              Company settings saved.
            </div>
          )}
          <RecordForm
            fields={[
              { key: 'name', name: 'Company name' },
              { key: 'timezone', name: 'IANA timezone' },
              { key: 'breaks_enabled', name: 'Enable break recording', type: 'checkbox' },
              {
                key: 'correction_days',
                name: 'Correction deadline (days)',
                type: 'number',
                min: 1,
                max: 90,
              },
              {
                key: 'location_retention_days',
                name: 'Location evidence retention (days)',
                type: 'number',
                min: 1,
                max: 365,
                hint: 'Applies to the scheduled administrator location-erasure job.',
              },
              reason,
            ]}
            initial={{ ...org.data, name: org.data.company }}
            onSave={async (value) => {
              await write('/settings', value, 'PATCH');
              setSaved(true);
              await client.invalidateQueries();
            }}
          />
          <div className="info-strip">
            Secure sessions expire after 12 hours. Account recovery, invitations and device-bound
            credentials are not enabled in this release.
          </div>
        </section>
      )}
    </>
  );
}
