/** Отправка текста в личку бота через ms_leo (как ChatScreen). */

import { isModerationError, moderationUserMessage } from "./moderationMessages";

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type MiniappSendResult =
  | { ok: true; replyParts: string[]; trainingPhotoUrl?: string }
  | { ok: false; error: string };

export type MiniappSendOptions = {
  /**
   * Если false и сервер вернул pending без reply_text — не ждать ответ через poll
   * (быстрый UI; комментарий Лео появится в ленте позже).
   * По умолчанию true (как в чате: дождаться текста).
   */
  awaitReply?: boolean;
};

async function pollPendingReplies(initData: string): Promise<string[]> {
  const parts: string[] = [];
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
      throw new Error(pj.error ?? `Ошибка poll ${pr.status}`);
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
  return parts;
}

/**
 * Собирает ответы бота (сразу или через poll), как в ChatScreen.
 */
export async function sendMiniappPrivateText(
  initData: string,
  text: string,
  opts?: MiniappSendOptions,
): Promise<MiniappSendResult> {
  const awaitReply = opts?.awaitReply !== false;
  if (!apiBase) {
    return { ok: false, error: "Нет VITE_MINIAPP_API_URL в сборке." };
  }
  if (!initData.trim()) {
    return { ok: false, error: "Нужен initData (открой апп из Telegram)." };
  }
  let res: Response;
  try {
    res = await fetch(`${apiBase}/api/miniapp/messages`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData, text }),
    });
  } catch (e) {
    return {
      ok: false,
      error: `Сеть недоступна: ${e instanceof Error ? e.message : String(e)}`,
    };
  }
  const j = (await res.json().catch(() => ({}))) as {
    error?: string;
    message?: string;
    ok?: boolean;
    pending?: boolean;
    reply_text?: string;
  };
  if (!res.ok) {
    if (isModerationError(j.error)) {
      return { ok: false, error: moderationUserMessage(j.error, j.message) };
    }
    return { ok: false, error: j.error ?? `Ошибка ${res.status}` };
  }
  const replyNow = j.reply_text?.trim();
  if (replyNow) {
    return { ok: true, replyParts: [replyNow] };
  }
  if (j.pending) {
    if (!awaitReply) {
      return { ok: true, replyParts: [] };
    }
    try {
      const parts = await pollPendingReplies(initData);
      return { ok: true, replyParts: parts };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : String(e) };
    }
  }
  return { ok: true, replyParts: ["Сообщение ушло боту. Открой чат с ботом в Telegram, если ждёшь ответ там."] };
}

/** То же для отчёта с фото: POST multipart /api/miniapp/workout. */
export async function sendMiniappTrainingWithPhoto(
  initData: string,
  text: string,
  photo: File,
  opts?: MiniappSendOptions,
): Promise<MiniappSendResult> {
  const awaitReply = opts?.awaitReply !== false;
  if (!apiBase) {
    return { ok: false, error: "Нет VITE_MINIAPP_API_URL в сборке." };
  }
  if (!initData.trim()) {
    return { ok: false, error: "Нужен initData (открой апп из Telegram)." };
  }
  const fd = new FormData();
  fd.append("init_data", initData);
  fd.append("text", text);
  fd.append("photo", photo, photo.name || "photo.jpg");
  let res: Response;
  try {
    res = await fetch(`${apiBase}/api/miniapp/workout`, {
      method: "POST",
      body: fd,
    });
  } catch (e) {
    return {
      ok: false,
      error: `Сеть недоступна: ${e instanceof Error ? e.message : String(e)}`,
    };
  }
  const j = (await res.json().catch(() => ({}))) as {
    error?: string;
    message?: string;
    ok?: boolean;
    pending?: boolean;
    reply_text?: string;
    photo_url?: string;
  };
  if (!res.ok) {
    if (res.status === 400 && j.error === "media_not_configured") {
      return {
        ok: false,
        error: "Фото тренировки на сервере не настроено (MINIAPP_PUBLIC_BASE_URL / каталог media).",
      };
    }
    if (res.status === 400 && j.error === "unsupported_image") {
      return { ok: false, error: "Не удалось прочитать фото. Попробуй JPG, PNG, WEBP или GIF." };
    }
    if (res.status === 400 && j.error === "photo_too_large") {
      return { ok: false, error: "Фото слишком большое. Максимум 6 МБ." };
    }
    if (res.status === 400 && j.error === "invalid_multipart") {
      return { ok: false, error: "Не удалось отправить фото. Попробуй выбрать снимок заново." };
    }
    if (isModerationError(j.error)) {
      return { ok: false, error: moderationUserMessage(j.error, j.message) };
    }
    return { ok: false, error: j.error ?? `Ошибка ${res.status}` };
  }
  const photoURL = j.photo_url?.trim();
  const replyNow = j.reply_text?.trim();
  if (replyNow) {
    return { ok: true, replyParts: [replyNow], trainingPhotoUrl: photoURL || undefined };
  }
  if (j.pending) {
    if (!awaitReply) {
      return { ok: true, replyParts: [], trainingPhotoUrl: photoURL || undefined };
    }
    try {
      const parts = await pollPendingReplies(initData);
      return { ok: true, replyParts: parts, trainingPhotoUrl: photoURL || undefined };
    } catch (e) {
      return { ok: false, error: e instanceof Error ? e.message : String(e) };
    }
  }
  return {
    ok: true,
    replyParts: ["Сообщение ушло боту. Открой чат с ботом в Telegram, если ждёшь ответ там."],
    trainingPhotoUrl: photoURL || undefined,
  };
}
