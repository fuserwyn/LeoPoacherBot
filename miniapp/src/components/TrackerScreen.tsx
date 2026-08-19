import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  leoAutonomy,
  leoProposeTask,
  leoSprint,
  sprintApply,
  sprintGenerate,
  sprintIdeas,
  trackerAttachImage,
  trackerAuthors,
  trackerAutoQa,
  trackerAvatarUrl,
  trackerCancel,
  trackerCreate,
  trackerDelete,
  trackerList,
  trackerQa,
  trackerReschedule,
  trackerRunNow,
  trackerTask,
  type LeoAutonomy,
  type SprintFeature,
  type SprintIdea,
  type TrackerTask,
} from "../lib/trackerApi";
import { TaskImageEditor, type TaskImage } from "./TaskImageEditor";
import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import "./TrackerScreen.css";

type Props = {
  initData: string;
  showAlert: (text: string) => void;
};

/** Колонки разработчика и тестировщика — те же, что на доске MyVibeLab. */
const DEV_COLS = [
  { key: "todo", title: "Ожидает" },
  { key: "doing", title: "В работе" },
  { key: "review", title: "Review" },
  { key: "test", title: "Тест" },
  { key: "deploy", title: "Сборка" },
  { key: "done", title: "Выполнено" },
  { key: "canceled", title: "Отменено" },
];

const QA_COLS = [
  { key: "todo", title: "К тестированию" },
  { key: "doing", title: "В тестировании" },
  { key: "done", title: "Готово" },
];

const WHEN_PRESETS = [
  { value: "через 1 мин", label: "Сейчас" },
  { value: "завтра 4:20", label: "Завтра в 4:20" },
  { value: "custom", label: "Дата и время…" },
];

/** «2026-08-20T09:00» из datetime-local → «2026-08-20 09:00», как ждёт трекер. */
function whenFromPicker(value: string): string {
  return value.replace("T", " ");
}

/** «[Спринт 2] текст» → номер спринта отдельным бейджем, как в MyVibeLab. */
function parsePrompt(prompt: string): { sprint: number | null; text: string } {
  const raw = String(prompt || "").trim();
  const m = raw.match(/^\[Спринт\s+(\d+)\]\s*/i);
  if (m) return { sprint: Number(m[1]), text: raw.slice(m[0].length).trim() || raw };
  return { sprint: null, text: raw };
}

/** RFC3339 с сервера → «19.08 14:30» в часовом поясе читателя. Пусто — прочерк. */
function formatWhen(iso: string): string {
  const t = Date.parse(iso || "");
  if (!t) return "—";
  return new Date(t).toLocaleString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** Задачу придумал Лео, а админ одобрил. Тот же id пишет MyVibeLab (board_share.LEO_AUTHOR_ID). */
const LEO_AUTHOR_ID = -1;

/** Автор задачи: Лео, гость мини-аппа или сам MyVibeLab, если ставили оттуда. */
function authorLabel(task: TrackerTask, authors: Record<number, string>): string {
  const id = Number(task.author_id) || 0;
  if (id === LEO_AUTHOR_ID) return "Лео";
  if (!id) return "Из чата";
  return authors[id] || `id ${id}`;
}

/** Картинка автора: у Лео своя, у людей — из чата стаи, у задач из чата её нет. */
function authorAvatar(task: TrackerTask, initData: string): string {
  const id = Number(task.author_id) || 0;
  if (id === LEO_AUTHOR_ID) return LEO_AVATAR_URL;
  return id > 0 ? trackerAvatarUrl(initData, id) : "";
}

function plural(n: number, one: string, few: string, many: string): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return few;
  return many;
}

function cardClasses(t: TrackerTask, isQa: boolean): string {
  const cls = ["tracker-card"];
  if (isQa) {
    if ((t.qa_column || "todo") === "doing") cls.push("is-running");
  } else if (t.status === "running" || t.status === "reviewing") cls.push("is-running");
  else if (t.dev_column === "review") cls.push("is-review");
  else if (t.dev_column === "test") cls.push("is-qa");
  else if (t.dev_column === "deploy") cls.push("is-deploy");
  else if (t.status === "holding") cls.push("is-holding");
  if (t.error) cls.push("is-err");
  if (t.handed_to_qa && !isQa) cls.push("is-qa");
  return cls.join(" ");
}

