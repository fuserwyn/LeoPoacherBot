import { daysWordRu } from "./streakLabel";

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
  if (d < 7) return `${d} ${daysWordRu(d)} назад`;
  return new Date(t).toLocaleString("ru-RU", { day: "2-digit", month: "2-digit" });
}

/**
 * Локальные дата и время для ленты и чатов (часовой пояс устройства).
 * Принимает ISO-строку или unix-millis.
 */
export function formatChatTime(input: string | number): string {
  const t = typeof input === "number" ? input : new Date(input).getTime();
  if (Number.isNaN(t)) return "";
  const d = new Date(t);
  const date = d.toLocaleDateString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
  const time = d.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  return `${date}, ${time}`;
}

/** То же, что formatChatTime — для ленты и карточек. */
export const formatLocalDateTime = formatChatTime;
