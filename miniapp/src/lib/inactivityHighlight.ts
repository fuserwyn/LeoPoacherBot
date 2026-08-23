/** Подсветка профиля по дням без тренировки (5 — предупреждение, 6–7 — кик). */
export type InactivityHighlight = "none" | "yellow" | "red" | "bordeaux";

export function inactivityHighlight(daysSinceLastTraining: number): InactivityHighlight {
  if (daysSinceLastTraining < 5) return "none";
  if (daysSinceLastTraining === 5) return "yellow";
  if (daysSinceLastTraining === 6) return "red";
  return "bordeaux";
}
