/** Склонение «день / дня / дней» (как russianPlural на бэкенде). */
function daysWordForm(count: number): string {
  const n = Math.abs(count) % 100;
  const n1 = n % 10;
  if (n > 10 && n < 20) return "дней";
  if (n1 > 1 && n1 < 5) return "дня";
  if (n1 === 1) return "день";
  return "дней";
}

/** Формат остатка до кика — зеркало formatDurationToDays на бэкенде. */
export function formatDurationToDays(ms: number): string {
  const totalMinutes = Math.max(0, Math.floor(ms / 60_000));
  const days = Math.floor(totalMinutes / (24 * 60));
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
  const minutes = totalMinutes % 60;

  if (days > 0) {
    if (hours > 0) return `${days} ${daysWordForm(days)} ${hours} ч.`;
    return `${days} ${daysWordForm(days)}`;
  }
  if (hours > 0) {
    if (minutes > 0) return `${hours} ч. ${minutes} мин.`;
    return `${hours} ч.`;
  }
  return `${minutes} мин.`;
}

export type RemovalRemaining = {
  ms: number;
  text: string;
  deadlineLocal: string;
};

/** День предупреждения по таймеру (5/6/7), как milestone за 72/48/24 ч до кика. null — ещё рано. */
export function inactiveDaysFromRemovalRemaining(ms: number): number | null {
  const hours = ms / 3_600_000;
  if (hours > 72) return null;
  if (hours > 48) return 5;
  if (hours > 24) return 6;
  return 7;
}

/** Остаток до inactivity_removal_at (RFC3339 с бэкенда). */
export function removalRemainingUntil(removalAtRFC3339: string, now: Date): RemovalRemaining | null {
  const deadline = new Date(removalAtRFC3339);
  if (Number.isNaN(deadline.getTime())) return null;
  const ms = deadline.getTime() - now.getTime();
  if (ms <= 0) return null;
  const pad = (n: number) => String(n).padStart(2, "0");
  const deadlineLocal = `${pad(deadline.getDate())}.${pad(deadline.getMonth() + 1)}.${deadline.getFullYear()}, ${pad(deadline.getHours())}:${pad(deadline.getMinutes())}`;
  return { ms, text: formatDurationToDays(ms), deadlineLocal };
}
