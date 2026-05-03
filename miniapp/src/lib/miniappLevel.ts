/** Совпадает с шагом «до следующего уровня» в профиле (кубки). */
export const MINIAPP_LEVEL_XP_STEP = 200;

/** Уровень для UI: 1 при xp до 199, 2 при 200–399, … */
export function miniappLevelFromXp(xp: number): number {
  const x = Math.max(0, Math.floor(xp));
  return Math.max(1, Math.floor(x / MINIAPP_LEVEL_XP_STEP) + 1);
}
