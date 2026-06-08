/**
 * Клиентские аналитические события воронок (analytics_BT_v1 §3/§4), которые сервер
 * сам не видит: miniapp_opened (юзер открыл мини-апп) и workout_log_started (открыл
 * форму логирования). Имя строго из белого списка на сервере. Best-effort.
 */
const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "").trim() ?? "";

export type MiniappAnalyticsEvent = "miniapp_opened" | "workout_log_started";

function send(initData: string, event: MiniappAnalyticsEvent, entryPoint?: string): void {
  if (!apiBase || !initData) return;
  try {
    void fetch(`${apiBase}/api/miniapp/analytics/event`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData, event, entry_point: entryPoint ?? "" }),
      keepalive: true,
    }).catch(() => {});
  } catch {
    /* аналитика не должна влиять на аппу */
  }
}

let openedReported = false;

/** miniapp_opened — один раз за загрузку аппы. */
export function reportMiniappOpened(initData: string, entryPoint = "miniapp"): void {
  if (openedReported) return;
  openedReported = true;
  send(initData, "miniapp_opened", entryPoint);
}

/** workout_log_started — каждый раз при открытии формы тренировки. */
export function reportWorkoutLogStarted(initData: string): void {
  send(initData, "workout_log_started");
}

/** Только для тестов. */
export function __resetMiniappEventsForTests(): void {
  openedReported = false;
}
