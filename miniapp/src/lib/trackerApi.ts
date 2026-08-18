/**
 * Доска задач Леопарда. Живёт в MyVibeLab, ходим к ней через свой бэкенд
 * (ms_leo, ручка /api/miniapp/admin/tracker): секрет подписи гостевой сессии
 * нельзя отдавать в браузер, а наш initData MyVibeLab проверить не может —
 * он подписан токеном Леопарда. Формат карточек и статусов общий с MyVibeLab,
 * поэтому типы описывают ровно то, что отдаёт его трекер.
 */
const api = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type TrackerTask = {
  id: number;
  prompt: string;
  repo: string;
  when: string;
  repeat: string;
  kind: string;
  status: string;
  status_label: string;
  status_icon: string;
  done: boolean;
  active: boolean;
  can_delete: boolean;
  auto_review: boolean;
  manual_qa: boolean;
  fast_track: boolean;
  error: string;
  has_result: boolean;
  phase: string;
  qa_status: string | null;
  qa_label: string;
  qa_icon: string;
  auto_qa_running: boolean;
  dev_column: string;
  qa_column: string | null;
  handed_to_qa: boolean;
  attachments_count: number;
  has_attachments: boolean;
  author_id: number | null;
  steps?: string[];
  steps_running?: boolean;
  live_step?: string;
  result?: string;
  created_at?: string;
  last_run_at?: string;
  model_key?: string;
};

export type SprintIdea = {
  id?: string;
  title?: string;
  name?: string;
  summary?: string;
  description?: string;
  [key: string]: unknown;
};

export type SprintFeature = {
  title?: string;
  prompt?: string;
  sprint?: number;
  requirements?: unknown;
  acceptance?: unknown;
  test_criteria?: unknown;
  [key: string]: unknown;
};

export type TrackerOp =
  | "list"
  | "create"
  | "task"
  | "status"
  | "cancel"
  | "delete"
  | "qa"
  | "auto_qa"
  | "prompt"
  | "reschedule"
  | "sprint_ideas"
  | "sprint_generate"
  | "sprint_apply";

type Envelope<T> = { ok?: boolean; data?: T; error?: string; message?: string };

async function call<T>(
  initData: string,
  op: TrackerOp,
  extra: { task_id?: number; payload?: Record<string, unknown> } = {},
): Promise<T> {
  if (!api) throw new Error("API не настроен");
  const res = await fetch(`${api}/api/miniapp/admin/tracker`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, op, ...extra }),
  });
  const j = (await res.json().catch(() => ({}))) as Envelope<T>;
  if (!res.ok || j.ok === false) {
    throw new Error(j.message || trackerErrorLabel(j.error) || `Ошибка ${res.status}`);
  }
  return (j.data ?? ({} as T)) as T;
}

function trackerErrorLabel(code?: string): string {
  switch (code) {
    case "forbidden":
      return "Нет прав администратора";
    case "chat_mismatch":
      return "Открой мини-апп из чата стаи";
    case "tracker not configured":
    case "tracker_not_configured":
      return "Трекер не настроен: нет BOARD_SSO_SECRET";
    case "invalid_action":
      return "Такое действие доске недоступно";
    default:
      return code ?? "";
  }
}

export function trackerList(initData: string) {
  return call<{ tasks: TrackerTask[]; repo: string | null }>(initData, "list");
}

export function trackerCreate(
  initData: string,
  payload: { when: string; prompt: string; auto_review?: boolean; manual_qa?: boolean; fast_track?: boolean },
) {
  return call<{ id: number; when: string }>(initData, "create", { payload });
}

export function trackerTask(initData: string, taskId: number) {
  return call<{ task: TrackerTask }>(initData, "task", { task_id: taskId });
}

export function trackerCancel(initData: string, taskId: number) {
  return call<{ ok: boolean }>(initData, "cancel", { payload: { id: taskId } });
}

export function trackerDelete(initData: string, taskId: number) {
  return call<{ ok: boolean }>(initData, "delete", { task_id: taskId });
}

/** Доска тестировщика: start — взять в тест, pass — принять, fail — вернуть, reset — в очередь. */
export function trackerQa(initData: string, taskId: number, action: "start" | "pass" | "fail" | "reset") {
  return call<{ ok: boolean }>(initData, "qa", { payload: { id: taskId, action } });
}

export function trackerAutoQa(initData: string, taskId: number) {
  return call<{ ok: boolean }>(initData, "auto_qa", { payload: { id: taskId } });
}

export function sprintIdeas(initData: string, hint: string) {
  return call<{ ideas: SprintIdea[]; recommended_id?: string; analysis?: string; topic_question?: string }>(
    initData,
    "sprint_ideas",
    { payload: { hint } },
  );
}

export function sprintGenerate(
  initData: string,
  payload: { hint: string; idea: SprintIdea; sprint_count: number; tasks_per_sprint: number },
) {
  return call<{ features: SprintFeature[]; plan?: Record<string, unknown>; analysis?: string }>(
    initData,
    "sprint_generate",
    { payload },
  );
}

export function sprintApply(
  initData: string,
  payload: { features: SprintFeature[]; sprint_count: number; tasks_per_sprint: number },
) {
  return call<{ created?: number; ok?: boolean }>(initData, "sprint_apply", { payload });
}
