import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { Moon, Sun } from 'lucide-react';
import { Button } from './ui';

type Theme = 'light' | 'dark';
function savedTheme(): Theme | null {
  try {
    const saved = localStorage.getItem('theme');
    return saved === 'light' || saved === 'dark' ? saved : null;
  } catch {
    return null;
  }
}
export function initialTheme(): Theme {
  return (
    savedTheme() ?? (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
  );
}
export function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
}
const ThemeContext = createContext({ theme: 'light' as Theme, toggle: () => {} });
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState(initialTheme);
  const [preference, setPreference] = useState<Theme | null>(savedTheme);
  useEffect(() => applyTheme(theme), [theme]);
  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const change = () => {
      if (!preference) setTheme(media.matches ? 'dark' : 'light');
    };
    const sync = (event: StorageEvent) => {
      if (event.key === 'theme' || event.key === null) {
        setPreference(savedTheme());
        setTheme(initialTheme());
      }
    };
    media.addEventListener('change', change);
    window.addEventListener('storage', sync);
    return () => {
      media.removeEventListener('change', change);
      window.removeEventListener('storage', sync);
    };
  }, [preference]);
  function toggle() {
    const next = theme === 'dark' ? 'light' : 'dark';
    setTheme(next);
    setPreference(next);
    try {
      localStorage.setItem('theme', next);
    } catch {
      /* Keep working without storage. */
    }
  }
  return <ThemeContext.Provider value={{ theme, toggle }}>{children}</ThemeContext.Provider>;
}
export function ThemeToggle() {
  const { theme, toggle } = useContext(ThemeContext);
  return (
    <Button
      type="button"
      variant="ghost"
      className="theme-toggle"
      aria-label="Toggle color theme"
      title={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
      aria-pressed={theme === 'dark'}
      onClick={toggle}
    >
      {theme === 'dark' ? (
        <Sun size={19} aria-hidden="true" />
      ) : (
        <Moon size={19} aria-hidden="true" />
      )}
    </Button>
  );
}
