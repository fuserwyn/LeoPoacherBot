/** Склонение «день» для серии (целое ≥ 0). */
function streakDayWord(n: number): string {
  const abs = Math.abs(n) % 100;
  const d = Math.abs(n) % 10;
  if (abs >= 11 && abs <= 14) return "дней";
  if (d === 1) return "день";
  if (d >= 2 && d <= 4) return "дня";
  return "дней";
}

/** Для aria / подсказок к бейджу «Стрик» в ленте. */
export function streakStreakAriaLabel(days: number): string {
  if (days <= 0) return "Стрик: пока нет дней подряд с тренировками";
  return `Стрик: ${days} ${streakDayWord(days)} подряд`;
}

/** Склонение «кубок» для счётчика прогресса. */
export function cupsWordRu(n: number): string {
  const abs = Math.abs(n) % 100;
  const d = Math.abs(n) % 10;
  if (abs >= 11 && abs <= 14) return "кубков";
  if (d === 1) return "кубок";
  if (d >= 2 && d <= 4) return "кубка";
  return "кубков";
}
