/**
 * Доска задач Леопарда. Живёт в MyVibeLab, ходим к ней через свой бэкенд
 * (ms_leo, ручка /api/miniapp/admin/tracker): секрет подписи гостевой сессии
 * нельзя отдавать в браузер, а наш initData MyVibeLab проверить не может —
 * он подписан токеном Леопарда. Формат карточек и статусов общий с MyVibeLab,
 * поэтому типы описывают ровно то, что отдаёт его трекер.
 */
const api = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type TrackerAttachment = {
  id: string;
  name: string;
  mime: string;
  size: number;
  url: string;
};

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
  /** Приходит только в детали задачи: что именно приложено. */
  attachments?: TrackerAttachment[];
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
  payload: {
    when: string;
    prompt: string;
    auto_review?: boolean;
    manual_qa?: boolean;
    fast_track?: boolean;
    /** Задачу придумал Лео: на доске автором пишем его. */
    leo?: boolean;
  },
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

/** Перенести запуск. Отменённую/завершённую этим же вернуть в «Ожидает» — время должно быть в будущем. */
export function trackerReschedule(initData: string, taskId: number, when: string) {
  return call<{ ok: boolean }>(initData, "reschedule", { payload: { id: taskId, when } });
}

/** Возобновить отменённую задачу немедленно. */
export function trackerRunNow(initData: string, taskId: number) {
  return call<{ ok: boolean }>(initData, "reschedule", { payload: { id: taskId, mode: "now" } });
}

export function trackerAutoQa(initData: string, taskId: number) {
  return call<{ ok: boolean }>(initData, "auto_qa", { payload: { id: taskId } });
}

/** Приложить картинку к задаче: base64 без префикса data:. */
export async function trackerAttachImage(
  initData: string,
  taskId: number,
  image: { data: string; filename: string; mime: string },
): Promise<void> {
  if (!api) throw new Error("API не настроен");
  const res = await fetch(`${api}/api/miniapp/admin/tracker/attach`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, task_id: taskId, ...image }),
  });
  const j = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string; message?: string };
  if (!res.ok || j.ok === false) {
    throw new Error(j.message || trackerErrorLabel(j.error) || `Ошибка ${res.status}`);
  }
}

/** Лео сам придумывает задачу. hint пуст — тему выбирает сам; busy — что уже на доске и что отклонили. */
export async function leoProposeTask(
  initData: string,
  hint: string,
  busy: string[],
): Promise<{ reply: string; title: string; task: string }> {
  if (!api) throw new Error("API не настроен");
  const res = await fetch(`${api}/api/miniapp/admin/tracker/leo-propose`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, hint, busy }),
  });
  const j = (await res.json().catch(() => ({}))) as {
    ok?: boolean;
    reply?: string;
    title?: string;
    task?: string;
    error?: string;
    message?: string;
  };
  if (!res.ok || j.ok === false) {
    throw new Error(j.message || trackerErrorLabel(j.error) || `Ошибка ${res.status}`);
  }
  return { reply: j.reply ?? "", title: j.title ?? "", task: j.task ?? "" };
}

/** Байты приложенного фото: MyVibeLab отдаёт их по гостевой куке, поэтому идём через свой бэкенд. */
export async function trackerAttachmentGet(
  initData: string,
  taskId: number,
  attId: string,
): Promise<string> {
  if (!api) throw new Error("API не настроен");
  const res = await fetch(`${api}/api/miniapp/admin/tracker/attachment`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, action: "get", task_id: taskId, att_id: attId }),
  });
  const j = (await res.json().catch(() => ({}))) as {
    ok?: boolean;
    mime?: string;
    data?: string;
    error?: string;
    message?: string;
  };
  if (!res.ok || j.ok === false || !j.data) {
    throw new Error(j.message || trackerErrorLabel(j.error) || `Ошибка ${res.status}`);
  }
  return `data:${j.mime || "image/jpeg"};base64,${j.data}`;
}

/** Снять фото с задачи. «Заменить» — это снять старое и приложить новое. */
export async function trackerAttachmentDelete(
  initData: string,
  taskId: number,
  attId: string,
): Promise<void> {
  if (!api) throw new Error("API не настроен");
  const res = await fetch(`${api}/api/miniapp/admin/tracker/attachment`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, action: "delete", task_id: taskId, att_id: attId }),
  });
  const j = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string; message?: string };
  if (!res.ok || j.ok === false) {
    throw new Error(j.message || trackerErrorLabel(j.error) || `Ошибка ${res.status}`);
  }
}

/** Автономный режим: Лео сам придумывает спринты, пока админ его не выключит. */
export type LeoAutonomy = {
  active: boolean;
  active_until: string;
  next_run_at: string;
  every_hours: number;
  tasks_per_run: number;
  last_run_at: string;
  last_note: string;
  max_days: number;
};

/** action: status — только посмотреть; start — включить на days дней; stop — выключить. */
export async function leoAutonomy(
  initData: string,
  payload: { action: "status" | "start" | "stop"; days?: number; every_hours?: number; tasks_per_run?: number },
): Promise<LeoAutonomy> {
  if (!api) throw new Error("API не настроен");
  const res = await fetch(`${api}/api/miniapp/admin/tracker/leo-autonomy`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, ...payload }),
  });
  const j = (await res.json().catch(() => ({}))) as {
    ok?: boolean;
    autonomy?: LeoAutonomy;
    error?: string;
    message?: string;
  };
  if (!res.ok || j.ok === false || !j.autonomy) {
    throw new Error(j.message || trackerErrorLabel(j.error) || `Ошибка ${res.status}`);
  }
  return j.autonomy;
}

/** Спринт глазами Лео: реплика, тема и набор задач. */
export async function leoSprint(
  initData: string,
  hint: string,
): Promise<{ reply: string; theme: string; tasks: string[] }> {
  if (!api) throw new Error("API не настроен");
  const res = await fetch(`${api}/api/miniapp/admin/tracker/leo-sprint`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, hint }),
  });
  const j = (await res.json().catch(() => ({}))) as {
    ok?: boolean;
    reply?: string;
    theme?: string;
    tasks?: string[];
    error?: string;
    message?: string;
  };
  if (!res.ok || j.ok === false) {
    throw new Error(j.message || trackerErrorLabel(j.error) || `Ошибка ${res.status}`);
  }
  return { reply: j.reply ?? "", theme: j.theme ?? "", tasks: j.tasks ?? [] };
}

export type TrackerPerson = { user_id: number; username: string; display_name: string };

/** Кто ставил задачи: в трекере есть только author_id, ник живёт у нас. */
export async function trackerAuthors(initData: string, ids: number[]): Promise<TrackerPerson[]> {
  if (!api || ids.length === 0) return [];
  const res = await fetch(`${api}/api/miniapp/admin/tracker/authors`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, ids }),
  });
  const j = (await res.json().catch(() => ({}))) as { ok?: boolean; people?: TrackerPerson[] };
  if (!res.ok || j.ok === false) return [];
  return j.people ?? [];
}

/** Аватарка участника: сервер отдаёт байты через Bot API, токен в клиент не уезжает. */
export function trackerAvatarUrl(initData: string, userId: number): string {
  if (!api || !userId) return "";
  return `${api}/api/miniapp/user-avatar?init_data=${encodeURIComponent(initData)}&user_id=${userId}`;
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
