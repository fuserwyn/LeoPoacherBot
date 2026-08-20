/** Тема оформления. По умолчанию — тёмная (как до редизайна); светлая — опция пользователя.
 *  Выбор хранится в localStorage и Telegram CloudStorage, на <html> — data-theme.
 *  Раннее применение (без мигания) — инлайн-скриптом в index.html; здесь — рантайм. */
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
  if (raw === "light" || raw === "leopard" || raw === "dark") {
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

function cloudStorage() {
  return window.Telegram?.WebApp?.CloudStorage;
}

export function persistTheme(mode: ThemeMode): void {
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    /* приватный режим */
  }
  try {
    cloudStorage()?.setItem(STORAGE_KEY, mode);
  } catch {
    /* старый клиент без CloudStorage */
  }
}

export function applyTheme(mode: ThemeMode): void {
  document.documentElement.setAttribute("data-theme", mode);
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) meta.setAttribute("content", THEME_COLOR[mode]);
  window.dispatchEvent(new CustomEvent("leo-theme", { detail: mode }));
}

export function setTheme(mode: ThemeMode): void {
  persistTheme(mode);
  applyTheme(mode);
}

/** Достаёт тему из CloudStorage Telegram — она живёт после закрытия Mini App. */
export function hydrateThemeFromCloud(onDone?: (mode: ThemeMode) => void): void {
  const cs = cloudStorage();
  if (!cs?.getItem) {
    onDone?.(getStoredTheme());
    return;
  }
  try {
    cs.getItem(STORAGE_KEY, (err, value) => {
      const cloud = parseTheme(typeof value === "string" ? value : null);
      const localRaw = (() => {
        try {
          return localStorage.getItem(STORAGE_KEY);
        } catch {
          return null;
        }
      })();
      if (!err && (value === "light" || value === "dark" || value === "leopard")) {
        persistTheme(cloud);
        applyTheme(cloud);
        onDone?.(cloud);
        return;
      }
      if (localRaw) {
        persistTheme(parseTheme(localRaw));
      }
      onDone?.(getStoredTheme());
    });
  } catch {
    onDone?.(getStoredTheme());
  }
}
