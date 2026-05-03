import { useCallback, useEffect, useMemo, useState } from "react";
import { ActivityCard, type ActivityCardProps } from "./ActivityCard";
import { PackGroupChatPanel } from "./PackGroupChatPanel";
import {
  dtoToCard,
  HEALTHY_FEED_EMOJIS,
  mergePackFeedReactions,
  mergeTrainingFeedReactions,
  resolveFeedAvatarUrl,
  type PackFeedItemDTO,
  type PackFeedThreadReplyDTO,
} from "../lib/packFeed";
import { timeAgoFromISO } from "../lib/timeAgo";
import { streakStreakAriaLabel } from "../lib/streakLabel";
import { formatInactivityRemovalHint } from "../lib/formatInactivityRemoval";
import {
  sortWorkoutCategoryIds,
  trainingDoneMatchesAnyCategory,
  WORKOUT_CATEGORY_OPTIONS_ALPHABETICAL,
  type WorkoutCategoryId,
} from "../lib/workoutCategories";
import "./FeedScreen.css";

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

type Props = {
  name: string;
  /** Уровень пользователя (ленточная шапка), считается в App от кубков (XP). */
  level: number;
  streak: number;
  userId: number;
  initData: string;
  inTelegram: boolean;
  showAlert: (m: string) => void;
  refreshToken?: number;
  /** ISO: возможный кик за неактивность (из profile/load). */
  inactivityRemovalAt?: string | null;
};

type Sub = "activity" | "room";

function mockFallback(_name: string, streak: number): ActivityCardProps[] {
  return [
    {
      avatar: "💬",
      name: "Стая",
      streak: Math.max(streak, 0),
      timeAgo: "сейчас",
      emoji: "ℹ️",
      activity: "Нет API",
      details: "VITE_MINIAPP_API_URL",
      comment: "Включи URL бота в билде, чтобы тянуть реальные отчёты из чата стаи.",
    },
  ];
}

