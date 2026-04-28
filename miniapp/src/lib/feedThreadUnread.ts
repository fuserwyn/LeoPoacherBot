/** Непрочитанные комментарии к твоим тренировкам в ленте стаи (сервер: miniapp_training_thread_unread). */

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export async function fetchFeedThreadUnreadCount(initData: string): Promise<number> {
  if (!apiBase || !initData.trim()) return 0;
  try {
    const res = await fetch(`${apiBase}/api/miniapp/feed/training/thread/unread-count`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData }),
    });
    const j = (await res.json().catch(() => ({}))) as { count?: number; ok?: boolean };
    if (!res.ok || !j.ok) return 0;
    return typeof j.count === "number" && j.count > 0 ? j.count : 0;
  } catch {
    return 0;
  }
}

export async function clearFeedThreadUnread(initData: string): Promise<boolean> {
  if (!apiBase || !initData.trim()) return false;
  try {
    const res = await fetch(`${apiBase}/api/miniapp/feed/training/thread/unread-clear`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData }),
    });
    const j = (await res.json().catch(() => ({}))) as { ok?: boolean };
    return res.ok && Boolean(j.ok);
  } catch {
    return false;
  }
}
