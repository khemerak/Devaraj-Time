import * as Dialog from '@radix-ui/react-dialog';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { X, LoaderCircle, Inbox } from 'lucide-react';
import type { ButtonHTMLAttributes, ReactNode } from 'react';
const variants = cva('button', {
  variants: {
    variant: { default: 'primary', outline: 'outline', ghost: 'ghost', danger: 'danger' },
  },
  defaultVariants: { variant: 'default' },
});
export function Button({
  className,
  variant,
  asChild,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof variants> & { asChild?: boolean }) {
  const Component = asChild ? Slot : 'button';
  return <Component className={twMerge(clsx(variants({ variant }), className))} {...props} />;
}
export function Modal({
  title,
  description,
  children,
  open,
  onOpenChange,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="overlay" />
        <Dialog.Content className="modal">
          <div className="modal-head">
            <div>
              <Dialog.Title>{title}</Dialog.Title>
              <Dialog.Description>
                {description || 'Changes are saved securely to your company.'}
              </Dialog.Description>
            </div>
            <Dialog.Close asChild>
              <Button variant="ghost" aria-label="Close">
                <X size={20} />
              </Button>
            </Dialog.Close>
          </div>
          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
export function Loading() {
  return (
    <div className="loading" role="status">
      <LoaderCircle className="spin" /> Loading your workspace…
      <div className="skeleton" />
      <div className="skeleton" />
    </div>
  );
}
export function Empty({
  title = 'Nothing here yet',
  children,
}: {
  title?: string;
  children?: ReactNode;
}) {
  return (
    <div className="empty">
      <span>
        <Inbox size={28} />
      </span>
      <h3>{title}</h3>
      <p>{children || 'Records will appear here when your team starts using Devaraj Time.'}</p>
    </div>
  );
}
export function ErrorMessage({ error }: { error: unknown }) {
  return error ? (
    <div className="error" role="alert">
      {error instanceof Error ? error.message : String(error)}
    </div>
  ) : null;
}
export function Badge({ value }: { value: string }) {
  return (
    <span
      className={`badge ${['late', 'rejected'].includes(value) ? 'amber' : ['approved', 'on_time', 'checked_in', 'verification_passed'].includes(value) ? 'green' : 'blue'}`}
    >
      {value.replaceAll('_', ' ')}
    </span>
  );
}
