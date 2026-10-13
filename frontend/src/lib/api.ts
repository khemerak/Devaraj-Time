let csrf = '';
export function setCSRF(value: string) {
  csrf = value;
}
export class APIError extends Error {
  constructor(
    message: string,
    public status: number
  ) {
    super(message);
  }
}
export async function api<T = any>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    credentials: 'same-origin',
    ...init,
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf, ...init?.headers },
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new APIError(body.error || 'Unable to connect to Devaraj Time', res.status);
  return body;
}
export function write(path: string, body: unknown, method = 'POST') {
  return api(path, { method, body: JSON.stringify(body) });
}
export function label(text: string) {
  return text.replaceAll('_', ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}
export function time(value: string | null, timezone: string) {
  return value
    ? new Intl.DateTimeFormat('en', {
        hour: '2-digit',
        minute: '2-digit',
        timeZone: timezone,
      }).format(new Date(value))
    : '—';
}
export function hours(minutes: number) {
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
}
export function pointIdentifier(value: string): string {
  const trimmed = value.trim();
  if (/^[A-Za-z0-9_-]{43}$/.test(trimmed)) return trimmed;
  const url = new URL(trimmed);
  if (
    !['http:', 'https:'].includes(url.protocol) ||
    !/^\/scan\/[A-Za-z0-9_-]{43}$/.test(url.pathname)
  )
    throw new Error('Use a valid Devaraj Time attendance QR code.');
  return url.pathname.split('/').pop()!;
}
