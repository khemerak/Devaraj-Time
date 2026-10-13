import {
  createContext,
  useContext,
  useState,
  useEffect,
  lazy,
  Suspense,
  type ReactNode,
} from 'react';
import { Link, NavLink, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Clock3,
  LayoutDashboard,
  ScanLine,
  CalendarDays,
  Users,
  Building2,
  Layers3,
  MapPin,
  QrCode,
  ClipboardCheck,
  FileBarChart2,
  ShieldCheck,
  Settings,
  Bell,
  LogOut,
  Menu,
  Sun,
  Moon,
  ChevronDown,
  ArrowUpRight,
  Check,
  Plus,
  X,
  Search,
} from 'lucide-react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
const TrendChart = lazy(() => import('./components/TrendChart'));
import { api, write, setCSRF, label, time, hours, APIError } from './lib/api';
import { Button, Loading, ErrorMessage, Empty, Badge, Modal } from './components/ui';
import ProfileControls from './components/ProfileControls';
import { EmployeeEntry, Enrollment, Recovery } from './pages/Entry';
const DevicePage = lazy(() =>
  import('./pages/Management').then((m) => ({ default: m.DevicePage }))
);
const SchedulingPage = lazy(() =>
  import('./pages/Management').then((m) => ({ default: m.SchedulingPage }))
);
const PlatformPage = lazy(() =>
  import('./pages/Management').then((m) => ({ default: m.PlatformPage }))
);
const AdminPage = lazy(() => import('./pages/Admin').then((m) => ({ default: m.AdminPage })));
const SettingsPage = lazy(() => import('./pages/Admin').then((m) => ({ default: m.SettingsPage })));
const QRPage = lazy(() => import('./pages/Admin').then((m) => ({ default: m.QRPage })));
const ScanPage = lazy(() => import('./pages/Scan').then((m) => ({ default: m.ScanPage })));
const RequestsPage = lazy(() =>
  import('./pages/Records').then((m) => ({ default: m.RequestsPage }))
);
const HistoryPage = lazy(() => import('./pages/Records').then((m) => ({ default: m.HistoryPage })));
const ReportsPage = lazy(() => import('./pages/Records').then((m) => ({ default: m.ReportsPage })));
const AuditPage = lazy(() => import('./pages/Records').then((m) => ({ default: m.AuditPage })));
const SchedulePage = lazy(() =>
  import('./pages/Records').then((m) => ({ default: m.SchedulePage }))
);

