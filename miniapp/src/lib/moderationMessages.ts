/** Системные тексты PRE-модерации (синхрон с backend moderation/warnings.go). */

const MODERATION_MESSAGES: Record<string, string> = {
  moderation_profanity: "Не опубликовал — в тексте есть недопустимые слова. Перепиши без оскорблений и мата.",
  moderation_link: "Не опубликовал — ссылки и упоминания @-каналов в Стае пока запрещены.",
  moderation_critical: "Не опубликовал — такой текст нельзя публиковать.",
  moderation_too_long: "Слишком длинно — сократи текст.",
  moderation_rate_limited: "Подожди немного перед следующим сообщением.",
  moderation_blocked: "Не опубликовал — текст не прошёл проверку.",
};

export function moderationUserMessage(code: string | undefined, fallback?: string): string {
  if (code && MODERATION_MESSAGES[code]) return MODERATION_MESSAGES[code];
  if (fallback?.trim()) return fallback.trim();
  return MODERATION_MESSAGES.moderation_blocked;
}

export function isModerationError(code: string | undefined): boolean {
  if (!code) return false;
  return code.startsWith("moderation_");
}
