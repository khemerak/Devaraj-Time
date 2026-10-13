import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, write } from '../lib/api';
import { Button, ErrorMessage, Loading } from '../components/ui';
import { RecordForm, useReferences } from './Admin';
import { PageTitle, useSession } from '../App';

export function DevicePage() {
  const employees = useQuery({
    queryKey: ['enroll-employees'],
    queryFn: () => api('/admin/employees?limit=100'),
  });
  const devices = useQuery({ queryKey: ['devices'], queryFn: () => api('/devices') });
  const client = useQueryClient();
  const [invite, setInvite] = useState(''),
    [inviteHours, setInviteHours] = useState(24),
    [error, setError] = useState<unknown>(null);
  return (
    <>
      <PageTitle
        title="Employee devices"
        description="Issue a one-time invitation after verifying the employee. Replacing enrollment revokes their previous credentials."
      />
      <section className="card settings-card">
        <h3>Enroll or replace a phone</h3>
        {employees.data && (
          <RecordForm
            fields={[
              {
                key: 'membership_id',
                name: 'Employee',
                options: employees.data.items.filter(
                  (e: any) => ['employee', 'manager'].includes(e.role) && e.active
                ),
              },
              { key: 'reason', name: 'Reason / identity verification', type: 'textarea', min: 3 },
            ]}
            submit="Issue replacement invitation"
            onSave={async (value) => {
              const result = await write('/devices/invitations', value);
              setInvite(result.url);
              setInviteHours(result.expires_in_hours);
              await client.invalidateQueries({ queryKey: ['devices'] });
            }}
          />
        )}
        {invite && (
          <div className="info-strip">
            <div>
              <p>
                Share this private link with that employee. It expires in {inviteHours} hours and
                can be used once.
              </p>
              <a href={invite}>{invite}</a>
            </div>
          </div>
        )}
      </section>
      <ErrorMessage error={error || devices.error || employees.error} />
      <section className="card">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Employee</th>
                <th>Enrolled</th>
                <th>Expires</th>
                <th>Status</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {devices.data?.items.map((d: any) => (
                <tr key={d.id}>
                  <td>
                    {d.name}
                    <small>{d.rp_id}</small>
                  </td>
                  <td>{new Date(d.created_at).toLocaleDateString()}</td>
                  <td>{new Date(d.expires_at).toLocaleDateString()}</td>
                  <td>
                    {d.revoked_at
                      ? 'Revoked'
                      : new Date(d.expires_at) < new Date()
                        ? 'Expired'
                        : 'Active'}
                  </td>
                  <td>
                    {!d.revoked_at && (
                      <Button
                        variant="danger"
                        onClick={async () => {
                          const reason = prompt('Reason for revoking this credential');
                          if (!reason) return;
                          try {
                            await write(`/devices/${encodeURIComponent(d.id)}/revoke`, { reason });
                            await client.invalidateQueries({ queryKey: ['devices'] });
                          } catch (e) {
                            setError(e);
                          }
                        }}
                      >
                        Revoke
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}
export function SchedulingPage() {
  const { user } = useSession();
  const refs = useReferences(true);
  const employees = useQuery({
    queryKey: ['schedule-employees'],
    queryFn: () => api('/admin/employees?limit=100'),
  });
  const history = useQuery({ queryKey: ['schedules'], queryFn: () => api('/schedules') });
  const client = useQueryClient();
  const [selected, setSelected] = useState<string[]>([]),
    [cycle, setCycle] = useState<string[]>([]),
    [error, setError] = useState<unknown>(null),
    [message, setMessage] = useState('');
  if (refs.isPending || employees.isPending) return <Loading />;
  return (
    <>
      <PageTitle
        title="Schedule assignments"
        description="Select employees, add a daily rotation, and choose effective dates. A single shift repeats its weekdays; multiple entries repeat in order. Use Rest for a day off."
      />
      <ErrorMessage error={error || refs.error || employees.error || history.error} />
      <section className="card settings-card">
        <fieldset>
          <legend>Employees affected</legend>
          {employees.data?.items.map((e: any) => (
            <label className="checkbox-label" key={e.id}>
              <input
                type="checkbox"
                checked={selected.includes(e.id)}
                onChange={(event) =>
                  setSelected(
                    event.target.checked
                      ? [...selected, e.id]
                      : selected.filter((id) => id !== e.id)
                  )
                }
              />
              {e.name}
            </label>
          ))}
        </fieldset>
        <label>
          Add a day to the rotation
          <select value="" onChange={(e) => setCycle([...cycle, e.target.value])}>
            <option value="">Select shift…</option>
            {refs.data?.shifts.map((s: any) => (
              <option value={s.id} key={s.id}>
                {s.name} ({s.start_time}–{s.end_time})
              </option>
            ))}
            <option value="rest">Rest day</option>
          </select>
        </label>
        <p>
          {cycle
            .map((id) =>
              id === 'rest' ? 'Rest' : refs.data?.shifts.find((s: any) => s.id === id)?.name
            )
            .join(' → ') || 'No rotation selected'}
        </p>
        <Button variant="ghost" onClick={() => setCycle([])}>
          Clear rotation
        </Button>
        <RecordForm
          fields={[
            { key: 'effective_from', name: 'Effective from', type: 'date' },
            {
              key: 'effective_to',
              name: 'Temporary override ends (optional)',
              type: 'date',
              optional: true,
            },
            { key: 'reason', name: 'Reason', type: 'textarea', min: 3 },
          ]}
          initial={{
            effective_from: new Intl.DateTimeFormat('en-CA', { timeZone: user.timezone }).format(
              new Date()
            ),
          }}
          submit="Assign schedule"
          onSave={async (v) => {
            setError(null);
            if (!selected.length || !cycle.length)
              throw new Error('Select employees and at least one shift or rest day.');
            const r = await write('/schedules', {
              ...v,
              membership_ids: selected,
              shift_ids: cycle,
            });
            setMessage(`Schedule assigned to ${r.count} employees.`);
            await client.invalidateQueries({ queryKey: ['schedules'] });
          }}
        />
        <p role="status">{message}</p>
      </section>
      <section className="card">
        <h3>Assignment history</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Employee</th>
                <th>Effective dates</th>
                <th>Rotation</th>
                <th>Changed by</th>
                <th>Reason / previous schedule</th>
              </tr>
            </thead>
            <tbody>
              {history.data?.items.map((s: any) => (
                <tr key={s.id}>
                  <td>{s.employee}</td>
                  <td>
                    {s.effective_from} – {s.effective_to || 'ongoing'}
                  </td>
                  <td>
                    {s.cycle
                      .map((sh: any) =>
                        sh ? `${sh.name} ${sh.start_time}–${sh.end_time}` : 'Rest'
                      )
                      .join(' → ')}
                  </td>
                  <td>
                    {s.actor || 'Migration'}
                    <small>{new Date(s.created_at).toLocaleString()}</small>
                  </td>
                  <td>
                    {s.reason}
                    <details>
                      <summary>Previous schedule</summary>
                      <pre>{JSON.stringify(s.previous_schedule, null, 2)}</pre>
                    </details>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}
export function PlatformPage() {
  const configuration = useQuery({
    queryKey: ['platform-config'],
    queryFn: () => api('/platform/config'),
  });
  const companies = useQuery({
    queryKey: ['platform-companies'],
    queryFn: () => api('/platform/companies'),
  });
  const client = useQueryClient();
  const [error, setError] = useState<unknown>(null),
    [message, setMessage] = useState('');
  return (
    <main className="platform-page">
      <PageTitle
        title="Platform administration"
        description="Provision isolated companies and their owners. Company data stays behind explicitly scoped endpoints."
        action={
          <Button
            onClick={async () => {
              await write('/auth/logout', {});
              location.assign('/admin-login');
            }}
          >
            Sign out
          </Button>
        }
      />
      <ErrorMessage error={error || companies.error} />
      <section className="card settings-card">
        <h3>Provision company owner</h3>
        <RecordForm
          fields={[
            { key: 'company', name: 'Company name' },
            { key: 'name', name: 'Owner name' },
            { key: 'email', name: 'Owner email (personal email supported)', type: 'email' },
            { key: 'password', name: 'Initial owner password', type: 'password' },
            { key: 'timezone', name: 'IANA timezone' },
          ]}
          initial={{ timezone: 'Asia/Phnom_Penh' }}
          submit="Provision company"
          onSave={async (v) => {
            await write('/platform/companies', v);
            setMessage('Company and owner provisioned. Share their credentials privately.');
            await client.invalidateQueries({ queryKey: ['platform-companies'] });
          }}
        />
        <p>{message}</p>
      </section>
      <section className="card">
        <h3>Companies</h3>
        {companies.data?.items.map((co: any) => (
          <div className="platform-company" key={co.id}>
            <h3>{co.name}</h3>
            <RecordForm
              fields={[
                { key: 'active', name: 'Company access enabled', type: 'checkbox' },
                {
                  key: 'subscription',
                  name: 'Subscription',
                  options: ['trial', 'active', 'suspended', 'cancelled'].map((v) => ({
                    id: v,
                    name: v,
                  })),
                },
                { key: 'reason', name: 'Reason', type: 'textarea', min: 3 },
              ]}
              initial={co}
              onSave={async (v) => {
                await write(`/platform/companies/${co.id}`, v, 'PATCH');
                await client.invalidateQueries({ queryKey: ['platform-companies'] });
              }}
            />
          </div>
        ))}
      </section>
      <section className="card settings-card">
        <h3>Platform enrollment configuration</h3>
        <ErrorMessage error={configuration.error} />
        {configuration.data && (
          <RecordForm
            key={JSON.stringify(configuration.data)}
            fields={[
              {
                key: 'invitation_hours',
                name: 'Invitation lifetime (hours)',
                type: 'number',
                min: 1,
                max: 72,
              },
              {
                key: 'credential_days',
                name: 'New credential lifetime (days)',
                type: 'number',
                min: 7,
                max: 365,
              },
              { key: 'support_email', name: 'Support email', type: 'email', optional: true },
              { key: 'reason', name: 'Reason', type: 'textarea', min: 3 },
            ]}
            initial={configuration.data}
            onSave={async (value) => {
              await write('/platform/config', value, 'PATCH');
              await client.invalidateQueries({ queryKey: ['platform-config'] });
            }}
          />
        )}
        <p>
          These lifetimes apply to newly issued invitations and newly enrolled credentials. Existing
          expiry dates remain unchanged.
        </p>
      </section>
      <section className="card settings-card">
        <h3>Verify personal recovery email</h3>
        <RecordForm
          fields={[
            { key: 'email', name: 'Recovery email', type: 'email' },
            { key: 'password', name: 'Current password', type: 'password' },
          ]}
          submit="Send verification link"
          onSave={async (v) => {
            const r = await write('/auth/recovery-email', v);
            setMessage(r.message);
          }}
        />
      </section>
    </main>
  );
}
