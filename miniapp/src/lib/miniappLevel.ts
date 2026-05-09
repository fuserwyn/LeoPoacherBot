/**
 * Накопленные кубки на границах уровней (шкала 1, §2.5): L1 с 0, L2 с 420, …
 * Индекс i = минимум кубков для уровня (i+1).
 */
export const CUP_LEVEL_STARTS: readonly number[] = [0, 420, 1260, 2940, 6300, 13020, 26460];

/**
 * Имена уровней-животных. Источник истины — leopardmoney.LevelNames на бэке
 * (ms_leo/internal/game/leopardmoney/const.go). Индекс = номер уровня (1-based);
 * "" на индексе 0 — нулевой уровень не используется.
 * L7+ — endgame без отдельного имени животного, оставляем последнего «Слон».
 */
export const MINIAPP_LEVEL_NAMES: readonly string[] = ["", "Сурикат", "Газель", "Зебра", "Гепард", "Лев", "Слон"];

/** Имя уровня по номеру 1-based; для L7+ возвращает «Слон» (endgame). */
export function miniappLevelName(level: number): string {
  if (level < 1) return "";
  if (level >= MINIAPP_LEVEL_NAMES.length) return MINIAPP_LEVEL_NAMES[MINIAPP_LEVEL_NAMES.length - 1] ?? "";
  return MINIAPP_LEVEL_NAMES[level] ?? "";
}

/** Размер сегмента L6→L7+ для прогресс-бара после входа в L7 (endgame). */
const CUPS_SEGMENT_ENDGAME = 13440;

/** Уровень 1…7 по накопленным кубкам. */
export function miniappLevelFromCups(cups: number): number {
  const c = Math.max(0, Math.floor(cups));
  let level = 1;
  for (let i = 1; i < CUP_LEVEL_STARTS.length; i++) {
    if (c >= CUP_LEVEL_STARTS[i]!) level = i + 1;
    else break;
  }
  return Math.min(level, CUP_LEVEL_STARTS.length);
}

export type CupsLevelProgress = {
  level: number;
  /** Кубки в текущем сегменте (от начала уровня). */
  cupsInSegment: number;
  /** Кубков до следующего уровня (в пределах сегмента). */
  cupsToNext: number;
};

/**
 * Прогресс внутри уровня для полоски «кубки / до следующего уровня».
 * Для L7+ знаменатель — фиксированный сегмент 13 440 (TBD в спеке).
 */
export function miniappCupsLevelProgress(cups: number): CupsLevelProgress {
  const c = Math.max(0, Math.floor(cups));
  const level = miniappLevelFromCups(c);
  const idx = level - 1;
  const start = CUP_LEVEL_STARTS[idx] ?? 0;
  const nextThreshold = idx + 1 < CUP_LEVEL_STARTS.length ? CUP_LEVEL_STARTS[idx + 1]! : null;
  const cupsInSegment = c - start;
  if (nextThreshold != null) {
    const segmentSize = nextThreshold - start;
    return { level, cupsInSegment, cupsToNext: Math.max(1, segmentSize) };
  }
  const seg = CUPS_SEGMENT_ENDGAME;
  const inSeg = cupsInSegment % seg;
  return { level, cupsInSegment: inSeg, cupsToNext: seg };
}

/** @deprecated Используй miniappLevelFromCups; оставлено для старых импортов. */
export const MINIAPP_LEVEL_XP_STEP = 200;

/** @deprecated */
export function miniappLevelFromXp(xp: number): number {
  return miniappLevelFromCups(xp);
}
