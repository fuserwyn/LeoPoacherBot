/** Разница от now до даты, по-русски (грубая). */
export function timeAgoFromISO(iso: string, now = Date.now()): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "—";
  const s = Math.max(0, Math.floor((now - t) / 1000));
  if (s < 60) return "только что";
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} мин назад`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} ч назад`;
  const d = Math.floor(h / 24);
  if (d < 7) return `${d} дн. назад`;
  return new Date(t).toLocaleString("ru-RU", { day: "2-digit", month: "2-digit" });
}

/**
 * Время для чатов: HH:MM, если сегодня; dd.MM HH:MM, если в этом году;
 * dd.MM.YY HH:MM, если раньше. По локальному часовому поясу пользователя — как
 * в Telegram. Принимает либо ISO-строку, либо unix-millis.
 */
export function formatChatTime(input: string | number, now = Date.now()): string {
  const t = typeof input === "number" ? input : new Date(input).getTime();
  if (Number.isNaN(t)) return "";
  const d = new Date(t);
  const nd = new Date(now);
  const sameDay =
    d.getFullYear() === nd.getFullYear() &&
    d.getMonth() === nd.getMonth() &&
    d.getDate() === nd.getDate();
  const hhmm = d.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  if (sameDay) return hhmm;
  const sameYear = d.getFullYear() === nd.getFullYear();
  if (sameYear) {
    return `${d.toLocaleDateString("ru-RU", { day: "2-digit", month: "2-digit" })} ${hhmm}`;
  }
  return `${d.toLocaleDateString("ru-RU", { day: "2-digit", month: "2-digit", year: "2-digit" })} ${hhmm}`;
}
