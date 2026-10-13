import { useEffect, useRef, useState } from 'react';
import { api, write, setCSRF, time, label } from '../lib/api';
import { passkey, locateForAttendance } from '../lib/passkey';
import { LocationError, type LocationProgress } from '../lib/location';
import { Button, ErrorMessage } from '../components/ui';

export function EmployeeEntry({
  point,
  returnTo = '/self',
}: {
  point?: string;
  returnTo?: string;
}) {
  const [status, setStatus] = useState('Checking your secure session…'),
    [error, setError] = useState<unknown>(null),
    [busy, setBusy] = useState(false),
    [needsPasskey, setNeedsPasskey] = useState(false),
    [result, setResult] = useState<any>(null),
    [identity, setIdentity] = useState<any>(null);
  const [locationProgress, setLocationProgress] = useState<LocationProgress | null>(null);
  const started = useRef(false),
    pending = useRef<any>(null);
  const locationController = useRef<AbortController | null>(null);
  useEffect(() => () => locationController.current?.abort(), []);
  async function record(user: any) {
    setBusy(true);
    setError(null);
    setNeedsPasskey(false);
    setIdentity(user);
    setCSRF(user.csrf);
    const pendingKey = `attendance-pending:${user.membership_id}:${point}`;
    let submissionStarted = false;
    try {
      if (!point) {
        location.assign(returnTo);
        return;
      }
      if (!['employee', 'manager'].includes(user.role)) {
        setNeedsPasskey(true);
        setStatus(
          'Use the employee’s enrolled passkey to record attendance. Administrative attendance remains in the workspace scanner.'
        );
        return;
      }
      setStatus('Checking workplace and location…');
      const workplace = await api(`/points/${encodeURIComponent(point)}`);
      if (!pending.current) {
        try {
          const saved = sessionStorage.getItem(pendingKey);
          if (saved) pending.current = JSON.parse(saved);
        } catch {}
      }
      if (!pending.current) {
        locationController.current?.abort();
        locationController.current = new AbortController();
        setLocationProgress(null);
        pending.current = {
          point,
          action: 'auto',
          idempotency_key: crypto.randomUUID(),
          location: workplace.location_required
            ? await locateForAttendance({
                maximumAccuracy: workplace.max_accuracy_m,
                signal: locationController.current.signal,
                onProgress: (progress) => {
                  setLocationProgress(progress);
                  setStatus(
                    progress.accuracy === null
                      ? 'Acquiring a fresh location…'
                      : `Improving location: ±${Math.ceil(progress.accuracy)} m; need ${progress.maximumAccuracy} m or better.`
                  );
                },
              })
            : null,
        };
      }
      try {
        sessionStorage.setItem(pendingKey, JSON.stringify(pending.current));
      } catch {}
      setStatus('Recording attendance…');
      submissionStarted = true;
      const response = await write('/attendance', pending.current);
      setResult({ ...response, timezone: user.timezone });
      pending.current = null;
      try {
        sessionStorage.removeItem(pendingKey);
      } catch {}
    } catch (e: any) {
      if (e.name === 'AbortError') return;
      setError(e);
      setStatus(
        e instanceof LocationError
          ? 'Location verification needs attention.'
          : submissionStarted && (!e.status || e.status >= 500)
            ? 'Attendance outcome is uncertain. Retry the same request safely.'
            : 'Attendance was not recorded.'
      );
      if (e.status && e.status < 500) {
        pending.current = null;
        try {
          sessionStorage.removeItem(pendingKey);
        } catch {}
      }
      if (e.status === 401) {
        setNeedsPasskey(true);
        setStatus('Device enrollment required');
      }
    } finally {
      setBusy(false);
    }
  }
  useEffect(() => {
    if (started.current) return;
    started.current = true;
    api('/auth/me')
      .then((me) => record(me.user))
      .catch(() => {
        setNeedsPasskey(true);
        setStatus('Confirm your identity with your enrolled passkey.');
      });
  }, []);
  async function authenticate() {
    setBusy(true);
    setError(null);
    try {
      await passkey();
      const me = await api('/auth/me');
      await record(me.user);
    } catch (e) {
      setError(e);
      setBusy(false);
    }
  }
  return (
    <main className="entry-page">
      <section className="card">
        <span className="eyebrow">DEVARAJ TIME · ATTENDANCE</span>
        {result ? (
          <>
            <h1>
              {result.already_recorded ? 'Already recorded' : `${label(result.action)} successful`}
            </h1>
            <p className="attendance-time">{time(result.recorded_at, result.timezone)}</p>
            <p>Your attendance is saved.</p>
          </>
        ) : (
          <>
            <h1>{needsPasskey ? 'Welcome back' : 'Record attendance'}</h1>
            <p role="status">{status}</p>
            {busy && locationProgress && (
              <p>
                {locationProgress.elapsedSeconds} s elapsed · Keep this page open while your phone
                obtains a precise location.
              </p>
            )}
            <ErrorMessage error={error} />
            {error instanceof LocationError && (
              <p>
                On iPhone: Settings → Privacy &amp; Security → Location Services → your browser →
                allow access and turn on Precise Location. You can also open the link directly in
                Safari or Chrome instead of the scanner preview.
              </p>
            )}
            {needsPasskey ? (
              <>
                <Button disabled={busy} onClick={authenticate}>
                  {busy ? 'Verifying…' : 'Continue with passkey'}
                </Button>
                <p>New or replaced phone? Ask HR for a one-time enrollment invitation.</p>
              </>
            ) : (
              error && (
                <Button disabled={busy} onClick={() => record(identity)}>
                  Retry attendance
                </Button>
              )
            )}
          </>
        )}
        <p>
          <a href="/self">Employee self-service</a> ·{' '}
          <a href="/admin-login">Administrative login</a>
        </p>
      </section>
    </main>
  );
}
export function Enrollment() {
  const [token] = useState(() => location.hash.slice(1)),
    [busy, setBusy] = useState(false),
    [done, setDone] = useState(false),
    [error, setError] = useState<unknown>(null);
  useEffect(() => {
    history.replaceState(null, '', location.pathname);
  }, []);
  return (
    <main className="entry-page">
      <section className="card">
        <h1>{done ? 'You’re enrolled' : 'Enroll your attendance passkey'}</h1>
        <p>
          {done
            ? 'Next time, scan the workplace QR with your phone’s Camera app.'
            : 'Use the phone you will bring to work. Your device will ask you to create and verify a passkey.'}
        </p>
        <ErrorMessage error={error} />
        {!done && (
          <Button
            disabled={busy || !token}
            onClick={async () => {
              setBusy(true);
              setError(null);
              try {
                await passkey(token);
                setDone(true);
              } catch (e) {
                setError(e);
              } finally {
                setBusy(false);
              }
            }}
          >
            {busy ? 'Enrolling…' : 'Create attendance passkey'}
          </Button>
        )}
        <p>
          Passkeys may sync through your Apple or Google account. HR revokes the credential on all
          devices when replacing it.
        </p>
      </section>
    </main>
  );
}
export function Recovery({ mode }: { mode: 'request' | 'reset' | 'verify' }) {
  const [token] = useState(() => location.hash.slice(1)),
    [message, setMessage] = useState(''),
    [error, setError] = useState<unknown>(null),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    history.replaceState(null, '', location.pathname);
  }, []);
  return (
    <main className="entry-page">
      <section className="card">
        <h1>
          {mode === 'request'
            ? 'Forgot password'
            : mode === 'reset'
              ? 'Choose a new password'
              : 'Verify recovery email'}
        </h1>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            setError(null);
            const form = new FormData(e.currentTarget);
            try {
              const result = await write(
                mode === 'request'
                  ? '/auth/recovery'
                  : mode === 'reset'
                    ? '/auth/reset'
                    : '/auth/verify-recovery',
                { token, email: form.get('email'), password: form.get('password') }
              );
              setMessage(result.message);
            } catch (e) {
              setError(e);
            } finally {
              setBusy(false);
            }
          }}
        >
          {mode === 'request' && (
            <label>
              Registered or verified recovery email
              <input name="email" type="email" required />
            </label>
          )}
          {mode === 'reset' && (
            <label>
              New password
              <input
                name="password"
                type="password"
                minLength={12}
                maxLength={128}
                autoComplete="new-password"
                required
              />
            </label>
          )}
          <ErrorMessage error={error} />
          <p role="status">{message}</p>
          <Button disabled={busy || !!message}>{busy ? 'Working…' : 'Continue'}</Button>
        </form>
        <p>
          <a href="/admin-login">Return to administrative login</a>
        </p>
      </section>
    </main>
  );
}
