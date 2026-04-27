/** Отправка текста в личку бота через ms_leo (как ChatScreen). */

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type MiniappSendResult = { ok: true; replyParts: string[] } | { ok: false; error: string };

/**
 * Собирает ответы бота (сразу или через poll), как в ChatScreen.
 */
export async function sendMiniappPrivateText(initData: string, text: string): Promise<MiniappSendResult> {
  if (!apiBase) {
    return { ok: false, error: "Нет VITE_MINIAPP_API_URL в сборке." };
  }
  if (!initData.trim()) {
    return { ok: false, error: "Нужен initData (открой апп из Telegram)." };
  }
  const res = await fetch(`${apiBase}/api/miniapp/messages`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, text }),
  });
  const j = (await res.json().catch(() => ({}))) as {
    error?: string;
    ok?: boolean;
    pending?: boolean;
    reply_text?: string;
  };
  if (!res.ok) {
    return { ok: false, error: j.error ?? `Ошибка ${res.status}` };
  }
  const parts: string[] = [];
  const replyNow = j.reply_text?.trim();
  if (replyNow) {
    parts.push(replyNow);
    return { ok: true, replyParts: parts };
  }
  if (j.pending) {
    const deadline = Date.now() + 4 * 60 * 1000;
    let gotAny = false;
    let idleEmpty = 0;
    while (Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 1500));
      const pr = await fetch(`${apiBase}/api/miniapp/personal-reply/poll`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData }),
      });
      const pj = (await pr.json().catch(() => ({}))) as { reply_text?: string; error?: string; ok?: boolean };
      if (!pr.ok) {
        return { ok: false, error: pj.error ?? `Ошибка poll ${pr.status}` };
      }
      const chunk = pj.reply_text?.trim();
      if (chunk) {
        gotAny = true;
        idleEmpty = 0;
        parts.push(chunk);
        continue;
      }
      idleEmpty++;
      if (gotAny && idleEmpty >= 2) break;
    }
    if (!gotAny) {
      parts.push("Ответ задерживается. Загляни в личку с ботом в Telegram.");
    }
    return { ok: true, replyParts: parts };
  }
  parts.push("Сообщение ушло боту. Открой чат с ботом в Telegram, если ждёшь ответ там.");
  return { ok: true, replyParts: parts };
}
