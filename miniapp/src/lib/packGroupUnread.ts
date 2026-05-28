/** Непрочитанные ответы в общем чате стаи (miniapp_pack_group_unread). */

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type PackGroupUnreadSummary = {
  count: number;
  messageIds: number[];
};

export async function fetchPackGroupUnreadSummary(initData: string): Promise<PackGroupUnreadSummary> {
  if (!apiBase || !initData.trim()) return { count: 0, messageIds: [] };
  try {
    const res = await fetch(`${apiBase}/api/miniapp/pack-group/unread-count`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData }),
    });
    const j = (await res.json().catch(() => ({}))) as {
      count?: number;
      pack_message_ids?: number[];
      ok?: boolean;
    };
    if (!res.ok || !j.ok) return { count: 0, messageIds: [] };
    const count = typeof j.count === "number" && j.count > 0 ? j.count : 0;
    const messageIds = Array.isArray(j.pack_message_ids)
      ? j.pack_message_ids.filter((id): id is number => typeof id === "number" && id > 0)
      : [];
    return { count, messageIds };
  } catch {
    return { count: 0, messageIds: [] };
  }
}

/** @deprecated use fetchPackGroupUnreadSummary */
export async function fetchPackGroupUnreadCount(initData: string): Promise<number> {
  const s = await fetchPackGroupUnreadSummary(initData);
  return s.count;
}

export async function clearPackGroupUnread(initData: string): Promise<boolean> {
  if (!apiBase || !initData.trim()) return false;
  try {
    const res = await fetch(`${apiBase}/api/miniapp/pack-group/unread-clear`, {
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
