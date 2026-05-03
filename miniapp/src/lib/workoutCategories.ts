/** Совпадает с типами в NewWorkoutScreen / текстом отчёта в App.tsx (`kind`). */
export type WorkoutCategoryId =
  | "run"
  | "walk"
  | "bike"
  | "swim"
  | "yoga"
  | "rowing"
  | "workout"
  | "crossfit"
  | "stretch"
  | "dance"
  | "hiit"
  | "cardio"
  | "kettlebell"
  | "strength"
  | "other";

export type WorkoutCategoryOption = { id: WorkoutCategoryId; label: string; emoji: string };

/** Порядок — как в форме новой тренировки (можно импортировать как список типов в форме). */
export const WORKOUT_CATEGORY_OPTIONS: WorkoutCategoryOption[] = [
  { id: "run", label: "Бег", emoji: "🏃" },
  { id: "walk", label: "Ходьба", emoji: "🚶" },
  { id: "bike", label: "Велосипед", emoji: "🚴" },
  { id: "swim", label: "Плавание", emoji: "🏊" },
  { id: "yoga", label: "Йога", emoji: "🧘" },
  { id: "rowing", label: "Гребля", emoji: "🚣" },
  { id: "workout", label: "Воркаут", emoji: "🔥" },
  { id: "crossfit", label: "Кроссфит", emoji: "🎯" },
  { id: "stretch", label: "Растяжка", emoji: "🧎" },
  { id: "dance", label: "Танцы", emoji: "💃" },
  { id: "hiit", label: "HIIT", emoji: "⚡" },
  { id: "cardio", label: "Кардио", emoji: "💓" },
  { id: "kettlebell", label: "Гиря", emoji: "🏋️" },
  { id: "strength", label: "Силовая", emoji: "🏋️" },
  { id: "other", label: "Другое", emoji: "✨" },
];

/** Для фильтра ленты: по русскому алфавиту названий (HIIT и латиница — по правилам `ru`). */
export const WORKOUT_CATEGORY_OPTIONS_ALPHABETICAL: WorkoutCategoryOption[] = [...WORKOUT_CATEGORY_OPTIONS].sort(
  (a, b) => a.label.localeCompare(b.label, "ru", { sensitivity: "base" }),
);

const FEED_CAT_ORDER = new Map(WORKOUT_CATEGORY_OPTIONS_ALPHABETICAL.map((o, i) => [o.id, i]));

/** Порядок id как в алфавитном списке фильтра (для чипов «выбрано»). */
export function sortWorkoutCategoryIds(ids: readonly WorkoutCategoryId[]): WorkoutCategoryId[] {
  return [...ids].sort((a, b) => (FEED_CAT_ORDER.get(a) ?? 0) - (FEED_CAT_ORDER.get(b) ?? 0));
}

/** Алиас для экрана новой тренировки. */
export const WORKOUT_TYPES = WORKOUT_CATEGORY_OPTIONS;

const LABEL_TO_ID = Object.fromEntries(
  WORKOUT_CATEGORY_OPTIONS.map((o) => [o.label.trim().toLowerCase(), o.id]),
) as Record<string, WorkoutCategoryId>;

const ID_TO_EMOJI = Object.fromEntries(WORKOUT_CATEGORY_OPTIONS.map((o) => [o.id, o.emoji])) as Record<
  WorkoutCategoryId,
  string
>;

/** Эмодзи в заголовке карточки ленты для отчёта мини-аппа; если формат не распознан — 💪. */
export function trainingDoneCategoryEmoji(text: string): string {
  const cat = parseTrainingDoneCategory(text);
  if (cat === null) return "💪";
  return ID_TO_EMOJI[cat] ?? "💪";
}

/** Заголовок карточки вместо общего «Тренировка»: тип из строки отчёта. */
export function trainingDoneCategoryDisplayLabel(text: string): string {
  const cat = parseTrainingDoneCategory(text);
  if (cat === null) return "Другое";
  return WORKOUT_CATEGORY_OPTIONS.find((o) => o.id === cat)?.label ?? "Другое";
}

/**
 * Первая строка текста отчёта из мини-аппа:
 * `#training_done — бег, 15 мин, инт. 3/5`
 */
export function parseTrainingDoneCategory(text: string): WorkoutCategoryId | null {
  const line = (text.trim().split("\n")[0] ?? "").trim();
  const m = line.match(/^#training_done\s*[—–-]\s*([^,]+),\s*\d+\s*мин/i);
  if (!m) return null;
  const raw = m[1].trim().toLowerCase();
  if (LABEL_TO_ID[raw]) return LABEL_TO_ID[raw];
  return "other";
}

/** Совпадение с выбранным фильтром категории (для `training_done`). */
export function trainingDoneMatchesCategory(text: string, filterId: WorkoutCategoryId): boolean {
  const cat = parseTrainingDoneCategory(text);
  if (cat == null) return filterId === "other";
  if (filterId === "other") return cat === "other";
  return cat === filterId;
}

/** Мультивыбор: отчёт попадает в ленту, если его тип есть в `selected` (не пустом). */
export function trainingDoneMatchesAnyCategory(
  text: string,
  selected: ReadonlySet<WorkoutCategoryId>,
): boolean {
  if (selected.size === 0) return true;
  const cat = parseTrainingDoneCategory(text);
  if (cat !== null) return selected.has(cat);
  return selected.has("other");
}