const Session = createContext<any>(null);
export function useSession() {
  return useContext(Session);
}
export function PageTitle({
  eyebrow = 'YOUR WORKSPACE',
  title,
  description,
  action,
}: {
  eyebrow?: string;
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="page-title">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {action}
    </div>
  );
}
export default function App() {
  const path = location.pathname;
  if (path === '/enroll') return <Enrollment />;
  if (path === '/forgot-password') return <Recovery mode="request" />;
  if (path === '/reset') return <Recovery mode="reset" />;
  if (path === '/verify-recovery') return <Recovery mode="verify" />;
  if (/^\/scan\/[A-Za-z0-9_-]{43}$/.test(path))
    return <EmployeeEntry point={path.split('/').pop()} />;
  if (path === '/employee') return <EmployeeEntry />;
  return <Workspace />;
}
function Workspace() {
  const me = useQuery({ queryKey: ['me'], queryFn: () => api('/auth/me') });
  const client = useQueryClient();
  if (me.isPending) return <Loading />;
  if (me.error) {
    if (me.error instanceof APIError && me.error.status === 401)
      if (
        ['/self', '/attendance', '/schedule', '/corrections', '/leave', '/profile'].some(
          (path) => location.pathname === path || location.pathname.startsWith(path + '/')
        )
      )
        return <EmployeeEntry returnTo={location.pathname} />;
    if (me.error instanceof APIError && me.error.status === 401)
      return (
        <Auth
          onSuccess={() => {
            client.clear();
            location.assign('/');
          }}
        />
      );
    return (
      <div className="auth-page">
        <ErrorMessage error={me.error} />
        <Button onClick={() => me.refetch()}>Retry connection</Button>
      </div>
    );
  }
  setCSRF(me.data.user.csrf);
  if (me.data.user.role === 'system_admin')
    return (
      <Suspense fallback={<Loading />}>
        <PlatformPage />
      </Suspense>
    );
  return (
    <Session.Provider value={me.data}>
      <Shell />
    </Session.Provider>
  );
}
const schema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
  name: z.string().optional(),
  company: z.string().optional(),
  timezone: z.string().optional(),
});
function Auth({ onSuccess }: { onSuccess: () => void }) {
  const [error, setError] = useState<unknown>(null);
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { timezone: 'Asia/Phnom_Penh' },
  });
  async function submit(values: z.infer<typeof schema>) {
    setError(null);
    try {
      await write('/auth/login', values);
      onSuccess();
    } catch (e) {
      setError(e);
    }
  }
  return (
    <div className="auth-page">
      <div className="auth-brand">
        <div className="brand">
          <span className="brand-icon">
            <Clock3 />
          </span>
          devaraj<span>time</span>
        </div>
        <span className="eyebrow">A LITTLE MORE CLARITY, EVERY DAY.</span>
        <h1>
          Good work starts
          <br />
          with trust.
        </h1>
        <p>
          Smarter attendance.
          <br />
          Fairer workplaces.
        </p>
        <div className="auth-art">
          <Clock3 size={120} />
          <span>
            <Check /> Every moment accounted for
          </span>
        </div>
        <small>Attendance that puts people first.</small>
      </div>
      <div className="auth-form">
        <span className="eyebrow">WELCOME TO DEVARAJ TIME</span>
        <h2>Administrative login</h2>
        <p>For platform administrators, company owners, administrators and HR.</p>
        <form onSubmit={form.handleSubmit(submit)}>
          <label>
            Work email
            <input type="email" {...form.register('email')} autoComplete="username" />
          </label>
          <label>
            Password
            <input
              type="password"
              {...form.register('password')}
              autoComplete="current-password"
              minLength={1}
            />
          </label>
          {Object.values(form.formState.errors).map((e, n) => (
            <ErrorMessage key={n} error={e.message} />
          ))}
          <ErrorMessage error={error} />
          <Button disabled={form.formState.isSubmitting}>
            {form.formState.isSubmitting ? 'Connecting…' : 'Sign in'}
            <ArrowUpRight size={17} />
          </Button>
        </form>
        <p>
          <a href="/forgot-password">Forgot password?</a>
        </p>
        <p>
          <a href="/employee">Employee attendance — continue with passkey</a>
        </p>
        <small>
          Accounts are provisioned by your administrator. Ask HR for an employee enrollment
          invitation.
        </small>
      </div>
    </div>
  );
}
const navigation = [
  { label: 'Overview', path: '/', icon: LayoutDashboard },
  { label: 'Scan attendance', path: '/scan', icon: ScanLine },
  { label: 'Attendance', path: '/attendance', icon: CalendarDays },
  { label: 'My schedule', path: '/schedule', icon: Clock3 },
  { label: 'Corrections', path: '/corrections', icon: ClipboardCheck },
  { label: 'Leave requests', path: '/leave', icon: CalendarDays },
  { label: 'Employees', path: '/employees', icon: Users, team: true },
  { label: 'Departments', path: '/departments', icon: Layers3, configure: true },
  { label: 'Branches', path: '/branches', icon: Building2, configure: true },
  { label: 'Shifts', path: '/shifts', icon: Clock3, scheduling: true },
  { label: 'Scheduling', path: '/scheduling', icon: CalendarDays, scheduling: true },
  { label: 'Employee devices', path: '/devices', icon: ShieldCheck, scheduling: true },
  { label: 'Workplaces', path: '/workplaces', icon: MapPin, configure: true },
  { label: 'QR codes', path: '/qr', icon: QrCode, configure: true },
  { label: 'Reports', path: '/reports', icon: FileBarChart2, team: true },
  { label: 'Audit trail', path: '/audit', icon: ShieldCheck, configure: true },
  { label: 'Settings', path: '/settings', icon: Settings, configure: true },
];
function Shell() {
  const { user, companies } = useSession();
  const team = user.role !== 'employee',
    configure = ['admin', 'owner'].includes(user.role);
  const [menu, setMenu] = useState(false),
    [notifications, setNotifications] = useState(false),
    [dark, setDark] = useState(localStorage.getItem('theme') === 'dark');
  const [mobile, setMobile] = useState(window.matchMedia('(max-width: 800px)').matches);
  useEffect(() => {
    const media = window.matchMedia('(max-width: 800px)');
    const update = () => {
      setMobile(media.matches);
      if (!media.matches) setMenu(false);
    };
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, []);
  useEffect(() => {
    if (!menu) return;
    const close = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setMenu(false);
        requestAnimationFrame(() =>
          document.querySelector<HTMLButtonElement>('button[aria-label="Open navigation"]')?.focus()
        );
      }
    };
    document.addEventListener('keydown', close);
    return () => document.removeEventListener('keydown', close);
  }, [menu]);
  const client = useQueryClient(),
    navigate = useNavigate();
  const notices = useQuery({
    queryKey: ['notifications'],
    queryFn: () => api('/notifications'),
    refetchInterval: 30000,
  });
  const [error, setError] = useState<unknown>(null);
  useEffect(() => {
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
    localStorage.setItem('theme', dark ? 'dark' : 'light');
  }, [dark]);
  async function logout() {
    try {
      await write('/auth/logout', {});
      client.clear();
      location.assign('/');
    } catch (e) {
      setError(e);
    }
  }
  async function switchCompany(id: string) {
    try {
      await write('/auth/company', { id });
      client.clear();
      location.assign('/');
    } catch (e) {
      setError(e);
    }
  }
  return (
    <div className="app-shell">
      <aside
        inert={mobile && !menu ? true : undefined}
        className={menu ? 'sidebar visible' : 'sidebar'}
      >
        {mobile && menu && (
          <Button variant="ghost" aria-label="Close navigation" onClick={() => setMenu(false)}>
            <X size={20} />
            Close menu
          </Button>
        )}
        <Link to="/" className="brand">
          <span className="brand-icon">
            <Clock3 />
          </span>
          devaraj<span>time</span>
        </Link>
        <div className="company-picker">
          <span className="company-avatar">{user.company.slice(0, 1)}</span>
          <div>
            <strong>{user.company}</strong>
            <small>{label(user.role)} workspace</small>
          </div>
          {companies.length > 1 && (
            <select
              aria-label="Active company"
              value={user.company_id}
              onChange={(e) => switchCompany(e.target.value)}
            >
              {companies.map((co: any) => (
                <option key={co.id} value={co.id}>
                  {co.name}
                </option>
              ))}
            </select>
          )}
          <ChevronDown size={15} />
        </div>
        <span className="nav-label">WORKSPACE</span>
        <nav>
          {navigation
            .filter(
              (n) =>
                (!n.team || team) &&
                (!n.configure || configure) &&
                (!n.scheduling || ['owner', 'admin', 'hr'].includes(user.role))
            )
            .map((n) => (
              <NavLink key={n.path} to={n.path} end={n.path === '/'} onClick={() => setMenu(false)}>
                <n.icon size={18} />
                {n.label}
              </NavLink>
            ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="privacy-note">
            <ShieldCheck size={20} />
            <strong>Built on trust</strong>
            <p>Your location is requested only when you record attendance.</p>
          </div>
          <Button variant="ghost" onClick={logout}>
            <LogOut size={17} />
            Sign out
          </Button>
        </div>
      </aside>
      {menu && <div className="mobile-backdrop" onClick={() => setMenu(false)} />}
      <div className="main-shell" inert={mobile && menu ? true : undefined}>
        <header className="topbar">
          <Button
            variant="ghost"
            className="mobile-toggle"
            aria-label="Open navigation"
            onClick={() => setMenu(true)}
          >
            <Menu />
          </Button>
          <div className="breadcrumb">
            Workspace <span>/</span>{' '}
            {navigation.find((n) => n.path === location.pathname)?.label || 'Attendance'}
          </div>
          <div className="top-actions">
            <Button variant="ghost" aria-label="Toggle color theme" onClick={() => setDark(!dark)}>
              {dark ? <Sun size={19} /> : <Moon size={19} />}
            </Button>
            <div className="notification-wrap">
              <Button
                variant="ghost"
                aria-label="Notifications"
                onClick={() => setNotifications(!notifications)}
              >
                <Bell size={19} />
                {notices.data?.some((n: any) => !n.read_at) && <i className="notification-dot" />}
              </Button>
              {notifications && (
                <div className="notification-pop">
                  <h3>Notifications</h3>
                  <ErrorMessage error={notices.error} />
                  {notices.data?.length ? (
                    notices.data.map((n: any) => (
                      <p key={n.id}>
                        {n.message}
                        <small>{new Date(n.created_at).toLocaleDateString()}</small>
                      </p>
                    ))
                  ) : (
                    <p>You’re all caught up.</p>
                  )}
                  <Button
                    variant="ghost"
                    onClick={async () => {
                      try {
                        await write('/notifications/read', {});
                        await client.invalidateQueries({ queryKey: ['notifications'] });
                      } catch (e) {
                        setError(e);
                      }
                    }}
                  >
                    Mark all as read
                  </Button>
                </div>
              )}
            </div>
            <Link to="/profile" className="account">
              <span className="avatar">
                {user.name
                  .split(' ')
                  .map((x: string) => x[0])
                  .slice(0, 2)
                  .join('')}
              </span>
              <div>
                <strong>{user.name}</strong>
                <small>{label(user.role)}</small>
              </div>
            </Link>
          </div>
        </header>
        <main>
          <ErrorMessage error={error} />
          <Suspense fallback={<Loading />}>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/self" element={<Dashboard />} />
              <Route path="/scan" element={<ScanPage />} />
              <Route path="/scan/:point" element={<ScanPage />} />
              <Route path="/attendance" element={<HistoryPage />} />
              <Route path="/attendance/:id" element={<HistoryPage />} />
              <Route path="/schedule" element={<SchedulePage />} />
              <Route path="/corrections" element={<RequestsPage kind="correction" />} />
              <Route path="/leave" element={<RequestsPage kind="leave" />} />
              <Route path="/profile" element={<Profile />} />
              {team && <Route path="/employees" element={<AdminPage resource="employees" />} />}
              {['owner', 'admin', 'hr'].includes(user.role) && (
                <>
                  <Route path="/shifts" element={<AdminPage resource="shifts" />} />
                  <Route path="/scheduling" element={<SchedulingPage />} />
                  <Route path="/devices" element={<DevicePage />} />
                </>
              )}
              <Route path="/reports" element={team ? <ReportsPage /> : <Navigate to="/" />} />
              {configure && (
                <>
                  {['departments', 'branches', 'workplaces'].map((resource) => (
                    <Route
                      key={resource}
                      path={`/${resource}`}
                      element={<AdminPage resource={resource} />}
                    />
                  ))}
                  <Route path="/qr" element={<QRPage />} />
                  <Route path="/settings" element={<SettingsPage />} />
                  <Route path="/audit" element={<AuditPage />} />
                </>
              )}
              <Route path="*" element={<Navigate to="/" />} />
            </Routes>
          </Suspense>
          <footer>
            Devaraj Time <span>Smarter Attendance. Fairer Workplaces.</span>
            <Link to="/profile">Your data & privacy</Link>
          </footer>
        </main>
      </div>
    </div>
  );
}
function Dashboard() {
  const { user } = useSession();
  const data = useQuery({
    queryKey: ['dashboard'],
    queryFn: () => api('/dashboard'),
    refetchInterval: 15000,
  });
  const org = useQuery({ queryKey: ['organization'], queryFn: () => api('/organization') });
  const recent = useQuery({
    queryKey: ['recent'],
    queryFn: () => api('/attendance?limit=6'),
    refetchInterval: 15000,
  });
  if (data.isPending) return <Loading />;
  if (data.error) return <ErrorMessage error={data.error} />;
  const d = data.data,
    m = d.metrics,
    own = d.own[0],
    team = user.role !== 'employee';
  const cards = team
    ? [
        {
          title: 'Active employees',
          value: m.active,
          caption: 'Across your authorized scope',
          icon: Users,
          color: 'blue',
        },
        {
          title: 'Currently working',
          value: m.checked_in,
          caption: `${m.on_break} on a recorded break`,
          icon: Clock3,
          color: 'green',
        },
        {
          title: 'Late arrivals',
          value: m.late,
          caption: 'Beyond the assigned grace period',
          icon: CalendarDays,
          color: 'amber',
        },
        {
          title: 'Pending requests',
          value: d.pending,
          caption: 'Awaiting a review decision',
          icon: ClipboardCheck,
          color: 'purple',
        },
      ]
    : [
        {
          title: 'Attendance state',
          value: own ? label(own.state) : 'Not checked in',
          caption: 'Your latest attendance session',
          icon: Clock3,
          color: 'blue',
        },
        {
          title: 'Recorded work',
          value: own ? hours(own.worked_minutes) : '0h 0m',
          caption: 'Calculated at check-out',
          icon: CalendarDays,
          color: 'green',
        },
        {
          title: 'My requests',
          value: d.pending,
          caption: 'Awaiting a review decision',
          icon: ClipboardCheck,
          color: 'amber',
        },
      ];
  return (
    <>
      <PageTitle
        eyebrow={team ? 'TEAM AT A GLANCE' : 'YOUR DAY, MADE CLEAR'}
        title={`Good ${new Date().getHours() < 12 ? 'morning' : 'afternoon'}, ${user.name.split(' ')[0]}`}
        description={`${new Intl.DateTimeFormat('en', { dateStyle: 'full', timeZone: user.timezone }).format(new Date())} · ${user.timezone}`}
        action={
          <Button asChild>
            <Link to="/scan">
              <ScanLine size={18} />
              Record attendance
            </Link>
          </Button>
        }
      />
      <div className="welcome-banner">
        <div>
          <span className="live-pill">
            <i />
            YOUR WORKSPACE IS LIVE
          </span>
          <h2>A clearer picture of your workday.</h2>
          <p>
            {team
              ? 'Stay close to your team. Review attendance, resolve requests, and keep everyone on the same page.'
              : 'Record your day, review your hours, and get answers when something needs a second look.'}
          </p>
        </div>
        <div className="banner-clock">
          <Clock3 size={76} />
          <span>
            {org.data?.shift
              ? `${org.data.shift.start_time} — ${org.data.shift.end_time}`
              : 'No shift assigned'}
          </span>
          <small>{org.data?.shift?.name || 'Contact your administrator'}</small>
        </div>
      </div>
      <div className="metrics">
        {cards.map((card) => (
          <article className="metric" key={card.title}>
            <div className="metric-top">
              <span>{card.title}</span>
              <span className={`metric-icon ${card.color}`}>
                <card.icon size={19} />
              </span>
            </div>
            <strong>{card.value}</strong>
            <small>{card.caption}</small>
          </article>
        ))}
      </div>
      <div className="dashboard-grid">
        <section className="card chart-card">
          <div className="card-heading">
            <div>
              <h3>Attendance pulse</h3>
              <p>Recorded attendance over the last seven days</p>
            </div>
            <span className="badge blue">Last 7 days</span>
          </div>
          {d.trend.length ? (
            <Suspense fallback={<Loading />}>
              <TrendChart data={d.trend} />
            </Suspense>
          ) : (
            <Empty title="Your attendance story starts here">
              Your first attendance records will bring this chart to life.
            </Empty>
          )}
          <div className="chart-legend">
            <span>
              <i />
              Recorded sessions
            </span>
            <span>
              <i />
              Late arrivals
            </span>
          </div>
        </section>
        <section className="card today-card">
          <div className="card-heading">
            <div>
              <h3>Today’s snapshot</h3>
              <p>Transparent, server-recorded attendance</p>
            </div>
            <CalendarDays size={19} />
          </div>
          {team ? (
            <div className="snapshot">
              {[
                ['Checked out', m.checked_out],
                ['On break', m.on_break],
                ['On approved leave', m.on_leave],
                ['No record yet', m.not_recorded],
              ].map(([key, v]) => (
                <div key={key}>
                  <span>{key}</span>
                  <strong>{v}</strong>
                </div>
              ))}
              <small>
                “No record yet” does not imply absence and includes unscheduled employees.
              </small>
            </div>
          ) : (
            <div className="snapshot">
              <div>
                <span>Check-in</span>
                <strong>{time(own?.check_in, user.timezone)}</strong>
              </div>
              <div>
                <span>Check-out</span>
                <strong>{time(own?.check_out, user.timezone)}</strong>
              </div>
              <div>
                <span>Shift</span>
                <strong>{org.data?.shift?.name || 'Unassigned'}</strong>
              </div>
              <div>
                <span>Grace period</span>
                <strong>{org.data?.shift?.grace_minutes ?? '—'} min</strong>
              </div>
            </div>
          )}
          <Button variant="outline" asChild>
            <Link to={team ? '/corrections' : '/schedule'}>
              {team ? 'Review requests' : 'View my schedule'}
              <ArrowUpRight size={16} />
            </Link>
          </Button>
        </section>
      </div>
      <section className="card">
        <div className="card-heading">
          <div>
            <h3>{team ? 'Recent team attendance' : 'My recent attendance'}</h3>
            <p>Real records. Clear timelines.</p>
          </div>
          <Button variant="ghost" asChild>
            <Link to="/attendance">
              View all
              <ArrowUpRight size={16} />
            </Link>
          </Button>
        </div>
        <ErrorMessage error={recent.error} />
        {recent.isPending ? (
          <Loading />
        ) : recent.data?.items.length ? (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Employee</th>
                  <th>Work date</th>
                  <th>Check-in</th>
                  <th>Check-out</th>
                  <th>Worked</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {recent.data.items.map((r: any) => (
                  <tr key={r.id}>
                    <td>
                      <Link to={`/attendance/${r.id}`} className="person">
                        <span className="avatar subtle">{r.name[0]}</span>
                        <div>
                          <strong>{r.name}</strong>
                          <small>{r.employee_code}</small>
                        </div>
                      </Link>
                    </td>
                    <td>{r.work_date}</td>
                    <td>{time(r.check_in, user.timezone)}</td>
                    <td>{time(r.check_out, user.timezone)}</td>
                    <td>{hours(r.worked_minutes)}</td>
                    <td>
                      <Badge value={r.status} />
                      {r.adjusted && <small>Adjusted</small>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty title="No attendance records yet" />
        )}
      </section>
    </>
  );
}
function Profile() {
  const { user } = useSession();
  const org = useQuery({ queryKey: ['organization'], queryFn: () => api('/organization') });
  return (
    <>
      <PageTitle
        title="My profile"
        description="Your workplace identity and how attendance data is used."
      />
      <section className="card profile">
        <span className="avatar large">{user.name[0]}</span>
        <h2>{user.name}</h2>
        <p>{user.email}</p>
        <Badge value={user.role} />
        <ProfileControls name={user.name} />
        <div className="detail-grid">
          <div>
            <small>Company</small>
            <strong>{user.company}</strong>
          </div>
          <div>
            <small>Employee code</small>
            <strong>{org.data?.employee_code}</strong>
          </div>
          <div>
            <small>Department</small>
            <strong>{org.data?.department || 'Unassigned'}</strong>
          </div>
          <div>
            <small>Branch</small>
            <strong>{org.data?.branch || 'Unassigned'}</strong>
          </div>
        </div>
        <h3>Your attendance, your information</h3>
        <p>
          Location is collected only when you explicitly record attendance. It is used to check the
          configured workplace boundary. Browser location can be spoofed and is evidence, not proof
          of physical presence. Original attendance events and approved corrections remain visible
          to you.
        </p>
        <p>
          Contact your company administrator to update your identity, resolve access issues, or
          request data access or deletion. Precise location evidence is erased by a scheduled
          administrator retention job; original clock events remain preserved.
        </p>
      </section>
    </>
  );
}
