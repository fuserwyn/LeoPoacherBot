/**
 * Аналитика leo_comment_displayed (analytics_BT_v1 §4): когда пользователь увидел
 * комментарий Лео в треде ленты. Шлёт beacon на сервер один раз на каждый reply
 * за сессию. Best-effort, на работу аппы не влияет. Дедуп серверный — по (user, reply).
 */
const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "").trim() ?? "";

const reported = new Set<number>();

export function reportLeoCommentDisplayed(initData: string, threadReplyId: number): void {
  if (!apiBase || !initData || !threadReplyId || threadReplyId <= 0) return;
  if (reported.has(threadReplyId)) return;
  reported.add(threadReplyId);
  try {
    void fetch(`${apiBase}/api/miniapp/analytics/leo-comment-displayed`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData, thread_reply_id: threadReplyId }),
      keepalive: true,
    }).catch(() => {});
  } catch {
    /* диагностика не должна влиять на аппу */
  }
}

/** Только для тестов. */
export function __resetLeoCommentDiagForTests(): void {
  reported.clear();
}