export function FeedScreen({
  name,
  level,
  streak,
  userId,
  initData,
  inTelegram,
  showAlert,
  refreshToken = 0,
  inactivityRemovalAt = null,
}: Props) {
  const [sub, setSub] = useState<Sub>("activity");

  const [feedItems, setFeedItems] = useState<PackFeedItemDTO[]>([]);
  const [useMockFeed, setUseMockFeed] = useState(false);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [threadDrafts, setThreadDrafts] = useState<Record<number, string>>({});
  const [threadPosting, setThreadPosting] = useState<Record<number, boolean>>({});
  const [threadReplyDeleting, setThreadReplyDeleting] = useState<Record<number, boolean>>({});
  /** Ответ на конкретное сообщение треда (reply_to_id) — ключ id отчёта user_messages. */
  const [threadReplyTargets, setThreadReplyTargets] = useState<
    Record<number, { replyToThreadId: number; authorLabel: string; excerpt: string } | undefined>
  >({});
  const [feedOnlyMine, setFeedOnlyMine] = useState(false);
  /** Мультивыбор типов тренировок (пусто = «все типы»). */
  const [feedCategoryIds, setFeedCategoryIds] = useState<WorkoutCategoryId[]>([]);

  const categoryFilterSet = useMemo(() => new Set(feedCategoryIds), [feedCategoryIds]);

  const visibleFeedItems = useMemo(() => {
    return feedItems.filter((it) => {
      if (it.type === "sick_leave") return false;
      if (feedOnlyMine && !it.is_you) return false;
      if (feedCategoryIds.length > 0) {
        if (it.type !== "training_done") return false;
        if (!trainingDoneMatchesAnyCategory(it.text, categoryFilterSet)) return false;
      }
      return true;
    });
  }, [feedItems, feedOnlyMine, feedCategoryIds, categoryFilterSet]);

  const hapticLight = useCallback(() => {
    window.Telegram?.WebApp?.HapticFeedback?.impactOccurred?.("light");
  }, []);

  const toggleFeedCategory = useCallback(
    (id: WorkoutCategoryId) => {
      hapticLight();
      setFeedCategoryIds((prev) => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return sortWorkoutCategoryIds([...next]);
      });
    },
    [hapticLight],
  );

  const clearFeedCategories = useCallback(() => {
    hapticLight();
    setFeedCategoryIds([]);
  }, [hapticLight]);

  const load = useCallback(async () => {
    if (!apiBase || !inTelegram || !initData) {
      setLoading(false);
      setUseMockFeed(true);
      setFeedItems([]);
      setErr(null);
      return;
    }
    setErr(null);
    setLoading(true);
    setUseMockFeed(false);
    try {
      const res = await fetch(`${apiBase}/api/miniapp/feed`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData }),
      });
      const j = (await res.json().catch(() => ({}))) as { ok?: boolean; items?: PackFeedItemDTO[]; error?: string };
      if (!res.ok) {
        if (res.status === 403) {
          setErr("Нет доступа к ленте стаи: нужна подписка/участие в группе, как в боте.");
          setFeedItems([]);
          return;
        }
        setErr(j.error ?? `Ошибка ${res.status}`);
        setFeedItems([]);
        return;
      }
      setFeedItems(j.items ?? []);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Сеть");
      setFeedItems([]);
    } finally {
      setLoading(false);
    }
  }, [inTelegram, initData]);

  const postTrainingReact = useCallback(
    async (userMessageId: number, emoji: string) => {
      if (!apiBase || !initData) return;
      try {
        const res = await fetch(`${apiBase}/api/miniapp/feed/training/react`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, user_message_id: userMessageId, emoji }),
        });
        const j = (await res.json().catch(() => ({}))) as { error?: string };
        if (!res.ok) {
          showAlert(j.error === "invalid_emoji" ? "Такую реакцию нельзя" : j.error ?? `Ошибка ${res.status}`);
          return;
        }
        await load();
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      }
    },
    [apiBase, initData, load, showAlert],
  );

  const postTrainingThread = useCallback(
    async (userMessageId: number, text: string, replyToThreadId?: number) => {
      const t = text.trim();
      if (!t) {
        showAlert("Введи текст комментария.");
        return;
      }
      if (!apiBase || !initData) return;
      setThreadPosting((p) => ({ ...p, [userMessageId]: true }));
      try {
        const res = await fetch(`${apiBase}/api/miniapp/feed/training/thread`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            init_data: initData,
            user_message_id: userMessageId,
            text: t,
            reply_to_id: replyToThreadId ?? 0,
          }),
        });
        const j = (await res.json().catch(() => ({}))) as {
          error?: string;
          thread?: PackFeedThreadReplyDTO[];
        };
        if (!res.ok) {
          const errMap: Record<string, string> = {
            empty_text: "Пустой комментарий",
            text_too_long: "Слишком длинно",
            not_found: "Запись не найдена (обнови ленту)",
            forbidden: "Нет доступа",
            chat_mismatch: "Открой мини-апп из чата стаи",
            thread_error: "Сервер не сохранил комментарий (таблица миграции или БД)",
            invalid_reply: "Не удалось ответить на это сообщение (обнови ленту)",
          };
          showAlert(errMap[j.error ?? ""] ?? j.error ?? `Ошибка ${res.status}`);
          return;
        }
        const postedThread = j.thread;
        setThreadDrafts((d) => ({ ...d, [userMessageId]: "" }));
        setThreadReplyTargets((r) => ({ ...r, [userMessageId]: undefined }));
        await load();
        if (Array.isArray(postedThread)) {
          setFeedItems((prev) =>
            prev.map((it) => (it.id === userMessageId ? { ...it, thread: postedThread } : it)),
          );
        }
        if (replyToThreadId) {
          window.setTimeout(() => void load(), 5000);
        }
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      } finally {
        setThreadPosting((p) => ({ ...p, [userMessageId]: false }));
      }
    },
    [apiBase, initData, load, showAlert],
  );

  const deleteTrainingThreadReply = useCallback(
    async (trainingUserMessageId: number, threadReplyId: number) => {
      if (!apiBase || !initData) return;
      setThreadReplyDeleting((p) => ({ ...p, [threadReplyId]: true }));
      try {
        const res = await fetch(`${apiBase}/api/miniapp/feed/training/thread/delete`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, thread_reply_id: threadReplyId }),
        });
        const j = (await res.json().catch(() => ({}))) as {
          error?: string;
          thread?: PackFeedThreadReplyDTO[];
        };
        if (!res.ok) {
          const errMap: Record<string, string> = {
            not_found: "Комментарий не найден или уже удалён",
            forbidden: "Нет доступа",
            chat_mismatch: "Открой мини-апп из чата стаи",
            thread_delete_error: "Не удалось удалить",
          };
          showAlert(errMap[j.error ?? ""] ?? j.error ?? `Ошибка ${res.status}`);
          return;
        }
        const updated = j.thread;
        await load();
        if (Array.isArray(updated)) {
          setFeedItems((prev) =>
            prev.map((it) => (it.id === trainingUserMessageId ? { ...it, thread: updated } : it)),
          );
        }
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      } finally {
        setThreadReplyDeleting((p) => ({ ...p, [threadReplyId]: false }));
      }
    },
    [apiBase, initData, load, showAlert],
  );

  const toggleTrainingThreadLike = useCallback(
    async (trainingUserMessageId: number, threadReplyId: number) => {
      if (!apiBase || !initData) return;
      try {
        const res = await fetch(`${apiBase}/api/miniapp/feed/training/thread/like`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, thread_reply_id: threadReplyId }),
        });
        const j = (await res.json().catch(() => ({}))) as { error?: string; thread?: PackFeedThreadReplyDTO[] };
        if (!res.ok) {
          showAlert(j.error ?? `Ошибка ${res.status}`);
          return;
        }
        const updated = j.thread;
        await load();
        if (Array.isArray(updated)) {
          setFeedItems((prev) =>
            prev.map((it) => (it.id === trainingUserMessageId ? { ...it, thread: updated } : it)),
          );
        }
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      }
    },
    [apiBase, initData, load, showAlert],
  );

  useEffect(() => {
    void load();
  }, [load, refreshToken]);

  const removalHint = formatInactivityRemovalHint(inactivityRemovalAt ?? undefined);

  return (
    <div className="feed">
      <div className="feed__sticky">
        <header className="feed__header">
          <div
            className="feed__level"
            aria-label={`Уровень ${level}`}
            title={`Уровень ${level} (по кубкам в профиле)`}
          >
            <span className="feed__level-k">Ур.</span>
            <span className="feed__level-v">{level}</span>
          </div>
          {removalHint ? (
            <div
              className="feed__kick-hint"
              title="Без отчёта #training_done — возможное исключение из стаи (МСК)"
              aria-label={`Возможное удаление за неактивность: ${removalHint}`}
            >
              до {removalHint}
            </div>
          ) : null}
          <div className="feed__streak" aria-label={streakStreakAriaLabel(streak)} title={streakStreakAriaLabel(streak)}>
            <span className="feed__streak-emoji" aria-hidden>
              🔥
            </span>
            <span className="feed__streak-row">
              <span className="feed__streak-word">Стрик</span>
              <span className="feed__streak-num">{streak}</span>
            </span>
          </div>
        </header>
        <div className="feed__subtabs" role="tablist" aria-label="Стая">
          <button
            type="button"
            className={`feed__subtab ${sub === "activity" ? "is-active" : ""}`}
            onClick={() => setSub("activity")}
            role="tab"
            aria-selected={sub === "activity"}
          >
            Лента
          </button>
          <button
            type="button"
            className={`feed__subtab ${sub === "room" ? "is-active" : ""}`}
            onClick={() => setSub("room")}
            role="tab"
            aria-selected={sub === "room"}
          >
            Чат
          </button>
        </div>
      </div>
      {sub === "room" && (
        <PackGroupChatPanel
          initData={initData}
          inTelegram={inTelegram}
          meId={userId}
          showAlert={showAlert}
          onHaptic={() => {
            const w = window.Telegram?.WebApp;
            w?.HapticFeedback?.impactOccurred?.("light");
          }}
        />
      )}
      {sub === "activity" && (
        <>
          <div className="feed__filters" aria-label="Фильтры ленты">
            <div className="feed__filter-scope" role="group" aria-label="Чьи отчёты">
              <button
                type="button"
                className={`feed__filter-pill${!feedOnlyMine ? " is-active" : ""}`}
                onClick={() => setFeedOnlyMine(false)}
              >
                Все
              </button>
              <button
                type="button"
                className={`feed__filter-pill${feedOnlyMine ? " is-active" : ""}`}
                onClick={() => setFeedOnlyMine(true)}
              >
                Мои тренировки
              </button>
            </div>
            <div className="feed__filter-cats" role="group" aria-label="Тип тренировки">
              <button
                type="button"
                className={`feed__filter-chip${feedCategoryIds.length === 0 ? " is-active" : ""}`}
                onClick={() => void clearFeedCategories()}
              >
                Все типы
              </button>
              {WORKOUT_CATEGORY_OPTIONS_ALPHABETICAL.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  className={`feed__filter-chip${feedCategoryIds.includes(c.id) ? " is-active" : ""}`}
                  onClick={() => toggleFeedCategory(c.id)}
                  title={c.label}
                >
                  <span className="feed__filter-chip-emoji" aria-hidden>
                    {c.emoji}
                  </span>
                  <span className="feed__filter-chip-label">{c.label}</span>
                </button>
              ))}
            </div>
          </div>
          <h2 className="section-title">Кто что постит</h2>
          {err && <p className="feed__err">{err}</p>}
          {loading && <p className="feed__load muted">Загрузка…</p>}
          <div className="feed__list">
            {!loading && !useMockFeed && feedItems.length === 0 && !err && (
              <p className="feed__empty muted">Пока нет отчётов в базе (или нет MONETIZED_CHAT_ID).</p>
            )}
            {!loading &&
              !useMockFeed &&
              feedItems.length > 0 &&
              visibleFeedItems.length === 0 &&
              !err && (
                <p className="feed__empty muted">
                  Ничего не подходит под фильтры — выбери другой тип или «Все типы» / «Все».
                </p>
              )}
            {useMockFeed &&
              mockFallback(name, streak).map((c, i) => (
                <div key={`mock-${i}`} className="feed__card-slot feed__card-slot--them">
                  <ActivityCard {...c} />
                </div>
              ))}
            {!useMockFeed &&
              visibleFeedItems.map((it) => {
                const base = dtoToCard(it);
                const isLeoSystemFeed =
                  it.type === "pack_join" ||
                  it.type === "pack_rejoin" ||
                  it.type === "daily_wisdom" ||
                  it.type === "pack_removed" ||
                  it.type === "inactive_notice";
                const slotClass = `feed__card-slot${it.is_you && !isLeoSystemFeed ? " feed__card-slot--mine" : " feed__card-slot--them"}`;
                if (it.type !== "training_done" && it.type !== "healthy") {
                  return (
                    <div key={it.id} className={slotClass}>
                      <ActivityCard {...base} />
                    </div>
                  );
                }
                const threadReplies = (it.thread ?? []).map((tr) => {
                  const rq =
                    typeof tr.reply_to_id === "number" &&
                    tr.reply_to_id > 0 &&
                    ((tr.reply_to_text || "").trim() !== "" || (tr.reply_to_username || "").trim() !== "")
                      ? {
                          author: tr.reply_to_is_leo
                            ? "Лео"
                            : (tr.reply_to_username || "").trim() || `Участник ${tr.user_id}`,
                          text: (tr.reply_to_text || "").trim(),
                          isLeo: Boolean(tr.reply_to_is_leo),
                        }
                      : undefined;
                  return {
                    id: tr.id,
                    author: (tr.username || "").trim() || `Участник ${tr.user_id}`,
                    text: tr.text,
                    timeAgo: timeAgoFromISO(tr.created_at),
                    isYou: tr.is_you,
                    isLeo: Boolean(tr.is_leo),
                    authorPhotoUrl: tr.author_photo_url?.trim()
                      ? resolveFeedAvatarUrl(tr.author_photo_url.trim())
                      : undefined,
                    replyTo: rq,
                    likeCount: tr.like_count ?? 0,
                    likeMe: Boolean(tr.like_me),
                  };
                });
                return (
                  <div key={it.id} className={slotClass}>
                    <ActivityCard
                      {...base}
                      reactions={
                        it.type === "training_done"
                          ? mergeTrainingFeedReactions(it.reactions)
                          : mergePackFeedReactions(HEALTHY_FEED_EMOJIS, it.reactions)
                      }
                      onReactionClick={(emoji) => void postTrainingReact(it.id, emoji)}
                      threadReplies={threadReplies}
                      onThreadReplyDelete={(replyId) => void deleteTrainingThreadReply(it.id, replyId)}
                      onThreadReplyLike={(replyId) => void toggleTrainingThreadLike(it.id, replyId)}
                      threadReplyDeleting={threadReplyDeleting}
                      threadReplyIntent={threadReplyTargets[it.id] ?? null}
                      onCancelThreadReplyIntent={() =>
                        setThreadReplyTargets((r) => ({ ...r, [it.id]: undefined }))
                      }
                      onThreadReplyIntent={(payload) =>
                        setThreadReplyTargets((r) => ({
                          ...r,
                          [it.id]: {
                            replyToThreadId: payload.replyToThreadId,
                            authorLabel: payload.authorLabel,
                            excerpt: payload.excerpt,
                          },
                        }))
                      }
                      threadComposer={{
                        draft: threadDrafts[it.id] ?? "",
                        onDraftChange: (v) => setThreadDrafts((d) => ({ ...d, [it.id]: v })),
                        onSubmit: (text) =>
                          void postTrainingThread(it.id, text, threadReplyTargets[it.id]?.replyToThreadId),
                        posting: threadPosting[it.id] ?? false,
                      }}
                    />
                  </div>
                );
              })}
          </div>
          {!loading && !err && apiBase && inTelegram && initData && (
            <div className="feed__actions">
              <button type="button" className="feed__btn" onClick={() => void load()}>
                Обновить
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
