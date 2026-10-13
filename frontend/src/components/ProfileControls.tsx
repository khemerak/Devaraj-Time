import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Button, Modal, ErrorMessage } from './ui';
import { write } from '../lib/api';
export default function ProfileControls({ name }: { name: string }) {
  const [mode, setMode] = useState(''),
    [error, setError] = useState<unknown>(null),
    [busy, setBusy] = useState(false);
  const client = useQueryClient();
  return (
    <>
      <div className="qr-downloads">
        <Button
          variant="outline"
          onClick={() => {
            setMode('name');
            setError(null);
          }}
        >
          Edit display name
        </Button>
        <Button
          variant="outline"
          onClick={() => {
            setMode('recovery');
            setError(null);
          }}
        >
          Verify recovery email
        </Button>
        <Button
          variant="outline"
          onClick={() => {
            setMode('password');
            setError(null);
          }}
        >
          Change password
        </Button>
      </div>
      <Modal
        open={!!mode}
        onOpenChange={(v) => !v && setMode('')}
        title={
          mode === 'name'
            ? 'Your display name'
            : mode === 'recovery'
              ? 'Verify recovery email'
              : 'Change your password'
        }
        description={
          mode === 'password'
            ? 'All existing sessions will be revoked. You will sign in again with your new password.'
            : 'This updates your account name in every company you belong to.'
        }
      >
        <form
          className="record-form"
          onSubmit={async (e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            setBusy(true);
            setError(null);
            try {
              if (mode === 'name') {
                await write('/auth/profile', { name: f.get('name') }, 'PATCH');
                setMode('');
                await client.invalidateQueries();
              } else if (mode === 'recovery') {
                const result = await write('/auth/recovery-email', {
                  email: f.get('email'),
                  password: f.get('current_password'),
                });
                setError(new Error(result.message));
              } else {
                if (f.get('new_password') !== f.get('confirm_password'))
                  throw new Error('The new passwords do not match.');
                await write('/auth/password', {
                  current_password: f.get('current_password'),
                  new_password: f.get('new_password'),
                });
                client.clear();
                location.assign('/');
              }
            } catch (err) {
              setError(err);
            } finally {
              setBusy(false);
            }
          }}
        >
          {mode === 'name' ? (
            <label>
              Display name
              <input
                name="name"
                defaultValue={name}
                minLength={2}
                maxLength={120}
                required
                autoComplete="name"
              />
            </label>
          ) : mode === 'recovery' ? (
            <>
              <label>
                Recovery email
                <input name="email" type="email" required />
              </label>
              <label>
                Current password
                <input
                  name="current_password"
                  type="password"
                  required
                  autoComplete="current-password"
                />
              </label>
              <p>
                Check the verification link. Password recovery can use this destination only after
                verification.
              </p>
            </>
          ) : (
            <div className="form-grid">
              <label className="full">
                Current password
                <input
                  type="password"
                  name="current_password"
                  required
                  autoComplete="current-password"
                  maxLength={128}
                />
              </label>
              <label className="full">
                New password
                <input
                  type="password"
                  name="new_password"
                  required
                  minLength={12}
                  maxLength={128}
                  autoComplete="new-password"
                />
              </label>
              <label className="full">
                Confirm new password
                <input
                  type="password"
                  name="confirm_password"
                  required
                  minLength={12}
                  maxLength={128}
                  autoComplete="new-password"
                />
              </label>
            </div>
          )}
          <ErrorMessage error={error} />
          <div className="form-actions">
            <Button disabled={busy}>{busy ? 'Saving…' : 'Save changes'}</Button>
          </div>
        </form>
      </Modal>
    </>
  );
}
