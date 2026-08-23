/** Непрочитанные комментарии к твоим тренировкам в ленте стаи (сервер: miniapp_training_thread_unread). */

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type FeedThreadUnreadSummary = {
  count: number;
  userMessageIds: number[];
};

export async function fetchFeedThreadUnreadSummary(initData: string): Promise<FeedThreadUnreadSummary> {
  if (!apiBase || !initData.trim()) return { count: 0, userMessageIds: [] };
  try {
    const res = await fetch(`${apiBase}/api/miniapp/feed/training/thread/unread-count`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData }),
    });
    const j = (await res.json().catch(() => ({}))) as {
      count?: number;
      user_message_ids?: number[];
      ok?: boolean;
    };
    if (!res.ok || !j.ok) return { count: 0, userMessageIds: [] };
    const count = typeof j.count === "number" && j.count > 0 ? j.count : 0;
    const userMessageIds = Array.isArray(j.user_message_ids)
      ? j.user_message_ids.filter((id): id is number => typeof id === "number" && id > 0)
      : [];
    return { count, userMessageIds };
  } catch {
    return { count: 0, userMessageIds: [] };
  }
}

/** @deprecated use fetchFeedThreadUnreadSummary */
export async function fetchFeedThreadUnreadCount(initData: string): Promise<number> {
  const s = await fetchFeedThreadUnreadSummary(initData);
  return s.count;
}

export async function clearFeedThreadUnread(initData: string, userMessageId?: number): Promise<boolean> {
  if (!apiBase || !initData.trim()) return false;
  try {
    const body: { init_data: string; user_message_id?: number } = { init_data: initData };
    if (userMessageId != null && userMessageId > 0) body.user_message_id = userMessageId;
    const res = await fetch(`${apiBase}/api/miniapp/feed/training/thread/unread-clear`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const j = (await res.json().catch(() => ({}))) as { ok?: boolean };
    return res.ok && Boolean(j.ok);
  } catch {
    return false;
  }
}
