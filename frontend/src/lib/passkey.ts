import { write } from './api';
export { locateForAttendance } from './location';
const decode = (s: string) =>
  Uint8Array.from(atob(s.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0));
const encode = (v: ArrayBuffer | null) =>
  v
    ? btoa(String.fromCharCode(...new Uint8Array(v)))
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '')
    : null;
export async function passkey(enrollmentToken?: string) {
  if (!window.isSecureContext || !navigator.credentials)
    throw new Error('Open the HTTPS attendance link to use your passkey.');
  const mode = enrollmentToken ? 'enroll' : 'passkey';
  const start = await write(
    `/auth/${mode}/begin`,
    enrollmentToken ? { token: enrollmentToken } : {}
  );
  const options = start.options.publicKey;
  options.challenge = decode(options.challenge);
  if (options.user) options.user.id = decode(options.user.id);
  for (const key of ['allowCredentials', 'excludeCredentials'])
    if (options[key]) options[key] = options[key].map((c: any) => ({ ...c, id: decode(c.id) }));
  let credential: PublicKeyCredential | null;
  try {
    credential = (await (enrollmentToken
      ? navigator.credentials.create({ publicKey: options })
      : navigator.credentials.get({ publicKey: options }))) as PublicKeyCredential | null;
  } catch (error) {
    if (error instanceof DOMException && error.name === 'NotAllowedError')
      throw new Error(
        enrollmentToken
          ? 'Passkey enrollment was cancelled or unavailable. Open the invitation on a supported phone browser and try again.'
          : 'Device enrollment required, or passkey authentication was cancelled. Ask HR for an invitation if this phone is not enrolled.'
      );
    throw error;
  }
  if (!credential) throw new Error('Passkey authentication was cancelled.');
  const r = credential.response as any;
  const response = {
    id: credential.id,
    rawId: encode(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    clientExtensionResults: credential.getClientExtensionResults(),
    response: {
      clientDataJSON: encode(r.clientDataJSON),
      ...(enrollmentToken
        ? { attestationObject: encode(r.attestationObject), transports: r.getTransports?.() || [] }
        : {
            authenticatorData: encode(r.authenticatorData),
            signature: encode(r.signature),
            userHandle: encode(r.userHandle),
          }),
    },
  };
  await write(`/auth/${mode}/finish`, { ceremony: start.ceremony, response });
}
