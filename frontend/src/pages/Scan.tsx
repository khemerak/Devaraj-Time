import { useEffect, useRef, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { BrowserQRCodeReader, type IScannerControls } from '@zxing/browser';
import {
  Camera,
  ScanLine,
  MapPin,
  CheckCircle2,
  ShieldCheck,
  RefreshCw,
  Flashlight,
  X,
} from 'lucide-react';
import { api, write, label, pointIdentifier } from '../lib/api';
import { locateForAttendance } from '../lib/location';
import { Button, ErrorMessage, Loading } from '../components/ui';
import { PageTitle, useSession } from '../App';

export function ScanPage() {
  const { point: routePoint } = useParams(),
    { user } = useSession(),
    client = useQueryClient();
  const [point, setPoint] = useState(routePoint || ''),
    [manual, setManual] = useState(''),
    [camera, setCamera] = useState(false),
    [error, setError] = useState<unknown>(null),
    [busy, setBusy] = useState(false),
    [position, setPosition] = useState<any>(null),
    [result, setResult] = useState<any>(null),
    [light, setLight] = useState(false),
    [retry, setRetry] = useState<any>(null);
  const video = useRef<HTMLVideoElement>(null),
    controls = useRef<IScannerControls | null>(null),
    found = useRef(false);
  const resolved = useQuery({
    queryKey: ['point', point],
    queryFn: () => api(`/points/${encodeURIComponent(point)}`),
    enabled: !!point,
  });
  const dashboard = useQuery({ queryKey: ['dashboard'], queryFn: () => api('/dashboard') });
  const org = useQuery({ queryKey: ['organization'], queryFn: () => api('/organization') });
  const state = dashboard.data?.own[0]?.state || '',
    actions =
      state === 'checked_in'
        ? ['check_out', ...(org.data?.breaks_enabled ? ['start_break'] : [])]
        : state === 'on_break'
          ? ['end_break']
          : ['check_in'];
  const [selected, setSelected] = useState('check_in');
  useEffect(() => {
    setSelected(actions[0]);
  }, [state, org.data?.breaks_enabled]);
  useEffect(() => {
    if (!camera) return;
    let cancelled = false;
    found.current = false;
    const reader = new BrowserQRCodeReader();
    reader
      .decodeFromVideoDevice(undefined, video.current!, (decoded, _, scanner) => {
        if (decoded && !found.current) {
          try {
            const id = pointIdentifier(decoded.getText());
            found.current = true;
            if (['employee', 'manager'].includes(user.role)) {
              scanner.stop();
              location.assign(`/scan/${id}`);
              return;
            }
            setPoint(id);
            setPosition(null);
            scanner.stop();
            setCamera(false);
          } catch (e) {
            setError(e);
          }
        }
      })
      .then((c) => {
        if (cancelled) c.stop();
        else controls.current = c;
      })
      .catch((e) => {
        setCamera(false);
        setError(
          new Error(
            'Camera unavailable or permission denied. You can open the printed attendance URL or enter its identifier below.'
          )
        );
      });
    return () => {
      cancelled = true;
      controls.current?.stop();
      controls.current = null;
    };
  }, [camera]);
  async function locate(): Promise<any> {
    return locateForAttendance({
      maximumAccuracy: resolved.data?.max_accuracy_m ?? 75,
      onProgress: (progress) =>
        setPosition(
          progress.accuracy === null ? null : { accuracy: progress.accuracy, acquiring: true }
        ),
    });
  }
  async function verify() {
    setBusy(true);
    setError(null);
    try {
      setPosition(await locate());
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  async function confirm() {
    setBusy(true);
    setError(null);
    try {
      let payload = retry;
      if (!payload) {
        const fresh = resolved.data.location_required ? await locate() : null;
        setPosition(fresh);
        payload = {
          point,
          action: selected,
          idempotency_key: crypto.randomUUID(),
          location: fresh,
        };
        setRetry(payload);
      }
      const response = await write('/attendance', payload);
      setResult(response);
      setRetry(null);
      await client.invalidateQueries();
    } catch (e: any) {
      setError(e);
      if (e.status && e.status !== 500) setRetry(null);
    } finally {
      setBusy(false);
    }
  }
  async function torch() {
    try {
      if (controls.current?.switchTorch) {
        await controls.current.switchTorch(!light);
        setLight(!light);
      } else throw new Error('Flashlight is not supported by this camera.');
    } catch (e) {
      setError(e);
    }
  }
  if (result)
    return (
      <>
        <PageTitle
          title="You’re all set"
          description="Your attendance was committed using the server’s time."
        />
        <section className="card scan-success">
          <span className="success-icon">
            <CheckCircle2 size={46} />
          </span>
          <h2>{label(result.action)} recorded</h2>
          <p>
            {user.name} · {result.workplace || resolved.data?.workplace}
          </p>
          <strong>
            {new Intl.DateTimeFormat('en', {
              dateStyle: 'full',
              timeStyle: 'medium',
              timeZone: user.timezone,
            }).format(new Date(result.recorded_at))}
          </strong>
          <small>{user.timezone}</small>
          <div className="info-strip">
            <ShieldCheck />
            {label(result.verification)}
          </div>
          <Button asChild>
            <Link to={`/attendance/${result.session_id}`}>View attendance details</Link>
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              setResult(null);
              setPoint('');
              setPosition(null);
            }}
          >
            Record another action
          </Button>
        </section>
      </>
    );
  return (
    <>
      <PageTitle
        title="Record attendance"
        description="Scan your workplace QR. Verify your location. Confirm your day."
      />
      <div className="scan-layout">
        <section className="card camera-card">
          <div className="card-heading">
            <div>
              <h3>Scan your workplace code</h3>
              <p>Camera access is used only on this screen.</p>
            </div>
            <ScanLine />
          </div>
          <div className={`camera-view ${camera ? 'active' : ''}`}>
            {camera ? (
              <video ref={video} muted playsInline />
            ) : (
              <div className="camera-idle">
                <ScanLine size={70} />
                <h3>{point ? 'QR code identified' : 'Ready when you are'}</h3>
                <p>
                  Point your camera at the printed
                  <br />
                  Devaraj Time QR code.
                </p>
              </div>
            )}
            <div className="scan-corners" />
          </div>
          <div className="camera-actions">
            <Button
              onClick={() => {
                setCamera(!camera);
                setError(null);
              }}
              disabled={busy || !!retry}
            >
              {camera ? <X size={18} /> : <Camera size={18} />}{' '}
              {camera ? 'Stop camera' : 'Start camera'}
            </Button>
            {camera && (
              <Button variant="outline" onClick={torch} aria-label="Toggle flashlight">
                <Flashlight size={18} />
              </Button>
            )}
          </div>
          <details>
            <summary>Open a printed URL or enter the QR identifier</summary>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                try {
                  setPoint(pointIdentifier(manual));
                  setError(null);
                  setPosition(null);
                  setRetry(null);
                } catch (err) {
                  setError(err);
                }
              }}
            >
              <label>
                Printed attendance URL or identifier
                <input
                  disabled={!!retry}
                  value={manual}
                  onChange={(e) => setManual(e.target.value)}
                  required
                  placeholder="https://…/scan/…"
                />
              </label>
              <Button disabled={!!retry} variant="outline">
                Read attendance point
              </Button>
            </form>
            <small>This fallback still requires account, branch and location verification.</small>
          </details>
        </section>
        <section className="card verification-card">
          <span className="eyebrow">BEFORE YOU CONFIRM</span>
          <h2>Make this moment count.</h2>
          <div className="scan-step">
            <span>1</span>
            <div>
              <h3>Your workplace</h3>
              {resolved.isFetching ? (
                <Loading />
              ) : resolved.data ? (
                <p>
                  <strong>{resolved.data.workplace}</strong>
                  <br />
                  {resolved.data.address}
                  <br />
                  {resolved.data.name}
                </p>
              ) : (
                <p>Scan a QR to identify your workplace.</p>
              )}
            </div>
          </div>
          <div className="scan-step">
            <span>2</span>
            <div>
              <h3>Location verification</h3>
              <p>
                {position
                  ? `${position.acquiring ? 'Improving location' : 'Position received'} · ±${Math.ceil(position.accuracy)} m reported error`
                  : resolved.data?.location_required === false
                    ? 'Location is not required by this workplace policy.'
                    : 'Location is requested only for this attendance event.'}
              </p>
              {resolved.data?.location_required && (
                <Button variant="outline" onClick={verify} disabled={busy}>
                  <MapPin size={16} />
                  {position ? 'Refresh position' : 'Get my location'}
                </Button>
              )}
            </div>
          </div>
          <div className="scan-step">
            <span>3</span>
            <div>
              <h3>Confirm your action</h3>
              <p>Current state: {state ? label(state) : 'Not checked in'}</p>
              <select
                aria-label="Attendance action"
                value={selected}
                onChange={(e) => {
                  setSelected(e.target.value);
                  setRetry(null);
                }}
                disabled={!!retry}
              >
                {actions.map((a) => (
                  <option key={a} value={a}>
                    {label(a)}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <ErrorMessage error={error || resolved.error || dashboard.error} />
          {!navigator.onLine && (
            <div className="error">
              You’re offline. Attendance cannot be recorded. Prepare an exception request when
              connected.
            </div>
          )}
          <Button
            className="confirm-attendance"
            disabled={!resolved.data || busy || dashboard.isPending || !navigator.onLine}
            onClick={confirm}
          >
            {busy
              ? 'Verifying and recording…'
              : retry
                ? 'Retry the same confirmed action'
                : `Confirm ${label(selected).toLowerCase()}`}
          </Button>
          <p className="privacy-caption">
            <ShieldCheck size={14} />
            The server checks the workplace boundary and records the authoritative time. QR + GPS is
            evidence, not proof of presence.
          </p>
          <Link to="/corrections">Having trouble? Request an attendance exception →</Link>
        </section>
      </div>
    </>
  );
}