function metaParts(t: TrackerTask, isQa: boolean): string[] {
  const parts: string[] = [];
  if (t.when) parts.push(t.when);
  if (t.repeat && t.repeat !== "разово") parts.push(t.repeat);
  if (!isQa && t.auto_review) parts.push("авто-ревью");
  if (t.manual_qa) parts.push("ручное QA");
  if (t.fast_track) parts.push("без ревью и теста");
  if (!isQa && t.handed_to_qa) parts.push("→ QA");
  if (t.error) parts.push("ошибка");
  return parts;
}

export function TrackerScreen({ initData, showAlert }: Props) {
  const [tab, setTab] = useState<"board" | "task" | "sprint">("board");
  /** author_id → как показать человека. Задачи знают только id. */
  const [authors, setAuthors] = useState<Record<number, string>>({});
  const [role, setRole] = useState<"developer" | "tester">("developer");
  const [tasks, setTasks] = useState<TrackerTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [prompt, setPrompt] = useState("");
  const [when, setWhen] = useState(WHEN_PRESETS[0].value);
  const [whenAt, setWhenAt] = useState("");
  const [image, setImage] = useState<TaskImage | null>(null);
  /** Тема для Лео; пусто — придумывает сам. */
  const [leoTopic, setLeoTopic] = useState("");
  /** Автономный режим: Лео сам придумывает спринты, пока админ не выключит. */
  const [autonomy, setAutonomy] = useState<LeoAutonomy | null>(null);
  const [autonomyDays, setAutonomyDays] = useState(1);
  const [autonomyBusy, setAutonomyBusy] = useState(false);
  /** Задача от Лео на утверждении: он предлагает — админ решает. */
  const [proposal, setProposal] = useState<{ reply: string; title: string; task: string } | null>(null);
  const [rejected, setRejected] = useState<string[]>([]);
  const [proposeBusy, setProposeBusy] = useState(false);
  const [editorFor, setEditorFor] = useState<"new" | number | null>(null);
  const [detail, setDetail] = useState<TrackerTask | null>(null);
  const [moveAt, setMoveAt] = useState("");

  const [hint, setHint] = useState("");
  const [sprintCount, setSprintCount] = useState(1);
  const [tasksPerSprint, setTasksPerSprint] = useState(5);
  const [ideas, setIdeas] = useState<SprintIdea[]>([]);
  const [pickedIdea, setPickedIdea] = useState<string>("");
  const [features, setFeatures] = useState<(SprintFeature & { _on: boolean })[]>([]);
  const [sprintBusy, setSprintBusy] = useState<"" | "ideas" | "plan" | "apply" | "leo">("");
  const [leoSprintReply, setLeoSprintReply] = useState("");

  const isQa = role === "tester";
  const detailRef = useRef<number | null>(null);
  detailRef.current = detail?.id ?? null;

  const load = useCallback(
    async (silent = false) => {
      try {
        const j = await trackerList(initData);
        const list = j.tasks ?? [];
        setTasks(list);
        const ids = Array.from(
          new Set(list.map((t) => Number(t.author_id) || 0).filter((id) => id > 0)),
        );
        if (ids.length > 0) {
          const people = await trackerAuthors(initData, ids);
          setAuthors((prev) => {
            const next = { ...prev };
            for (const p of people) {
              const nick = (p.username || "").replace(/^@+/, "");
              next[p.user_id] = p.display_name || (nick ? `@${nick}` : `id ${p.user_id}`);
            }
            for (const id of ids) if (!next[id]) next[id] = `id ${id}`;
            return next;
          });
        }
        const openId = detailRef.current;
        if (openId) {
          const fresh = (j.tasks ?? []).find((t) => t.id === openId);
          if (fresh) setDetail((prev) => (prev ? { ...prev, ...fresh } : prev));
        }
      } catch (e) {
        if (!silent) showAlert(e instanceof Error ? e.message : "Не удалось загрузить доску");
      } finally {
        setLoading(false);
      }
    },
    [initData, showAlert],
  );

  useEffect(() => {
    void load();
  }, [load]);

  // Живые статусы: агент работает прямо сейчас, и карточка должна это показывать.
  useEffect(() => {
    if (tab !== "board") return undefined;
    const id = window.setInterval(() => {
      if (!document.hidden) void load(true);
    }, 10000);
    return () => window.clearInterval(id);
  }, [tab, load]);

  const pool = useMemo(() => (isQa ? tasks.filter((t) => t.handed_to_qa) : tasks), [tasks, isQa]);
  const columns = isQa ? QA_COLS : DEV_COLS;
  const errCount = pool.filter((t) => t.error).length;
  const runningCount = pool.filter((t) =>
    isQa ? (t.qa_column || "todo") === "doing" : t.status === "running",
  ).length;

  const createTask = async () => {
    await createTaskWith(prompt.trim());
  };

  const createTaskWith = async (raw: string, opts?: { leo?: boolean }) => {
    const text = raw.trim();
    if (!text) {
      showAlert("Опиши задачу.");
      return;
    }
    if (when === "custom" && !whenAt) {
      showAlert("Выбери дату и время.");
      return;
    }
    setBusy(true);
    try {
      const res = await trackerCreate(initData, {
        when: when === "custom" ? whenFromPicker(whenAt) : when,
        prompt: text,
        // Задачу сочинил Лео — пусть на доске и стоит он, а не тот, кто одобрил.
        leo: opts?.leo,
      });
      if (image && res.id) {
        // Картинку трекер принимает только к существующей задаче, поэтому
        // прикладываем сразу после создания.
        await trackerAttachImage(initData, res.id, {
          data: image.data,
          filename: image.filename,
          mime: image.mime,
        });
      }
      setImage(null);
      setPrompt("");
      showAlert(`Задача поставлена на ${res.when || "ближайший запуск"}.`);
      await load();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось поставить задачу");
    } finally {
      setBusy(false);
    }
  };

  // Лео сам придумывает задачу. Чтобы не ходил по кругу, отдаём ему то, что
  // уже на доске, и то, что админ только что отклонил.
  const proposeFromLeo = async (extraReject?: string) => {
    setProposeBusy(true);
    try {
      const busy = [
        ...tasks.slice(0, 12).map((t) => parsePrompt(t.prompt).text.slice(0, 160)),
        ...rejected,
        ...(extraReject ? [extraReject] : []),
      ];
      const j = await leoProposeTask(initData, leoTopic.trim(), busy);
      setProposal(j);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Лео промолчал");
    } finally {
      setProposeBusy(false);
    }
  };

  // Статус автономного режима читаем при открытии вкладки: он живёт на сервере
  // и меняется сам (Лео отработал очередной спринт), а не только по нажатию.
  const loadAutonomy = useCallback(async () => {
    try {
      setAutonomy(await leoAutonomy(initData, { action: "status" }));
    } catch {
      // Режим доступен только настоящим админам — остальным просто не показываем блок.
      setAutonomy(null);
    }
  }, [initData]);

  useEffect(() => {
    if (tab === "task") void loadAutonomy();
  }, [tab, loadAutonomy]);

  const switchAutonomy = async (action: "start" | "stop") => {
    setAutonomyBusy(true);
    try {
      const next = await leoAutonomy(initData, {
        action,
        days: autonomyDays,
        every_hours: 4,
        tasks_per_run: 3,
      });
      setAutonomy(next);
      showAlert(
        action === "start"
          ? `Лео работает сам до ${formatWhen(next.active_until)}.`
          : "Лео больше не ставит задачи сам.",
      );
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не получилось");
    } finally {
      setAutonomyBusy(false);
    }
  };

  const approveProposal = async () => {
    if (!proposal?.task) return;
    setPrompt(proposal.task);
    setProposal(null);
    await createTaskWith(proposal.task, { leo: true });
  };

  const openTask = async (task: TrackerTask) => {
    setDetail(task);
    setMoveAt("");
    try {
      const j = await trackerTask(initData, task.id);
      if (j.task) setDetail(j.task);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось открыть задачу");
    }
  };

  const act = async (fn: () => Promise<unknown>, done: string) => {
    setBusy(true);
    try {
      await fn();
      showAlert(done);
      setDetail(null);
      await load();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не получилось");
    } finally {
      setBusy(false);
    }
  };

  const loadIdeas = async () => {
    const text = hint.trim();
    if (!text) {
      showAlert("Напиши тему спринта.");
      return;
    }
    setSprintBusy("ideas");
    setFeatures([]);
    try {
      const j = await sprintIdeas(initData, text);
      setIdeas(j.ideas ?? []);
      setPickedIdea(String(j.recommended_id || (j.ideas ?? [])[0]?.id || ""));
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось предложить идеи");
    } finally {
      setSprintBusy("");
    }
  };

  // Спринт от Лео: он же придумывает тему и нарезку. Задачи кладём в тот же
  // список, что и обычный генератор, — дальше «Поставить задачи в план».
  const askLeoSprint = async () => {
    setSprintBusy("leo");
    setLeoSprintReply("");
    try {
      const j = await leoSprint(initData, hint.trim());
      setLeoSprintReply(j.reply);
      if (j.theme) setHint(j.theme);
      setIdeas([]);
      setFeatures(
        j.tasks.map((t, i) => ({
          title: t.split(/[.!?]/, 1)[0].slice(0, 80) || `Задача ${i + 1}`,
          prompt: t,
          sprint: 1,
          _on: true,
        })),
      );
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Лео промолчал");
    } finally {
      setSprintBusy("");
    }
  };

  const buildPlan = async () => {
    const idea = ideas.find((i) => String(i.id) === pickedIdea) ?? ideas[0];
    if (!idea) {
      showAlert("Сначала выбери идею.");
      return;
    }
    setSprintBusy("plan");
    try {
      const j = await sprintGenerate(initData, {
        hint: hint.trim(),
        idea,
        sprint_count: sprintCount,
        tasks_per_sprint: tasksPerSprint,
      });
      setFeatures((j.features ?? []).map((f) => ({ ...f, _on: true })));
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось собрать план");
    } finally {
      setSprintBusy("");
    }
  };

  const applyPlan = async () => {
    const picked = features.filter((f) => f._on);
    if (!picked.length) {
      showAlert("Отметь хотя бы одну задачу.");
      return;
    }
    setSprintBusy("apply");
    try {
      const j = await sprintApply(initData, {
        features: picked.map(({ _on, ...rest }) => rest),
        sprint_count: sprintCount,
        tasks_per_sprint: tasksPerSprint,
      });
      showAlert(`Поставлено задач: ${j.created ?? picked.length}.`);
      setFeatures([]);
      setIdeas([]);
      setHint("");
      setTab("board");
      await load();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось поставить задачи");
    } finally {
      setSprintBusy("");
    }
  };

  return (
    <div className="tracker">
      <div className="tracker__tabs" role="tablist">
        <button
          type="button"
          role="tab"
          aria-selected={tab === "board"}
          className={tab === "board" ? "on" : ""}
          onClick={() => setTab("board")}
        >
          Доска
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "task"}
          className={tab === "task" ? "on" : ""}
          onClick={() => setTab("task")}
        >
          Задача
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "sprint"}
          className={tab === "sprint" ? "on" : ""}
          onClick={() => setTab("sprint")}
        >
          Спринты
        </button>
      </div>

      {tab === "board" ? (
        <>
          <div className="tracker__toolbar">
            <div className="tracker__roles">
              <button
                type="button"
                className={role === "developer" ? "on" : ""}
                onClick={() => setRole("developer")}
              >
                Разработчик
              </button>
              <button
                type="button"
                className={role === "tester" ? "on" : ""}
                onClick={() => setRole("tester")}
              >
                Тестировщик
              </button>
            </div>
            <button type="button" className="tracker__refresh" onClick={() => void load()}>
              Обновить
            </button>
          </div>

          <div className="tracker__summary">
            <span className="tracker-stat">
              <strong>{pool.length}</strong> {plural(pool.length, "задача", "задачи", "задач")}
            </span>
            {runningCount > 0 ? (
              <span className="tracker-stat tracker-stat--active">
                <strong>{runningCount}</strong> в работе
              </span>
            ) : null}
            {errCount > 0 ? (
              <span className="tracker-stat tracker-stat--err">
                <strong>{errCount}</strong> {plural(errCount, "ошибка", "ошибки", "ошибок")}
              </span>
            ) : null}
          </div>

          <p className="tracker__hint">
            {isQa
              ? "После сдачи агент сам прогоняет AI-тест. Можно принять, вернуть в работу или запустить тест ещё раз."
              : "Ожидает → в работе → Review → тест → выполнено."}
          </p>

          {loading ? (
            <p className="tracker__muted">Загружаю доску…</p>
          ) : (
            <div className="tracker__cols">
              {columns.map((col) => {
                const items = pool.filter((t) =>
                  isQa ? (t.qa_column || "todo") === col.key : (t.dev_column || "todo") === col.key,
                );
                return (
                  <div className="tracker-col" data-col={col.key} key={col.key}>
                    <div className="tracker-col__head">
                      <span className="tracker-col__title">{col.title}</span>
                      <span className="tracker-col__count">{items.length}</span>
                    </div>
                    <div className="tracker-col__list">
                      {items.length === 0 ? (
                        <div className="tracker-empty">
                          <span className="tracker-empty__icon" aria-hidden>
                            ∅
                          </span>
                          пусто
                        </div>
                      ) : (
                        items.map((t) => (
                          <TaskCard
                            key={t.id}
                            task={t}
                            isQa={isQa}
                            author={authorLabel(t, authors)}
                            avatar={authorAvatar(t, initData)}
                            onOpen={() => void openTask(t)}
                          />
                        ))
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </>
      ) : tab === "task" ? (
        <div className="tracker__task">
          <p className="tracker__hint">
            Опиши, что сделать, когда запускать и приложи картинку, если так понятнее.
          </p>

          {autonomy ? (
            <div className="tracker__leo tracker__auto">
              <div className="tracker__leo-head">
                <span aria-hidden>🤖</span>
                <b>Лео ведёт продукт сам</b>
                <span className={`tracker__auto-state${autonomy.active ? " is-on" : ""}`}>
                  {autonomy.active ? "включено" : "выключено"}
                </span>
              </div>
              <p className="tracker__hint">
                Раз в {autonomy.every_hours} ч Лео придумывает спринт своим голосом и сам ставит
                задачи — по {autonomy.tasks_per_run} за прогон. Выполняет их тот же агент, на доске
                у них его аватарка.
              </p>
              {autonomy.active ? (
                <p className="tracker__auto-facts">
                  До {formatWhen(autonomy.active_until)} · следующий спринт{" "}
                  {formatWhen(autonomy.next_run_at)}
                </p>
              ) : null}
              {autonomy.last_note ? (
                <p className="tracker__auto-facts">
                  Прошлый раз ({formatWhen(autonomy.last_run_at)}): {autonomy.last_note}
                </p>
              ) : null}
              <div className="tracker__new-row">
                <select
                  value={autonomyDays}
                  onChange={(e) => setAutonomyDays(Number(e.target.value))}
                  aria-label="Сколько дней Лео работает сам"
                >
                  {[1, 2, 3, 5, 7, 14]
                    .filter((d) => d <= (autonomy.max_days || 14))
                    .map((d) => (
                      <option key={d} value={d}>
                        {d} {plural(d, "день", "дня", "дней")}
                      </option>
                    ))}
                </select>
                <button
                  type="button"
                  className="tracker__primary"
                  disabled={autonomyBusy}
                  onClick={() => void switchAutonomy("start")}
                >
                  {autonomy.active ? "Продлить" : "Пусть работает сам"}
                </button>
                {autonomy.active ? (
                  <button
                    type="button"
                    className="tracker__attach"
                    disabled={autonomyBusy}
                    onClick={() => void switchAutonomy("stop")}
                  >
                    Остановить
                  </button>
                ) : null}
              </div>
            </div>
          ) : null}

          <div className="tracker__leo">
            <div className="tracker__leo-head">
              <span aria-hidden>🐆</span>
              <b>Задача от Лео</b>
            </div>
            <p className="tracker__hint">
              Напиши тему — или оставь пусто, тогда Лео решит сам. Дальше ты выбираешь: берём в работу или пусть
              думает ещё.
            </p>
            <div className="tracker__new-row">
              <input
                value={leoTopic}
                onChange={(e) => setLeoTopic(e.target.value)}
                placeholder="Тема (необязательно): например, удержание новичков"
              />
            </div>
            {proposal ? (
              <>
                <p className="tracker__leo-reply">{proposal.reply}</p>
                {proposal.title ? <p className="tracker__leo-title">{proposal.title}</p> : null}
                <p className="tracker__leo-task">{proposal.task}</p>
                <div className="tracker__new-row">
                  <select value={when} onChange={(e) => setWhen(e.target.value)}>
                    {WHEN_PRESETS.map((p) => (
                      <option key={p.value} value={p.value}>
                        {p.label}
                      </option>
                    ))}
                  </select>
                  {when === "custom" ? (
                    <input
                      type="datetime-local"
                      className="tracker__at"
                      value={whenAt}
                      onChange={(e) => setWhenAt(e.target.value)}
                    />
                  ) : null}
                  <button
                    type="button"
                    className="tracker__primary"
                    disabled={busy || proposeBusy || !proposal.task}
                    onClick={() => void approveProposal()}
                  >
                    Одобрить и поставить
                  </button>
                  <button
                    type="button"
                    className="tracker__attach"
                    disabled={proposeBusy}
                    onClick={() => {
                      const reject = proposal.title || proposal.task;
                      setRejected((prev) => [...prev, reject].slice(-10));
                      setProposal(null);
                      void proposeFromLeo(reject);
                    }}
                  >
                    Не то, давай другую
                  </button>
                </div>
              </>
            ) : (
              <button
                type="button"
                className="tracker__attach"
                disabled={proposeBusy}
                onClick={() => void proposeFromLeo()}
              >
                {proposeBusy ? "Лео придумывает…" : "Пусть Лео придумает"}
              </button>
            )}
          </div>

          <div className="tracker__new">
              <textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder="Что сделать? Например: в админке выгрузка отчёта по тренировкам за неделю"
              />
              {image ? (
                <div className="tracker__preview">
                  <img src={image.preview} alt="" />
                  <button type="button" onClick={() => setImage(null)} aria-label="Убрать картинку">
                    ✕
                  </button>
                </div>
              ) : null}
              <div className="tracker__new-row">
                <select value={when} onChange={(e) => setWhen(e.target.value)}>
                  {WHEN_PRESETS.map((p) => (
                    <option key={p.value} value={p.value}>
                      {p.label}
                    </option>
                  ))}
                </select>
                {when === "custom" ? (
                  <input
                    type="datetime-local"
                    className="tracker__at"
                    value={whenAt}
                    onChange={(e) => setWhenAt(e.target.value)}
                  />
                ) : null}
                <button type="button" className="tracker__attach" onClick={() => setEditorFor("new")}>
                  {image ? "🖼 Заменить" : "🖼 Картинка"}
                </button>
                <button type="button" className="tracker__primary" disabled={busy} onClick={() => void createTask()}>
                  Поставить задачу
                </button>
              </div>
            </div>
        </div>
      ) : (
        <div className="tracker__sprint">
          <p className="tracker__hint">
            Опиши тему спринта — агент разберёт проект, предложит идеи и нарежет задачи с требованиями и критериями
            приёмки.
          </p>
          <textarea
            value={hint}
            onChange={(e) => setHint(e.target.value)}
            placeholder="Например: снизить отток участников стаи"
          />
          <div className="tracker__new-row">
            <label className="tracker__num">
              Спринтов
              <input
                type="number"
                min={1}
                max={8}
                value={sprintCount}
                onChange={(e) => setSprintCount(Math.min(8, Math.max(1, Number(e.target.value) || 1)))}
              />
            </label>
            <label className="tracker__num">
              Задач в спринте
              <input
                type="number"
                min={1}
                max={12}
                value={tasksPerSprint}
                onChange={(e) => setTasksPerSprint(Math.min(12, Math.max(1, Number(e.target.value) || 1)))}
              />
            </label>
            <button
              type="button"
              className="tracker__primary"
              disabled={sprintBusy !== ""}
              onClick={() => void loadIdeas()}
            >
              {sprintBusy === "ideas" ? "Думаю…" : "Предложить идеи"}
            </button>
            <button
              type="button"
              className="tracker__attach"
              disabled={sprintBusy !== ""}
              onClick={() => void askLeoSprint()}
            >
              {sprintBusy === "leo" ? "Лео думает…" : "🐆 Спринт от Лео"}
            </button>
          </div>
          {leoSprintReply ? <p className="tracker__leo-reply">🐆 {leoSprintReply}</p> : null}

          {ideas.length > 0 ? (
            <div className="tracker__ideas">
              <h3 className="tracker__subtitle">Идеи</h3>
              {ideas.map((idea, i) => {
                const id = String(idea.id ?? i);
                const on = pickedIdea === id;
                return (
                  <button
                    type="button"
                    key={id}
                    className={`tracker-idea${on ? " on" : ""}`}
                    onClick={() => setPickedIdea(id)}
                  >
                    <b>{String(idea.title || idea.name || `Идея ${i + 1}`)}</b>
                    <small>{String(idea.summary || idea.description || "")}</small>
                  </button>
                );
              })}
              <button
                type="button"
                className="tracker__primary tracker__primary--block"
                disabled={sprintBusy !== ""}
                onClick={() => void buildPlan()}
              >
                {sprintBusy === "plan" ? "Собираю план…" : "Собрать план и задачи"}
              </button>
            </div>
          ) : null}

          {features.length > 0 ? (
            <div className="tracker__feats">
              <h3 className="tracker__subtitle">Задачи спринта</h3>
              {features.map((f, i) => (
                <label className="tracker-feat" key={`${f.title}-${i}`}>
                  <input
                    type="checkbox"
                    checked={f._on}
                    onChange={(e) =>
                      setFeatures((prev) =>
                        prev.map((item, idx) => (idx === i ? { ...item, _on: e.target.checked } : item)),
                      )
                    }
                  />
                  <span>
                    <b>
                      {f.sprint ? <span className="tracker-badge tracker-badge--sprint">Спринт {f.sprint}</span> : null}
                      {String(f.title || "")}
                    </b>
                    <small>{String(f.prompt || "").slice(0, 220)}</small>
                  </span>
                </label>
              ))}
              <button
                type="button"
                className="tracker__primary tracker__primary--block"
                disabled={sprintBusy !== ""}
                onClick={() => void applyPlan()}
              >
                {sprintBusy === "apply" ? "Ставлю…" : "Поставить задачи в план"}
              </button>
            </div>
          ) : null}
        </div>
      )}

      {detail ? (
        <div className="tracker-modal" role="dialog" aria-modal="true">
          <div className="tracker-modal__box">
            <div className="tracker-modal__head">
              <span className="tracker-card__id">#{detail.id}</span>
              <span className="tracker-modal__status">
                {detail.status_icon} {detail.status_label}
              </span>
              <button type="button" className="tracker-modal__close" onClick={() => setDetail(null)} aria-label="Закрыть">
                ✕
              </button>
            </div>
            <div className="tracker-card__author tracker-modal__author">
              {authorAvatar(detail, initData) ? (
                <img src={authorAvatar(detail, initData)} alt="" loading="lazy" />
              ) : (
                <span className="tracker-card__author-dot">🤖</span>
              )}
              <span>Поставил: {authorLabel(detail, authors)}</span>
            </div>
            <p className="tracker-modal__prompt">{parsePrompt(detail.prompt).text}</p>
            <div className="tracker-card__meta">
              {metaParts(detail, isQa).map((p) => (
                <span key={p}>{p}</span>
              ))}
            </div>
            {detail.steps && detail.steps.length > 0 ? (
              <pre className="tracker-modal__log">{detail.steps.slice(-40).join("\n")}</pre>
            ) : null}
            {detail.result ? <pre className="tracker-modal__log">{detail.result}</pre> : null}
            {detail.error ? <pre className="tracker-modal__log tracker-modal__log--err">{detail.error}</pre> : null}
            {/* Перенос запуска. Он же возвращает завершённую или отменённую
                задачу в «Ожидает»: время в будущем — и она снова в очереди.
                Выполняющуюся двигать нельзя, сначала останови. */}
            {detail.status !== "running" && detail.status !== "reviewing" ? (
              <div className="tracker-modal__move">
                <label>
                  {["canceled", "done", "error"].includes(detail.status) ? "Вернуть в ожидание на" : "Запуск"}
                  <input type="datetime-local" value={moveAt} onChange={(e) => setMoveAt(e.target.value)} />
                </label>
                <button
                  type="button"
                  disabled={busy || !moveAt}
                  onClick={() =>
                    void act(
                      () => trackerReschedule(initData, detail.id, moveAt.replace("T", " ")),
                      "Время запуска изменено.",
                    )
                  }
                >
                  Перенести
                </button>
              </div>
            ) : null}

            <div className="tracker-modal__actions">
              <button type="button" disabled={busy} onClick={() => setEditorFor(detail.id)}>
                🖼 Картинка
              </button>
              {["canceled", "done", "error"].includes(detail.status) ? (
                <button
                  type="button"
                  className="tracker-modal__accent"
                  disabled={busy}
                  onClick={() => void act(() => trackerRunNow(initData, detail.id), "Задача снова в работе.")}
                >
                  Вернуть в работу
                </button>
              ) : null}
              {isQa && detail.handed_to_qa ? (
                <>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => void act(() => trackerQa(initData, detail.id, "pass"), "Задача принята.")}
                  >
                    Принять
                  </button>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => void act(() => trackerQa(initData, detail.id, "fail"), "Вернул разработчику.")}
                  >
                    Вернуть
                  </button>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => void act(() => trackerAutoQa(initData, detail.id), "AI-тест запущен.")}
                  >
                    AI-тест
                  </button>
                </>
              ) : null}
              {detail.active ? (
                <button
                  type="button"
                  disabled={busy}
                  onClick={() =>
                    void act(
                      () => trackerCancel(initData, detail.id),
                      detail.status === "running" || detail.status === "reviewing"
                        ? "Задача остановлена."
                        : "Задача отменена.",
                    )
                  }
                >
                  {detail.status === "running" || detail.status === "reviewing" ? "Остановить" : "Отменить"}
                </button>
              ) : null}
              {detail.can_delete ? (
                <button
                  type="button"
                  className="tracker-modal__danger"
                  disabled={busy}
                  onClick={() => void act(() => trackerDelete(initData, detail.id), "Задача удалена.")}
                >
                  Удалить
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}

      {editorFor !== null ? (
        <TaskImageEditor
          onCancel={() => setEditorFor(null)}
          onDone={(img) => {
            const target = editorFor;
            setEditorFor(null);
            if (target === "new") {
              setImage(img);
              return;
            }
            if (typeof target === "number") {
              void act(
                () => trackerAttachImage(initData, target, { data: img.data, filename: img.filename, mime: img.mime }),
                "Картинка приложена.",
              );
            }
          }}
        />
      ) : null}
    </div>
  );
}

function TaskCard({
  task,
  isQa,
  author,
  avatar,
  onOpen,
}: {
  task: TrackerTask;
  isQa: boolean;
  author: string;
  avatar: string;
  onOpen: () => void;
}) {
  const parsed = parsePrompt(task.prompt);
  const statusText = isQa
    ? `${task.qa_icon || "🧪"} ${task.qa_label || "QA"}`
    : `${task.status_icon || ""} ${task.status_label || ""}`;
  const steps = Array.isArray(task.steps) ? task.steps : [];
  const live = (task.live_step || "").trim() || (steps.length ? String(steps[steps.length - 1] || "").trim() : "");
  const showLive = task.status === "running" || task.status === "reviewing" || task.steps_running;
  const meta = metaParts(task, isQa);
  return (
    <div className={cardClasses(task, isQa)} role="button" tabIndex={0} onClick={onOpen}>
      <div className="tracker-card__head">
        <span className="tracker-card__id">#{task.id}</span>
        <span className="tracker-card__status">{statusText}</span>
      </div>
      <div className="tracker-card__author">
        {avatar ? <img src={avatar} alt="" loading="lazy" /> : <span className="tracker-card__author-dot">🤖</span>}
        <span>{author}</span>
      </div>
      <div className="tracker-card__text">{parsed.text || task.prompt}</div>
      {parsed.sprint || task.kind === "deploy_fix" || task.manual_qa || task.fast_track || task.has_attachments ? (
        <div className="tracker-card__badges">
          {parsed.sprint ? <span className="tracker-badge tracker-badge--sprint">Спринт {parsed.sprint}</span> : null}
          {task.kind === "deploy_fix" ? (
            <span className="tracker-badge tracker-badge--deploy">авто-фикс деплоя</span>
          ) : null}
          {task.manual_qa ? <span className="tracker-badge tracker-badge--qa">ручное QA</span> : null}
          {task.fast_track ? <span className="tracker-badge tracker-badge--deploy">⚡ быстрый выпуск</span> : null}
          {task.has_attachments ? (
            <span className="tracker-badge tracker-badge--photo">📷 {task.attachments_count || 1}</span>
          ) : null}
        </div>
      ) : null}
      {showLive ? (
        <div className="tracker-card__live">{live || "⏳ выполняется…"}</div>
      ) : null}
      {meta.length > 0 ? (
        <div className="tracker-card__meta">
          {meta.map((p) => (
            <span key={p}>{p}</span>
          ))}
        </div>
      ) : null}
    </div>
  );
}
