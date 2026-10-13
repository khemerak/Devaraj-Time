import { describe, it, expect, vi, afterEach } from 'vitest';
import { api, setCSRF, pointIdentifier, hours } from './api';
describe('API attendance contract', () => {
  afterEach(() => vi.unstubAllGlobals());
  it('includes CSRF on a real mutation and propagates a server rejection', async () => {
    setCSRF('csrf-test');
    const fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 422,
      json: async () => ({ error: 'Outside boundary' }),
    });
    vi.stubGlobal('fetch', fetch);
    await expect(api('/attendance', { method: 'POST', body: '{}' })).rejects.toThrow(
      'Outside boundary'
    );
    expect(fetch.mock.calls[0][1].headers['X-CSRF-Token']).toBe('csrf-test');
  });
  it('never turns a failed response into a success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Offline')));
    await expect(api('/attendance')).rejects.toThrow('Offline');
  });
  it('accepts persistent printed attendance URLs across deployment hostnames', () => {
    vi.stubGlobal('window', { location: { origin: 'https://attendance.example' } });
    const id = 'a'.repeat(43);
    expect(pointIdentifier('https://attendance.example/scan/' + id)).toBe(id);
    expect(pointIdentifier('http://localhost:5173/scan/' + id)).toBe(id);
    expect(() => pointIdentifier('javascript:/scan/' + id)).toThrow();
    expect(() => pointIdentifier('https://attendance.example/other/' + id)).toThrow();
    expect(pointIdentifier(id)).toBe(id);
  });
  it('displays recorded durations in clear units', () => {
    expect(hours(485)).toBe('8h 5m');
  });
});
