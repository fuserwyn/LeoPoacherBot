/** Тема оформления. По умолчанию — тёмная (как до редизайна); светлая — опция пользователя.
 *  Выбор хранится в localStorage и применяется атрибутом data-theme на <html>.
 *  Раннее применение (без мигания) — инлайн-скриптом в index.html; здесь — рантайм-переключение. */
export type ThemeMode = "light" | "dark" | "leopard";

/** Розовая леопардовая тема — с 5 уровня (Лев). */
export const LEOPARD_THEME_MIN_LEVEL = 5;

const STORAGE_KEY = "leo-theme";
const THEME_COLOR: Record<ThemeMode, string> = {
  light: "#f5f1f5",
  dark: "#0d0d12",
  leopard: "#f6d4de",
};

export function canUseLeopardTheme(level: number): boolean {
  return level >= LEOPARD_THEME_MIN_LEVEL;
}

export function themeAllowedForLevel(mode: ThemeMode, level: number): ThemeMode {
  if (mode === "leopard" && !canUseLeopardTheme(level)) {
    return "dark";
  }
  return mode;
}

export function enforceThemeForLevel(level: number): ThemeMode {
  const stored = getStoredTheme();
  const next = themeAllowedForLevel(stored, level);
  if (next !== stored) {
    setTheme(next);
  }
  return next;
}

export function parseTheme(raw: string | null | undefined): ThemeMode {
  if (raw === "light" || raw === "leopard") {
    return raw;
  }
  return "dark";
}

export function getStoredTheme(): ThemeMode {
  try {
    return parseTheme(localStorage.getItem(STORAGE_KEY));
  } catch {
    return "dark";
  }
}

export function applyTheme(mode: ThemeMode): void {
  document.documentElement.setAttribute("data-theme", mode);
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) meta.setAttribute("content", THEME_COLOR[mode]);
}

export function setTheme(mode: ThemeMode): void {
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    /* приватный режим — просто применим без сохранения */
  }
  applyTheme(mode);
}
