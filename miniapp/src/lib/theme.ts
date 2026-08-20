/** Тема оформления. По умолчанию — тёмная (как до редизайна); светлая — опция пользователя.
 *  Источник правды — профиль на сервере. localStorage и Telegram CloudStorage — кэш:
 *  WebView после закрытия Mini App часто их теряет. */
export type ThemeMode = "light" | "dark" | "leopard";

/** Розовая леопардовая тема — с 5 уровня (Лев). */
export const LEOPARD_THEME_MIN_LEVEL = 5;

const STORAGE_KEY = "leo-theme";
const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

/** Сервер уже отдал тему — CloudStorage не должен затереть её устаревшим значением. */
let themeLockedFromServer = false;

export function resetThemeRuntimeForTests(): void {
  themeLockedFromServer = false;
}
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

export function hasStoredTheme(): boolean {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw === "light" || raw === "dark" || raw === "leopard";
  } catch {
    return false;
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

export async function persistThemeToServer(initData: string, mode: ThemeMode): Promise<void> {
  if (!apiBase || !initData.trim()) return;
  try {
    await fetch(`${apiBase}/api/miniapp/profile/save`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData, theme: mode }),
    });
  } catch {
    /* тема останется в localStorage / CloudStorage */
  }
}

/** Тема из профиля. Пустой ответ не затирает кэш и не пишет dark на сервер. */
export function hydrateThemeFromServer(raw: unknown, level: number): ThemeMode {
  if (raw === "light" || raw === "dark" || raw === "leopard") {
    themeLockedFromServer = true;
    const next = themeAllowedForLevel(raw, level);
    setTheme(next);
    return next;
  }
  return enforceThemeForLevel(level);
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
      if (themeLockedFromServer) {
        onDone?.(getStoredTheme());
        return;
      }
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
