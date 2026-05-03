/** Подпись к дате кика за неактивность (ISO из API, обычно Europe/Moscow). */
export function formatInactivityRemovalHint(iso: string | null | undefined): string | null {
  const s = (iso ?? "").trim();
  if (!s) return null;
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return null;
  try {
    return new Intl.DateTimeFormat("ru-RU", {
      day: "numeric",
      month: "short",
      hour: "2-digit",
      minute: "2-digit",
      timeZone: "Europe/Moscow",
    }).format(d);
  } catch {
    return null;
  }
}
