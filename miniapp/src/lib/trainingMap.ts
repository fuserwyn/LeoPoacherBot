import type { WorkoutCategoryId } from "./workoutCategories";

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type TrainingMapStatus = "done" | "next" | "remaining";

export type TrainingMapNode = {
  id: WorkoutCategoryId;
  label: string;
  emoji: string;
  index: number;
  status: TrainingMapStatus;
  x: number;
  y: number;
};

export type TrainingMapSnapshot = {
  workoutsTotal: number;
  completed: number;
  remaining: number;
  nextIndex: number;
  lap: number;
  nodes: TrainingMapNode[];
};

/** Порядок узлов карты — совпадает с ms_leo/internal/game/trainingmap. */
export const TRAINING_MAP_PATH: { id: WorkoutCategoryId; label: string; emoji: string; x: number; y: number }[] = [
  { id: "run", label: "Бег", emoji: "🏃", x: 16, y: 16 },
  { id: "strength", label: "Силовая", emoji: "🏋️", x: 50, y: 10 },
  { id: "yoga", label: "Йога", emoji: "🧘", x: 84, y: 18 },
  { id: "walk", label: "Ходьба", emoji: "🚶", x: 78, y: 46 },
  { id: "hiit", label: "HIIT", emoji: "⚡", x: 42, y: 42 },
  { id: "bike", label: "Велосипед", emoji: "🚴", x: 18, y: 62 },
  { id: "stretch", label: "Растяжка", emoji: "🧎", x: 48, y: 78 },
  { id: "swim", label: "Плавание", emoji: "🏊", x: 84, y: 82 },
];

export function buildTrainingMapSnapshot(workoutsTotal: number): TrainingMapSnapshot {
  const n = TRAINING_MAP_PATH.length;
  const total = Number.isFinite(workoutsTotal) && workoutsTotal > 0 ? Math.floor(workoutsTotal) : 0;
  const completed = n === 0 ? 0 : total % n;
  const remaining = n - completed;
  const nextIndex = completed;
  const lap = Math.floor(total / n) + 1;
  const nodes: TrainingMapNode[] = TRAINING_MAP_PATH.map((def, i) => ({
    ...def,
    index: i,
    status: i < completed ? "done" : i === nextIndex ? "next" : "remaining",
  }));
  return { workoutsTotal: total, completed, remaining, nextIndex, lap, nodes };
}

type ApiNode = {
  id?: string;
  label?: string;
  emoji?: string;
  index?: number;
  status?: string;
  x?: number;
  y?: number;
};

function parseApiSnapshot(j: {
  workouts_total?: number;
  completed?: number;
  remaining?: number;
  next_index?: number;
  lap?: number;
  nodes?: ApiNode[];
}): TrainingMapSnapshot | null {
  if (!Array.isArray(j.nodes) || j.nodes.length === 0) return null;
  const ids = new Set(TRAINING_MAP_PATH.map((p) => p.id));
  const nodes: TrainingMapNode[] = [];
  for (const raw of j.nodes) {
    const id = raw.id as WorkoutCategoryId | undefined;
    if (!id || !ids.has(id)) continue;
    const status: TrainingMapStatus =
      raw.status === "done" || raw.status === "next" || raw.status === "remaining" ? raw.status : "remaining";
    const fallback = TRAINING_MAP_PATH.find((p) => p.id === id);
    nodes.push({
      id,
      label: typeof raw.label === "string" && raw.label ? raw.label : fallback?.label ?? id,
      emoji: typeof raw.emoji === "string" && raw.emoji ? raw.emoji : fallback?.emoji ?? "💪",
      index: typeof raw.index === "number" ? raw.index : nodes.length,
      status,
      x: typeof raw.x === "number" ? raw.x : fallback?.x ?? 50,
      y: typeof raw.y === "number" ? raw.y : fallback?.y ?? 50,
    });
  }
  if (nodes.length === 0) return null;
  const n = nodes.length;
  const total = typeof j.workouts_total === "number" ? Math.max(0, Math.floor(j.workouts_total)) : 0;
  const completed = typeof j.completed === "number" ? j.completed : total % n;
  const remaining = typeof j.remaining === "number" ? j.remaining : Math.max(0, n - completed);
  const nextIndex = typeof j.next_index === "number" ? j.next_index : completed % n;
  const lap = typeof j.lap === "number" ? j.lap : Math.floor(total / n) + 1;
  return { workoutsTotal: total, completed, remaining, nextIndex, lap, nodes };
}

export async function fetchTrainingMap(initData: string, fallbackWorkouts: number): Promise<TrainingMapSnapshot> {
  const fallback = buildTrainingMapSnapshot(fallbackWorkouts);
  if (!apiBase || !initData.trim()) return fallback;
  try {
    const res = await fetch(`${apiBase}/api/miniapp/training-map`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData }),
    });
    const j = (await res.json().catch(() => ({}))) as { ok?: boolean } & Parameters<typeof parseApiSnapshot>[0];
    if (!res.ok || !j.ok) return fallback;
    return parseApiSnapshot(j) ?? fallback;
  } catch {
    return fallback;
  }
}
