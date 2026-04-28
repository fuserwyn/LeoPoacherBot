import { useCallback, useEffect, useState } from "react";
import { ActivityCard, type ActivityCardProps } from "./ActivityCard";
import { PackGroupChatPanel } from "./PackGroupChatPanel";
import {
  dtoToCard,
  HEALTHY_FEED_EMOJIS,
  mergePackFeedReactions,
  mergeTrainingFeedReactions,
  SICK_LEAVE_FEED_EMOJIS,
  type PackFeedItemDTO,
  type PackFeedThreadReplyDTO,
} from "../lib/packFeed";
import { timeAgoFromISO } from "../lib/timeAgo";
import "./FeedScreen.css";

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

type Props = {
  name: string;
  streak: number;
  userId: number;
  initData: string;
  inTelegram: boolean;
  showAlert: (m: string) => void;
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

export function FeedScreen({ name, streak, userId, initData, inTelegram, showAlert }: Props) {
  const [sub, setSub] = useState<Sub>("activity");
  const subtitle =
    streak === 0
      ? "Две вкладки: чужие отчёты и общий чат с @leo"
      : streak === 1
        ? "Лента отчётов и комната стаи: Лео отвечает только с @leo"
        : "Стая: кто тренировался, и чат с упоминанием бота.";

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
  }, [inTelegram, initData, name, streak]);

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
  }, [load]);

  return (
    <div className="feed">
      <header className="feed__header">
        <div>
          <h1 className="feed__greet">Стая, {name}</h1>
          <p className="feed__sub muted">{subtitle}</p>
        </div>
        <div className="feed__streak" aria-label={`Серия ${streak} дней`}>
          <span>🔥</span> {streak}
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
          <h2 className="section-title">Кто что постит</h2>
          {err && <p className="feed__err">{err}</p>}
          {loading && <p className="feed__load muted">Загрузка…</p>}
          <div className="feed__list">
            {!loading && !useMockFeed && feedItems.length === 0 && !err && (
              <p className="feed__empty muted">Пока нет отчётов в базе (или нет MONETIZED_CHAT_ID).</p>
            )}
            {useMockFeed &&
              mockFallback(name, streak).map((c, i) => <ActivityCard key={`mock-${i}`} {...c} />)}
            {!useMockFeed &&
              feedItems.map((it) => {
                const base = dtoToCard(it);
                if (it.type !== "training_done" && it.type !== "sick_leave" && it.type !== "healthy") {
                  return <ActivityCard key={it.id} {...base} />;
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
                    authorPhotoUrl: tr.author_photo_url,
                    replyTo: rq,
                    likeCount: tr.like_count ?? 0,
                    likeMe: Boolean(tr.like_me),
                  };
                });
                return (
                  <ActivityCard
                    key={it.id}
                    {...base}
                    reactions={
                      it.type === "training_done"
                        ? mergeTrainingFeedReactions(it.reactions)
                        : it.type === "sick_leave"
                          ? mergePackFeedReactions(SICK_LEAVE_FEED_EMOJIS, it.reactions)
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
