import { daysWordRu } from "./streakLabel";

// Единый источник правды по порогам ачивок: используется и витриной в профиле
// (ProfileScreen), и детектором новых ачивок в App (уведомление при достижении).
export const STREAK_ACHIEVEMENTS = [
  { days: 7, colorClass: "profile__achievement--7", variant: "paw" },
  { days: 14, colorClass: "profile__achievement--14", variant: "paw" },
  { days: 30, colorClass: "profile__achievement--30", variant: "paw" },
  { days: 42, colorClass: "profile__achievement--42", variant: "heart" },
  { days: 60, colorClass: "profile__achievement--60", variant: "paw" },
  { days: 90, colorClass: "profile__achievement--90", variant: "paw" },
  { days: 180, colorClass: "profile__achievement--180", variant: "paw" },
  { days: 365, colorClass: "profile__achievement--365", variant: "paw" },
  { days: 420, colorClass: "profile__achievement--420", variant: "paw-crown" },
] as const;

// Ачивки за общее число тренировок. «Заработано» считаем на фронте из total workouts —
// бэкенд про эти пороги не знает (это чисто визуальная витрина в профиле).
// variant: внутри звезды лапка ("paw") или сердце ("heart", для 42 — как у стрика).
// Первый порог — 1: новичок получает ачивку сразу за первую тренировку, а не ждёт десятой.
export const WORKOUT_ACHIEVEMENTS = [
  { count: 1, variant: "paw" },
  { count: 10, variant: "paw" },
  { count: 20, variant: "paw" },
  { count: 42, variant: "heart" },
  { count: 50, variant: "paw" },
  { count: 100, variant: "paw" },
  { count: 200, variant: "paw" },
  { count: 420, variant: "crown" }, // звезда с короной
  { count: 500, variant: "paw" },
  { count: 1000, variant: "leo" }, // медитирующий Лео
] as const;

// Правильное склонение: 1 тренировка, 2-4 тренировки, 5-20 тренировок.
export function workoutsWordRu(n: number): string {
  const mod100 = n % 100;
  if (mod100 >= 11 && mod100 <= 14) return "тренировок";
  const mod10 = n % 10;
  if (mod10 === 1) return "тренировка";
  if (mod10 >= 2 && mod10 <= 4) return "тренировки";
  return "тренировок";
}

/** Ключ ачивки: "streak-7", "workout-10". Стабилен — по нему сверяем «уже видел / новая». */
export type AchievementKey = string;

/**
 * Список ключей уже открытых ачивок по текущим показателям.
 * `achievementCount` — сколько стрик-ачивок начислил бэкенд (первые N порогов по порядку).
 * `workouts` — суммарное число тренировок (пороги считаем на фронте).
 */
export function earnedAchievementKeys(achievementCount: number, workouts: number): AchievementKey[] {
  const keys: AchievementKey[] = [];
  const streakEarned = Math.max(0, Math.min(achievementCount, STREAK_ACHIEVEMENTS.length));
  for (let i = 0; i < streakEarned; i++) {
    keys.push(`streak-${STREAK_ACHIEVEMENTS[i].days}`);
  }
  for (const { count } of WORKOUT_ACHIEVEMENTS) {
    if (workouts >= count) keys.push(`workout-${count}`);
  }
  return keys;
}

export type AchievementKind = "streak" | "workout";

/** Тип и порог ачивки по ключу (для иконки/текста уведомления). */
export function parseAchievementKey(key: AchievementKey): { kind: AchievementKind; threshold: number } | null {
  const [kind, raw] = key.split("-");
  const threshold = Number(raw);
  if ((kind !== "streak" && kind !== "workout") || !Number.isFinite(threshold)) return null;
  return { kind, threshold };
}

/**
 * Новые ачивки, достойные поздравления: которых ещё нет в `seen`, за вычетом порогов,
 * которые пользователь заведомо перерос.
 *
 * Второе нужно при расширении каталога задним числом. Когда мы добавили ачивку за первую
 * тренировку, у ветерана с 300 тренировками ключ "workout-1" тоже стал «новым» — но тост
 * за него был бы нелепым: этот порог он прошёл давным-давно. Празднуем только то, что выше
 * всего уже виденного в своём виде.
 */
export function freshAchievementKeys(earned: AchievementKey[], seen: AchievementKey[]): AchievementKey[] {
  const seenSet = new Set(seen);
  const maxSeen: Record<AchievementKind, number> = { streak: 0, workout: 0 };
  for (const key of seen) {
    const parsed = parseAchievementKey(key);
    if (parsed && parsed.threshold > maxSeen[parsed.kind]) maxSeen[parsed.kind] = parsed.threshold;
  }
  return earned.filter((key) => {
    if (seenSet.has(key)) return false;
    const parsed = parseAchievementKey(key);
    return parsed ? parsed.threshold > maxSeen[parsed.kind] : false;
  });
}

/** Человекочитаемая подпись ачивки для уведомления: «Стрик 7 дней», «10 тренировок». */
export function achievementLabel(key: AchievementKey): string {
  const parsed = parseAchievementKey(key);
  if (!parsed) return "Новая ачивка";
  return parsed.kind === "streak"
    ? `Стрик ${parsed.threshold} ${daysWordRu(parsed.threshold)}`
    : `${parsed.threshold} ${workoutsWordRu(parsed.threshold)}`;
}
