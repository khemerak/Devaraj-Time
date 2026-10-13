import { afterEach, describe, it, expect, vi } from 'vitest';
import { locateForAttendance } from './location';
describe('location consent', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });
  it('does not manufacture a location when permission is denied', async () => {
    vi.stubGlobal('navigator', {
      geolocation: {
        watchPosition: (_ok: any, deny: any) => {
          deny({ code: 1 });
          return 7;
        },
        clearWatch: vi.fn(),
      },
    });
    await expect(locateForAttendance()).rejects.toThrow('Location permission denied');
  });
  it('requests fresh high accuracy and preserves the measurement', async () => {
    const get = vi.fn((ok: any, _deny: any, _options: any) => {
      ok({ coords: { latitude: 11.5, longitude: 104.9, accuracy: 14 }, timestamp: Date.now() });
      return 7;
    });
    const clear = vi.fn();
    vi.stubGlobal('navigator', { geolocation: { watchPosition: get, clearWatch: clear } });
    const location = await locateForAttendance();
    expect(location.latitude).toBe(11.5);
    expect(location.accuracy).toBe(14);
    expect(get.mock.calls[0][2]).toEqual({
      enableHighAccuracy: true,
      maximumAge: 0,
      timeout: 10000,
    });
    expect(clear).toHaveBeenCalledWith(7);
  });
  it('waits through a coarse first fix and uses a fresh precise fix', async () => {
    vi.useFakeTimers();
    let update: (p: any) => void = () => {};
    const clear = vi.fn();
    vi.stubGlobal('navigator', {
      geolocation: {
        watchPosition: (success: any) => {
          update = success;
          return 5;
        },
        clearWatch: clear,
      },
    });
    const progress = vi.fn();
    const promise = locateForAttendance({ maximumAccuracy: 50, onProgress: progress });
    let settled = false;
    void promise.then(() => {
      settled = true;
    });
    update({ coords: { latitude: 11.5, longitude: 104.9, accuracy: 700 }, timestamp: Date.now() });
    await Promise.resolve();
    expect(settled).toBe(false);
    update({ coords: { latitude: 11.51, longitude: 104.91, accuracy: 20 }, timestamp: Date.now() });
    expect((await promise).accuracy).toBe(20);
    expect(clear).toHaveBeenCalledWith(5);
    expect(progress.mock.calls.some(([p]) => p.accuracy === 700)).toBe(true);
  });
  it('rejects persistently approximate readings instead of loosening policy', async () => {
    vi.useFakeTimers();
    const clear = vi.fn();
    vi.stubGlobal('navigator', {
      geolocation: {
        watchPosition: (ok: any) => {
          ok({
            coords: { latitude: 11.5, longitude: 104.9, accuracy: 600 },
            timestamp: Date.now(),
          });
          return 3;
        },
        clearWatch: clear,
      },
    });
    const promise = locateForAttendance({ maximumAccuracy: 75, timeoutMs: 45000 });
    const rejection = expect(promise).rejects.toThrow('±600 m; workplace requires 75 m');
    await vi.advanceTimersByTimeAsync(45000);
    await rejection;
    expect(clear).toHaveBeenCalledWith(3);
  });
  it('discards stale precise fixes and never replaces their timestamps', async () => {
    vi.useFakeTimers();
    vi.stubGlobal('navigator', {
      geolocation: {
        watchPosition: (ok: any) => {
          ok({
            coords: { latitude: 11.5, longitude: 104.9, accuracy: 5 },
            timestamp: Date.now() - 60000,
          });
          return 1;
        },
        clearWatch: vi.fn(),
      },
    });
    const promise = locateForAttendance({ timeoutMs: 1000 });
    const rejection = expect(promise).rejects.toThrow('fresh location could not be obtained');
    await vi.advanceTimersByTimeAsync(1000);
    await rejection;
  });
  it('stops location acquisition when the screen is left', async () => {
    const controller = new AbortController(),
      clear = vi.fn();
    vi.stubGlobal('navigator', { geolocation: { watchPosition: () => 9, clearWatch: clear } });
    const promise = locateForAttendance({ signal: controller.signal });
    const rejection = expect(promise).rejects.toThrow('cancelled');
    controller.abort();
    await rejection;
    expect(clear).toHaveBeenCalledWith(9);
  });
});
