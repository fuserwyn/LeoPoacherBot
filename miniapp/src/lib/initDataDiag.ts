/**
 * Прод-диагностика источника initData. Один раз за загрузку аппы шлёт на сервер
 * лёгкий beacon: откуда удалось взять initData (webapp/hash/session/none), на какой
 * платформе и версии Telegram. Сервер это логирует — так мы по реальным данным
 * видим, почему у части юзеров initData оказывается пустым, и подтверждаем причину
 * фактами, а не гипотезой. Best-effort: любые ошибки глотаются, на работу аппы не влияет.
 */
import type { InitDataSource } from "./telegramInitData";

function apiBase(): string {
  return (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "").trim() ?? "";
}

export type InitDataDiagPayload = {
  source: InitDataSource;
  /** Подписанный initData (может быть пустым) — сервер проверит подпись и залогирует валидность/uid. */
  initData: string;
  platform?: string;
  tgVersion?: string;
  /** Сколько мс ретраев заняло появление initData (0 — сразу или так и не появился). */
  triesMs?: number;
};

let reported = false;

/** Шлёт один beacon за сессию. Повторные вызовы игнорируются. */
export function reportInitDataSource(p: InitDataDiagPayload): void {
  if (reported) return;
  const base = apiBase();
  if (!base) return; // без API URL отправлять некуда (например, в браузере вне прод-сборки)
  reported = true;
  const body = JSON.stringify({
    source: p.source,
    has_init: Boolean(p.initData),
    init_data: p.initData,
    platform: p.platform ?? "",
    tg_version: p.tgVersion ?? "",
    tries_ms: p.triesMs ?? 0,
  });
  try {
    // keepalive — чтобы beacon ушёл, даже если апку тут же закрыли.
    void fetch(`${base}/api/miniapp/diag/init-source`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
      keepalive: true,
    }).catch(() => {});
  } catch {
    /* диагностика не должна влиять на аппу */
  }
}

/** Только для тестов: сбросить «отправлено один раз». */
export function __resetInitDataDiagForTests(): void {
  reported = false;
}
