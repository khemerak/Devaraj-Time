export type AttendanceLocation = {
  latitude: number;
  longitude: number;
  accuracy: number;
  measured_at: string;
};
export type LocationProgress = {
  accuracy: number | null;
  elapsedSeconds: number;
  maximumAccuracy: number;
};
export class LocationError extends Error {
  constructor(
    public code: 'denied' | 'accuracy' | 'unavailable',
    message: string
  ) {
    super(message);
  }
}
export function locateForAttendance({
  maximumAccuracy = 75,
  timeoutMs = 45000,
  onProgress,
  signal,
}: {
  maximumAccuracy?: number;
  timeoutMs?: number;
  onProgress?: (progress: LocationProgress) => void;
  signal?: AbortSignal;
} = {}): Promise<AttendanceLocation> {
  return new Promise((resolve, reject) => {
    const geo = navigator.geolocation;
    if (!geo || typeof geo.watchPosition !== 'function') {
      reject(
        new LocationError(
          'unavailable',
          'Location is unavailable in this browser. Open this HTTPS link in Safari or Chrome and allow location access.'
        )
      );
      return;
    }
    let done = false,
      watch: number | undefined,
      latestAccuracy: number | null = null,
      requesting = false;
    const start = Date.now();
    const progress = () =>
      onProgress?.({
        accuracy: latestAccuracy,
        elapsedSeconds: Math.floor((Date.now() - start) / 1000),
        maximumAccuracy,
      });
    const cleanup = () => {
      clearTimeout(deadline);
      clearInterval(ticker);
      if (watch !== undefined) geo.clearWatch(watch);
      signal?.removeEventListener('abort', abort);
    };
    const finish = (value?: AttendanceLocation, error?: Error) => {
      if (done) return;
      done = true;
      cleanup();
      if (error) reject(error);
      else resolve(value!);
    };
    const abort = () =>
      finish(undefined, new DOMException('Location acquisition cancelled', 'AbortError'));
    const accept = (p: GeolocationPosition) => {
      if (done) return;
      requesting = false;
      const { latitude, longitude, accuracy } = p.coords;
      if (
        ![latitude, longitude, accuracy, p.timestamp].every(Number.isFinite) ||
        Math.abs(latitude) > 90 ||
        Math.abs(longitude) > 180 ||
        accuracy <= 0
      )
        return;
      // Preserve the provider's timestamp. A cached fix is never relabeled as fresh.
      if (Date.now() - p.timestamp > 15000 || p.timestamp > Date.now() + 10000) return;
      latestAccuracy = accuracy;
      progress();
      if (accuracy <= maximumAccuracy)
        finish({ latitude, longitude, accuracy, measured_at: new Date(p.timestamp).toISOString() });
    };
    const failed = (e: GeolocationPositionError) => {
      requesting = false;
      if (done) return;
      if (e.code === 1)
        finish(
          undefined,
          new LocationError(
            'denied',
            'Location permission denied. Allow location for this website and enable Precise Location for the browser, then retry.'
          )
        );
      // Temporary timeouts/unavailable fixes do not discard the ongoing watch.
    };
    const options = { enableHighAccuracy: true, maximumAge: 0, timeout: 10000 };
    const deadline = setTimeout(
      () =>
        finish(
          undefined,
          new LocationError(
            latestAccuracy === null ? 'unavailable' : 'accuracy',
            latestAccuracy === null
              ? 'A fresh location could not be obtained. Keep this page open, check browser location permissions, and retry near a window or outdoors.'
              : `Location remains too approximate (±${Math.ceil(latestAccuracy)} m; workplace requires ${maximumAccuracy} m or better). Enable Precise Location for the browser and retry near a window or outdoors.`
          )
        ),
      timeoutMs
    );
    const ticker = setInterval(() => {
      if (done) return;
      progress();
      // Some stationary devices emit no further watch updates after a coarse first fix.
      if (!requesting && typeof geo.getCurrentPosition === 'function') {
        requesting = true;
        geo.getCurrentPosition(accept, failed, options);
      }
    }, 4000);
    signal?.addEventListener('abort', abort, { once: true });
    if (signal?.aborted) {
      abort();
      return;
    }
    progress();
    try {
      watch = geo.watchPosition(accept, failed, options);
      if (done) geo.clearWatch(watch);
    } catch {
      finish(
        undefined,
        new LocationError(
          'unavailable',
          'Location could not start. Open the HTTPS link in a supported browser and allow location access.'
        )
      );
    }
  });
}
