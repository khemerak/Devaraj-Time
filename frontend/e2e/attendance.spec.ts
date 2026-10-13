import { test, expect, request, type APIRequestContext, type Page } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';

const env = Object.fromEntries(
  readFileSync('../.env', 'utf8')
    .split(/\r?\n/)
    .filter((l) => /^[A-Z_]+=/.test(l))
    .map((l) => [l.slice(0, l.indexOf('=')), l.slice(l.indexOf('=') + 1)])
);
const origin = process.env.E2E_URL || env.PUBLIC_URL;
let point: string,
  employee: string,
  invitation: string,
  owner: APIRequestContext,
  csrf = '';
async function mutate(path: string, data: any) {
  const r = await owner.post(origin + '/api' + path, {
    headers: { Origin: origin, 'X-CSRF-Token': csrf },
    data,
  });
  expect(r.ok(), await r.text()).toBeTruthy();
  return r.json();
}
async function virtualPasskey(page: Page) {
  const session = await page.context().newCDPSession(page);
  await session.send('WebAuthn.enable');
  const result = await session.send('WebAuthn.addVirtualAuthenticator', {
    options: {
      protocol: 'ctap2',
      ctap2Version: 'ctap2_1',
      transport: 'internal',
      hasResidentKey: true,
      hasUserVerification: true,
      isUserVerified: true,
      automaticPresenceSimulation: true,
    },
  });
  return { session, id: result.authenticatorId };
}
test.beforeAll(async () => {
  test.setTimeout(180000);
  const platform = await request.newContext({ baseURL: origin }),
    password = env.DEMO_PASSWORD;
  const login = await platform.post('/api/auth/login', {
    headers: { Origin: origin },
    data: {
      email: process.env.E2E_PLATFORM_EMAIL || 'platform@devaraj.example',
      password: process.env.E2E_PLATFORM_PASSWORD || password,
    },
  });
  expect(login.status()).toBe(200);
  const pm = await (await platform.get('/api/auth/me')).json(),
    suffix = randomUUID().slice(0, 8),
    email = `owner-${suffix}@gmail.com`;
  const provision = await platform.post('/api/platform/companies', {
    headers: { Origin: origin, 'X-CSRF-Token': pm.user.csrf },
    data: { company: `Passkey QA ${suffix}`, name: 'QA Owner', email, password, timezone: 'UTC' },
  });
  expect(provision.status(), await provision.text()).toBe(201);
  owner = await request.newContext({ baseURL: origin });
  expect(
    (
      await owner.post('/api/auth/login', {
        headers: { Origin: origin },
        data: { email, password },
      })
    ).status()
  ).toBe(200);
  const me = await (await owner.get('/api/auth/me')).json();
  csrf = me.user.csrf;
  const refs = await (await owner.get('/api/references')).json(),
    branch = refs.branches[0].id;
  const shift = await mutate('/admin/shifts', {
    name: 'All-day test',
    start_time: '00:00',
    end_time: '23:59',
    expected_minutes: 480,
    grace_minutes: 10,
    early_window_minutes: 720,
    workdays: [0, 1, 2, 3, 4, 5, 6],
    reason: 'Browser test shift',
  });
  const workplace = await mutate('/admin/workplaces', {
    name: 'Test entrance',
    branch_id: branch,
    address: 'GPS fixture only',
    latitude: 11.5564,
    longitude: 104.9282,
    radius_m: 150,
    max_accuracy_m: 75,
    location_required: true,
    active: true,
    reason: 'Browser geofence fixture',
  });
  const p = await mutate('/admin/points', {
      name: 'Permanent entrance',
      workplace_id: workplace.id,
      active: true,
      reason: 'Browser QR test',
    }),
    ps = await (await owner.get('/api/admin/points')).json();
  point = ps.items.find((row: any) => row.id === p.id).public_id;
  const emp = await mutate('/admin/employees', {
    name: 'Passkey Employee',
    email: `employee-${suffix}@e2e.example`,
    employee_code: 'QA-EMP',
    role: 'employee',
    branch_id: branch,
    shift_id: shift.id,
    start_date: '2026-01-01',
    active: true,
    reason: 'Invited browser fixture',
  });
  employee = emp.id;
  invitation = (
    await mutate('/devices/invitations', {
      membership_id: employee,
      reason: 'Identity verified for browser test',
    })
  ).url;
  await platform.dispose();
});
test.afterAll(async () => {
  await owner?.dispose();
});
test('signed passkey enrollment and passwordless attendance, duplicate scans and revocation', async ({
  browser,
}) => {
  test.setTimeout(180000);
  const ctx = await browser.newContext({
      viewport: { width: 390, height: 844 },
      geolocation: { latitude: 11.5564, longitude: 104.9282, accuracy: 10 },
      permissions: ['geolocation'],
    }),
    page = await ctx.newPage(),
    virtual = await virtualPasskey(page);
  await page.goto(origin + '/scan/' + point);
  await expect(page.getByRole('button', { name: 'Continue with passkey' })).toBeVisible();
  const empty = await (await owner.get('/api/attendance')).json();
  expect(empty.items).toHaveLength(0);
  // Simulate HR's browser being used to enroll an employee phone.
  await ctx.addCookies((await owner.storageState()).cookies);
  await page.goto(invitation);
  await page.getByRole('button', { name: 'Create attendance passkey' }).click();
  await expect(page.getByRole('heading', { name: 'You’re enrolled' })).toBeVisible({
    timeout: 30000,
  });
  expect((await page.request.get(origin + '/api/auth/me')).status()).toBe(401);
  const credentials = await virtual.session.send('WebAuthn.getCredentials', {
    authenticatorId: virtual.id,
  });
  expect(credentials.credentials).toHaveLength(1);
  const used = await page.request.post(origin + '/api/auth/enroll/begin', {
    headers: { Origin: origin },
    data: { token: invitation.split('#')[1] },
  });
  expect(used.status()).toBe(401);
  // A lost response after commit must replay the exact saved payload on reload.
  await page.route('**/api/attendance', async (route) => {
    const response = await route.fetch();
    expect(response.status()).toBe(201);
    await route.abort();
  });
  await page.goto(origin + '/scan/' + point);
  await page.getByRole('button', { name: 'Continue with passkey' }).click();
  await expect(page.getByRole('alert')).toBeVisible({ timeout: 30000 });
  await page.unroute('**/api/attendance');
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Check In successful' })).toBeVisible({
    timeout: 30000,
  });
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Already recorded' })).toBeVisible({
    timeout: 30000,
  });
  const records = await (await owner.get('/api/attendance')).json();
  expect(records.items).toHaveLength(1);
  const details = await (await owner.get('/api/attendance/' + records.items[0].id)).json();
  expect(details.events).toHaveLength(1);
  await ctx.clearCookies();
  await page.goto(origin + '/scan/' + point);
  await page.getByRole('button', { name: 'Continue with passkey' }).click();
  await expect(page.getByRole('heading', { name: 'Already recorded' })).toBeVisible({
    timeout: 30000,
  });
  const me = await (await page.request.get(origin + '/api/auth/me')).json();
  const outside = await page.request.post(origin + '/api/attendance', {
    headers: { Origin: origin, 'X-CSRF-Token': me.user.csrf },
    data: {
      point,
      action: 'check_out',
      idempotency_key: randomUUID(),
      location: { latitude: 0, longitude: 0, accuracy: 10, measured_at: new Date().toISOString() },
    },
  });
  expect(outside.status()).toBe(422);
  const devices = await (await owner.get('/api/devices')).json(),
    device = devices.items.find((d: any) => d.membership_id === employee);
  await mutate('/devices/' + encodeURIComponent(device.id) + '/revoke', {
    reason: 'Lost phone test',
  });
  expect((await page.request.get(origin + '/api/auth/me')).status()).toBe(401);
  await page.reload();
  await page.getByRole('button', { name: 'Continue with passkey' }).click();
  await expect(page.getByRole('alert')).toContainText('Device enrollment required', {
    timeout: 30000,
  });
  await ctx.close();
});
test('location denial is clear and public signup is absent', async ({ browser }) => {
  test.setTimeout(180000);
  const invite = (
      await mutate('/devices/invitations', {
        membership_id: employee,
        reason: 'New fixture verifies location denial',
      })
    ).url,
    ctx = await browser.newContext({ viewport: { width: 390, height: 844 } }),
    page = await ctx.newPage();
  await virtualPasskey(page);
  await page.addInitScript(() =>
    Object.defineProperty(navigator, 'geolocation', {
      value: {
        watchPosition: (_ok: any, fail: any) => {
          fail({ code: 1 });
          return 1;
        },
        clearWatch: () => {},
      },
    })
  );
  await page.goto(invite);
  await page.getByRole('button', { name: 'Create attendance passkey' }).click();
  await expect(page.getByRole('heading', { name: 'You’re enrolled' })).toBeVisible({
    timeout: 30000,
  });
  await page.goto(origin + '/scan/' + point);
  await page.getByRole('button', { name: 'Continue with passkey' }).click();
  await expect(page.getByRole('alert')).toContainText('Location permission denied', {
    timeout: 30000,
  });
  await ctx.clearCookies();
  await page.goto(origin + '/admin-login');
  await expect(page.getByRole('heading', { name: 'Administrative login' })).toBeVisible();
  await expect(page.getByRole('button', { name: /Create workspace|Create a company/ })).toHaveCount(
    0
  );
  expect(
    (
      await page.request.post(origin + '/api/auth/register', {
        headers: { Origin: origin },
        data: {},
      })
    ).status()
  ).toBe(404);
  await ctx.close();
});
