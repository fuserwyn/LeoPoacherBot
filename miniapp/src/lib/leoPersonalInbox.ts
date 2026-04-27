/** Очередь личных сообщений Лео (предупреждения, ответы из poll) — счётчик и забор через тот же API, что ChatScreen. */

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export async function fetchLeoPendingCount(initData: string): Promise<number> {
  if (!apiBase || !initData.trim()) return 0;
  try {
    const res = await fetch(`${apiBase}/api/miniapp/personal-reply/pending-count`, {
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

/** Забирает все ожидающие фрагменты (как серия poll), порядок FIFO. Макс. 25 на цикл. */
export async function drainLeoPersonalInbox(initData: string): Promise<string[]> {
  if (!apiBase || !initData.trim()) return [];
  const out: string[] = [];
  for (let i = 0; i < 25; i++) {
    const res = await fetch(`${apiBase}/api/miniapp/personal-reply/poll`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData }),
    });
    const j = (await res.json().catch(() => ({}))) as { reply_text?: string; ok?: boolean };
    if (!res.ok || !j.ok) break;
    const chunk = j.reply_text?.trim();
    if (!chunk) break;
    out.push(chunk);
  }
  return out;
}
