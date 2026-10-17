// @vitest-environment jsdom
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { ThemeProvider, ThemeToggle } from './Theme';
let systemChange: (() => void) | undefined;
let darkSystem = true;
beforeEach(() => {
  localStorage.clear();
  darkSystem = true;
  vi.stubGlobal(
    'matchMedia',
    vi.fn(() => ({
      get matches() {
        return darkSystem;
      },
      addEventListener: (_: string, fn: () => void) => {
        systemChange = fn;
      },
      removeEventListener: vi.fn(),
    }))
  );
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
it('uses the system theme until an explicit choice', () => {
  render(
    <ThemeProvider>
      <ThemeToggle />
    </ThemeProvider>
  );
  expect(document.documentElement.dataset.theme).toBe('dark');
  act(() => {
    darkSystem = false;
    systemChange?.();
  });
  expect(document.documentElement.dataset.theme).toBe('light');
  fireEvent.click(screen.getByRole('button', { name: 'Toggle color theme' }));
  expect(localStorage.getItem('theme')).toBe('dark');
  act(() => systemChange?.());
  expect(document.documentElement.dataset.theme).toBe('dark');
});
it('preserves saved preferences and synchronizes controls', () => {
  localStorage.setItem('theme', 'light');
  render(
    <ThemeProvider>
      <ThemeToggle />
      <ThemeToggle />
    </ThemeProvider>
  );
  expect(document.documentElement.dataset.theme).toBe('light');
  fireEvent.click(screen.getAllByRole('button')[0]);
  screen
    .getAllByRole('button')
    .forEach((button) => expect(button.getAttribute('aria-pressed')).toBe('true'));
  expect(document.documentElement.style.colorScheme).toBe('dark');
});
it('synchronizes changes from another tab', () => {
  render(
    <ThemeProvider>
      <ThemeToggle />
    </ThemeProvider>
  );
  act(() => {
    localStorage.setItem('theme', 'light');
    window.dispatchEvent(new StorageEvent('storage', { key: 'theme', newValue: 'light' }));
  });
  expect(document.documentElement.dataset.theme).toBe('light');
});
it('works when storage is blocked', () => {
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
    throw new Error('Blocked');
  });
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new Error('Blocked');
  });
  render(
    <ThemeProvider>
      <ThemeToggle />
    </ThemeProvider>
  );
  fireEvent.click(screen.getByRole('button', { name: 'Toggle color theme' }));
  expect(document.documentElement.dataset.theme).toBe('light');
});
