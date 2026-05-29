/** 1 день, 2 дня, 5 дней (целое ≥ 0). */
export function daysWordRu(n: number): string {
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
  return `Стрик: ${days} ${daysWordRu(days)} подряд`;
}

/**
 * Сколько миллисекунд осталось до сгорания стрика.
 *
 * Стрик жив, пока последняя тренировка была сегодня (`daysSinceLastTraining === 0`)
 * или вчера (`=== 1`). Сгорает он в конце дня, следующего за днём последней
 * тренировки (см. ComputeStreakDays на бэкенде: пропуск ≥1 полного дня сбрасывает
 * стрик). Считаем по локальному времени устройства — оно синхронизировано с TZ,
 * по которому бэкенд и определяет границы суток.
 *
 * Возвращает null, если стрика нет, дата последней тренировки неизвестна
 * (`daysSinceLastTraining < 0`) или стрик уже должен сгореть (`> 1`).
 */
export function streakBurnRemainingMs(
  streak: number,
  daysSinceLastTraining: number,
  now: Date = new Date(),
): number | null {
  if (streak <= 0) return null;
  if (daysSinceLastTraining < 0 || daysSinceLastTraining > 1) return null;
  // Дедлайн = конец дня после последней тренировки = начало сегодняшнего дня + (2 − daysSince) суток.
  // daysSince=1 (вчера) → конец сегодня; daysSince=0 (сегодня) → конец завтра.
  const deadline = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  deadline.setDate(deadline.getDate() + (2 - daysSinceLastTraining));
  const ms = deadline.getTime() - now.getTime();
  return ms > 0 ? ms : null;
}

/** «5 ч 20 мин», «1 день 4 ч», «18 мин» — компактный остаток времени до сгорания. */
export function formatStreakBurnRemaining(ms: number): string {
  const totalMin = Math.max(0, Math.floor(ms / 60_000));
  const days = Math.floor(totalMin / (60 * 24));
  const hours = Math.floor((totalMin % (60 * 24)) / 60);
  const mins = totalMin % 60;
  if (days > 0) return `${days} ${daysWordRu(days)} ${hours} ч`;
  if (hours > 0) return `${hours} ч ${mins} мин`;
  return `${mins} мин`;
}

/** Подпись под счётчиком стрика: «сгорит через 5 ч 20 мин» или null, если стрик не горит. */
export function streakBurnLabel(
  streak: number,
  daysSinceLastTraining: number,
  now: Date = new Date(),
): string | null {
  const ms = streakBurnRemainingMs(streak, daysSinceLastTraining, now);
  if (ms == null) return null;
  return `сгорит через ${formatStreakBurnRemaining(ms)}`;
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
