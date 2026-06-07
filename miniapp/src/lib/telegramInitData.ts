/**
 * Надёжное получение Telegram initData — подписанной строки запуска, которую
 * бэкенд проверяет по hash. Полагаться только на `window.Telegram.WebApp.initData`
 * нельзя: на части клиентов (особенно iOS) он какое-то время пустой, а после
 * перезагрузки/перерисовки SPA вообще теряется. Тогда все запросы к API навсегда
 * упираются в «нужен initData». Поэтому собираем строку из трёх источников по
 * приоритету и кэшируем в sessionStorage (как это делает @telegram-apps/sdk).
 *
 *  1. window.Telegram.WebApp.initData — если уже заполнен.
 *  2. tgWebAppData из location.hash / location.search — параметры самого запуска.
 *  3. sessionStorage — то, что нашли при первом успешном запуске в этой сессии.
 */
export const INIT_DATA_STORAGE_KEY = "__leo_tg_init_data";

/** Достаёт значение tgWebAppData из «#a=b&...» или «?a=b&...» строки. */
function fromParamString(raw: string): string {
  if (!raw) return "";
  const cleaned = raw.replace(/^[#?]/, "");
  if (!cleaned) return "";
  try {
    const params = new URLSearchParams(cleaned);
    // URLSearchParams декодирует %-кодирование один раз — ровно как WebApp.initData.
    return params.get("tgWebAppData") ?? "";
  } catch {
    return "";
  }
}

function readStored(): string {
  try {
    return window.sessionStorage.getItem(INIT_DATA_STORAGE_KEY) ?? "";
  } catch {
    return "";
  }
}

function persist(value: string): void {
  if (!value) return;
  try {
    window.sessionStorage.setItem(INIT_DATA_STORAGE_KEY, value);
  } catch {
    /* приватный режим / переполнение — кэш не критичен */
  }
}

/** Откуда удалось взять initData — для прод-диагностики причины пустого initData. */
export type InitDataSource = "webapp" | "hash" | "search" | "session" | "none";

/**
 * Как resolveInitData, но дополнительно сообщает источник. Нужно, чтобы в проде
 * увидеть распределение причин: сколько юзеров получили initData напрямую из
 * WebApp, сколько спас хэш/sessionStorage, а у скольких его нет вовсе ("none").
 */
export function resolveInitDataWithSource(): { value: string; source: InitDataSource } {
  const live = window.Telegram?.WebApp?.initData ?? "";
  if (live) {
    persist(live);
    return { value: live, source: "webapp" };
  }

  const fromHash = fromParamString(window.location.hash);
  if (fromHash) {
    persist(fromHash);
    return { value: fromHash, source: "hash" };
  }

  const fromSearch = fromParamString(window.location.search);
  if (fromSearch) {
    persist(fromSearch);
    return { value: fromSearch, source: "search" };
  }

  const stored = readStored();
  if (stored) {
    return { value: stored, source: "session" };
  }

  return { value: "", source: "none" };
}

/**
 * Возвращает подписанный initData или "" если взять неоткуда.
 * Любую непустую находку кэширует в sessionStorage, чтобы пережить перезагрузку.
 */
export function resolveInitData(): string {
  return resolveInitDataWithSource().value;
}
