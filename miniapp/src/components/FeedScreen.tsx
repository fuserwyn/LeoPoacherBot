import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { usePullToRefresh } from "../hooks/usePullToRefresh";
import { useHorizontalDragScroll } from "../hooks/useHorizontalDragScroll";
import { ActivityCard, type ActivityCardProps } from "./ActivityCard";
import { PackGroupChatPanel } from "./PackGroupChatPanel";
import {
  dtoToCard,
  feedHasMatchingTrainingReport,
  mergePackFeedIncremental,
  mergeFeedReactionsForType,
  optimisticTogglePackFeedReaction,
  optimisticToggleThreadReplyLike,
  resolveFeedAvatarUrl,
  sortPackFeedItemsDesc,
  type PackFeedItemDTO,
  type PackFeedThreadReplyDTO,
} from "../lib/packFeed";
import { moderationUserMessage, isModerationError } from "../lib/moderationMessages";
import { clearFeedThreadUnread, fetchFeedThreadUnreadSummary } from "../lib/feedThreadUnread";
import { formatLocalDateTime } from "../lib/timeAgo";
import { streakStreakAriaLabel } from "../lib/streakLabel";
import {
  sortWorkoutCategoryIds,
  trainingDoneMatchesAnyCategory,
  WORKOUT_CATEGORY_OPTIONS_ALPHABETICAL,
  type WorkoutCategoryId,
} from "../lib/workoutCategories";
import "./FeedScreen.css";

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";
const FEED_POLL_MS = 8000;

function applyOptimisticFeedItem(items: PackFeedItemDTO[], optimistic: PackFeedItemDTO | null | undefined): PackFeedItemDTO[] {
  if (!optimistic) return items;
  if (feedHasMatchingTrainingReport(items, optimistic.text, optimistic.user_id)) return items;
  if (items.some((i) => i.id === optimistic.id)) return items;
  return sortPackFeedItemsDesc([optimistic, ...items]);
}

function bumpMaxFeedId(maxRef: { current: number }, items: PackFeedItemDTO[]) {
  for (const it of items) {
    if (it.id > maxRef.current) maxRef.current = it.id;
  }
}

type Props = {
  name: string;
  streak: number;
  userId: number;
  initData: string;
  inTelegram: boolean;
  showAlert: (m: string) => void;
  refreshToken?: number;
  /** Временная карточка сразу после отчёта (до ответа polling). */
  optimisticFeedItem?: PackFeedItemDTO | null;
  onOptimisticConsumed?: () => void;
  /** Перезагрузить общие данные (стрик, уровень, кубки) — вызывается при pull-to-refresh. */
  onRefreshAll?: () => Promise<void> | void;
  /** Вкладка «Стая» видима (keep-alive). */
  active?: boolean;
  /** Обновить бейджи таббара (после открытия общего чата). */
  onRefreshTabBadges?: () => void;
  /** Сразу убрать бейдж общего чата из UI. */
  onPackGroupChatOpened?: () => void;
  /** Непрочитанные комментарии в ленте (бейдж подвкладки). */
  feedThreadUnreadCount?: number;
  /** Мгновенно убрать бейдж «Лента» из UI (после просмотра подвкладки). */
  onFeedThreadRead?: () => void;
  /** Непрочитанные ответы в общем чате (бейдж подвкладки). */
  packGroupUnreadCount?: number;
};

type Sub = "activity" | "room";

type FeedViewportStyle = CSSProperties & {
  "--feed-vvh"?: string;
  "--feed-bottom-nav-h"?: string;
};

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
  streak,
  userId,
  initData,
  inTelegram,
  showAlert,
  refreshToken = 0,
  optimisticFeedItem = null,
  onOptimisticConsumed,
  onRefreshAll,
  active = true,
  onRefreshTabBadges,
  onPackGroupChatOpened,
  feedThreadUnreadCount = 0,
  onFeedThreadRead,
  packGroupUnreadCount = 0,
}: Props) {
  const [sub, setSub] = useState<Sub>("activity");
  const [unreadFeedCardIds, setUnreadFeedCardIds] = useState<Set<number>>(() => new Set());

  const [feedItems, setFeedItems] = useState<PackFeedItemDTO[]>([]);
  const [useMockFeed, setUseMockFeed] = useState(false);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [threadDrafts, setThreadDrafts] = useState<Record<number, string>>({});
  const [threadPosting, setThreadPosting] = useState<Record<number, boolean>>({});
  const [threadReplyDeleting, setThreadReplyDeleting] = useState<Record<number, boolean>>({});
  const [feedReportPosting, setFeedReportPosting] = useState<Record<number, boolean>>({});
  const [threadReplyReporting, setThreadReplyReporting] = useState<Record<number, boolean>>({});
  /** Ответ на конкретное сообщение треда (reply_to_id) — ключ id отчёта user_messages. */
  const [threadReplyTargets, setThreadReplyTargets] = useState<
    Record<number, { replyToThreadId: number; authorLabel: string; excerpt: string } | undefined>
  >({});
  const [feedOnlyMine, setFeedOnlyMine] = useState(false);
  /** Мультивыбор типов тренировок (пусто = «все типы»). */
  const [feedCategoryIds, setFeedCategoryIds] = useState<WorkoutCategoryId[]>([]);
  const [viewportStyle, setViewportStyle] = useState<FeedViewportStyle>({});
  const feedHeaderRef = useRef<HTMLDivElement>(null);
  const filterCatsRef = useHorizontalDragScroll<HTMLDivElement>();
  const maxFeedIdRef = useRef(0);
  const loadedOnceRef = useRef(false);
  /** Идёт полный синк — чтобы не плодить дубли (двойной fetch на маунте, наложение поллинга и пост-экшн-синков). */
  const fullSyncInFlightRef = useRef(false);

  const categoryFilterSet = useMemo(() => new Set(feedCategoryIds), [feedCategoryIds]);

  const feedWithOptimistic = useMemo(
    () => applyOptimisticFeedItem(feedItems, optimisticFeedItem),
    [feedItems, optimisticFeedItem],
  );

  const visibleFeedItems = useMemo(() => {
    return feedWithOptimistic.filter((it) => {
      if (it.type === "sick_leave") return false;
      // Свой только что отправленный пост показываем всегда, даже если активен
      // фильтр по типам тренировок (иначе кажется, что отчёт не отправился).
      if (optimisticFeedItem && it.id === optimisticFeedItem.id) return true;
      if (feedOnlyMine && !it.is_you) return false;
      if (feedCategoryIds.length > 0) {
        if (it.type !== "training_done") return false;
        if (!trainingDoneMatchesAnyCategory(it.text, categoryFilterSet)) return false;
      }
      return true;
    });
  }, [feedWithOptimistic, feedOnlyMine, feedCategoryIds, categoryFilterSet, optimisticFeedItem]);

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

  const refreshUnreadFeedCards = useCallback(async () => {
    if (!inTelegram || !initData.trim()) {
      setUnreadFeedCardIds(new Set());
      return;
    }
    const summary = await fetchFeedThreadUnreadSummary(initData);
    setUnreadFeedCardIds(new Set(summary.userMessageIds));
  }, [inTelegram, initData]);

  useEffect(() => {
    if (!active || !inTelegram || !initData.trim()) return;
    void refreshUnreadFeedCards();
    const t = window.setInterval(() => void refreshUnreadFeedCards(), 30_000);
    return () => window.clearInterval(t);
  }, [active, inTelegram, initData, refreshUnreadFeedCards, feedThreadUnreadCount]);

  const markFeedCardThreadRead = useCallback(
    (userMessageId: number) => {
      if (userMessageId <= 0 || !initData.trim()) return;
      setUnreadFeedCardIds((prev) => {
        if (!prev.has(userMessageId)) return prev;
        const next = new Set(prev);
        next.delete(userMessageId);
        return next;
      });
      void (async () => {
        await clearFeedThreadUnread(initData, userMessageId);
        onRefreshTabBadges?.();
        await refreshUnreadFeedCards();
      })();
    },
    [initData, onRefreshTabBadges, refreshUnreadFeedCards],
  );

  // Бейдж «Лента» = непрочитанные комментарии к твоим отчётам. На подвкладке «Лента»
  // считаем их просмотренными сразу — без скролла до карточки (она может быть вне
  // окна последних 50 постов).
  useEffect(() => {
    if (!active || sub !== "activity" || !inTelegram || !initData.trim()) return;
    if (feedThreadUnreadCount <= 0 && unreadFeedCardIds.size === 0) return;

    onFeedThreadRead?.();
    setUnreadFeedCardIds(new Set());
    void (async () => {
      await clearFeedThreadUnread(initData);
      onRefreshTabBadges?.();
    })();
  }, [
    active,
    sub,
    inTelegram,
    initData,
    feedThreadUnreadCount,
    unreadFeedCardIds.size,
    onFeedThreadRead,
    onRefreshTabBadges,
  ]);

  const feedSubtabBadge = (count: number) => (count > 9 ? "9+" : count > 0 ? String(count) : null);

  const syncFeed = useCallback(
    async (opts?: { full?: boolean; silent?: boolean }) => {
      if (!apiBase || !inTelegram || !initData) {
        setLoading(false);
        setUseMockFeed(true);
        setFeedItems([]);
        setErr(null);
        maxFeedIdRef.current = 0;
        return;
      }
      const full = opts?.full === true || maxFeedIdRef.current === 0;
      const sinceId = full ? 0 : maxFeedIdRef.current;
      // Дедуп: если полный синк уже выполняется, новый полный синк пропускаем
      // (защищает от двойного fetch на маунте и наложения поллинга/пост-экшн-синков).
      if (full && fullSyncInFlightRef.current) return;
      if (full) fullSyncInFlightRef.current = true;
      if (!opts?.silent) setErr(null);
      if (!opts?.silent && !loadedOnceRef.current) setLoading(true);
      setUseMockFeed(false);
      try {
        const res = await fetch(`${apiBase}/api/miniapp/feed`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, since_id: sinceId }),
        });
        const j = (await res.json().catch(() => ({}))) as { ok?: boolean; items?: PackFeedItemDTO[]; error?: string };
        if (!res.ok) {
          if (res.status === 403) {
            setErr("Нет доступа к ленте стаи: нужна подписка/участие в группе, как в боте.");
            if (!opts?.silent) setFeedItems([]);
            return;
          }
          if (!opts?.silent) {
            setErr(j.error ?? `Ошибка ${res.status}`);
            setFeedItems([]);
          }
          return;
        }
        // Успешный ответ (в т.ч. тихий поллинг) снимает «липкий» баннер ошибки.
        setErr(null);
        const incoming = j.items ?? [];
        if (full || sinceId === 0) {
          setFeedItems(incoming);
          maxFeedIdRef.current = 0;
          bumpMaxFeedId(maxFeedIdRef, incoming);
          if (
            optimisticFeedItem &&
            feedHasMatchingTrainingReport(incoming, optimisticFeedItem.text, optimisticFeedItem.user_id)
          ) {
            onOptimisticConsumed?.();
          }
        } else {
          setFeedItems((prev) => {
            const next = mergePackFeedIncremental(prev, incoming);
            bumpMaxFeedId(maxFeedIdRef, incoming);
            if (
              optimisticFeedItem &&
              feedHasMatchingTrainingReport(next, optimisticFeedItem.text, optimisticFeedItem.user_id)
            ) {
              onOptimisticConsumed?.();
            }
            return next;
          });
        }
        loadedOnceRef.current = true;
      } catch (e) {
        if (!opts?.silent) {
          setErr(e instanceof Error ? e.message : "Сеть");
          setFeedItems([]);
        }
      } finally {
        if (full) fullSyncInFlightRef.current = false;
        if (!opts?.silent) setLoading(false);
      }
    },
    [inTelegram, initData, optimisticFeedItem, onOptimisticConsumed],
  );

  const postTrainingReact = useCallback(
    (userMessageId: number, emoji: string) => {
      if (!apiBase || !initData) return;

      const applyToggle = () =>
        setFeedItems((prev) =>
          prev.map((it) =>
            it.id === userMessageId
              ? { ...it, reactions: optimisticTogglePackFeedReaction(it.reactions, emoji) }
              : it,
          ),
        );

      // Оптимистично переключаем реакцию сразу. Откат — повторное переключение
      // того же эмодзи на этом же посте (toggle идемпотентен), чтобы не затереть
      // параллельные оптимистичные изменения других реакций/постов.
      applyToggle();

      void (async () => {
        try {
          const res = await fetch(`${apiBase}/api/miniapp/feed/training/react`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ init_data: initData, user_message_id: userMessageId, emoji }),
          });
          const j = (await res.json().catch(() => ({}))) as { error?: string };
          if (!res.ok) {
            applyToggle();
            showAlert(j.error === "invalid_emoji" ? "Такую реакцию нельзя" : j.error ?? `Ошибка ${res.status}`);
            return;
          }
          void syncFeed({ full: true, silent: true });
        } catch (e) {
          applyToggle();
          showAlert(e instanceof Error ? e.message : "Сеть");
        }
      })();
    },
    [apiBase, initData, syncFeed, showAlert],
  );

  const voteFeedPoll = useCallback(
    async (userMessageId: number, optionIndex: number) => {
      if (!apiBase || !initData) return;
      try {
        const res = await fetch(`${apiBase}/api/miniapp/feed/poll/vote`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, user_message_id: userMessageId, option_index: optionIndex }),
        });
        const j = (await res.json().catch(() => ({}))) as { error?: string };
        if (!res.ok) {
          const errMap: Record<string, string> = {
            not_found: "Опрос не найден",
            invalid_option: "Неверный вариант",
            forbidden: "Нет доступа",
            chat_mismatch: "Открой miniapp из чата стаи",
            poll_vote_error: "Не удалось сохранить голос",
          };
          showAlert(errMap[j.error ?? ""] ?? j.error ?? `Ошибка ${res.status}`);
          return;
        }
        await syncFeed({ full: true, silent: true });
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      }
    },
    [apiBase, initData, syncFeed, showAlert],
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
          message?: string;
          thread?: PackFeedThreadReplyDTO[];
        };
        if (!res.ok) {
          if (isModerationError(j.error)) {
            showAlert(moderationUserMessage(j.error, j.message));
            return;
          }
          const errMap: Record<string, string> = {
            empty_text: "Пустой комментарий",
            text_too_long: "Слишком длинно (максимум 500 символов)",
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
        if (Array.isArray(postedThread)) {
          setFeedItems((prev) =>
            prev.map((it) => (it.id === userMessageId ? { ...it, thread: postedThread } : it)),
          );
        } else {
          await syncFeed({ full: true, silent: true });
        }
        if (replyToThreadId) {
          window.setTimeout(() => void syncFeed({ full: true, silent: true }), 5000);
        }
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      } finally {
        setThreadPosting((p) => ({ ...p, [userMessageId]: false }));
      }
    },
    [apiBase, initData, syncFeed, showAlert],
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
        if (Array.isArray(updated)) {
          setFeedItems((prev) =>
            prev.map((it) => (it.id === trainingUserMessageId ? { ...it, thread: updated } : it)),
          );
        } else {
          await syncFeed({ full: true, silent: true });
        }
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      } finally {
        setThreadReplyDeleting((p) => ({ ...p, [threadReplyId]: false }));
      }
    },
    [apiBase, initData, syncFeed, showAlert],
  );

  const reportFeedContent = useCallback(
    async (userMessageId: number, threadReplyId = 0) => {
      if (!apiBase || !initData) return;
      const postingKey = threadReplyId > 0 ? threadReplyId : userMessageId;
      const setPosting =
        threadReplyId > 0 ? setThreadReplyReporting : setFeedReportPosting;
      setPosting((p) => ({ ...p, [postingKey]: true }));
      try {
        const res = await fetch(`${apiBase}/api/miniapp/feed/report`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            init_data: initData,
            user_message_id: userMessageId,
            thread_reply_id: threadReplyId,
          }),
        });
        const j = (await res.json().catch(() => ({}))) as { error?: string };
        if (!res.ok) {
          const errMap: Record<string, string> = {
            not_found: "Запись не найдена (обнови ленту)",
            forbidden: "Нет доступа",
            chat_mismatch: "Открой мини-апп из чата стаи",
            cannot_report_self: "Нельзя пожаловаться на своё сообщение",
            cannot_report_leo: "На это сообщение пожаловаться нельзя",
            already_reported: "Ты уже отправлял жалобу на это",
            report_error: "Не удалось отправить жалобу",
          };
          showAlert(errMap[j.error ?? ""] ?? j.error ?? `Ошибка ${res.status}`);
          return;
        }
        showAlert(
          threadReplyId > 0
            ? "Жалоба на комментарий отправлена. Админы увидят её в поддержке."
            : "Жалоба отправлена. Админы увидят её в поддержке.",
        );
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      } finally {
        setPosting((p) => ({ ...p, [postingKey]: false }));
      }
    },
    [apiBase, initData, showAlert],
  );

  const toggleTrainingThreadLike = useCallback(
    (trainingUserMessageId: number, threadReplyId: number) => {
      if (!apiBase || !initData) return;

      const applyToggle = () =>
        setFeedItems((prev) =>
          prev.map((it) =>
            it.id === trainingUserMessageId && it.thread
              ? { ...it, thread: optimisticToggleThreadReplyLike(it.thread, threadReplyId) }
              : it,
          ),
        );

      // Оптимистично переключаем лайк; откат — повторный toggle того же лайка,
      // чтобы не затирать другие параллельные оптимистичные изменения.
      applyToggle();

      void (async () => {
        try {
          const res = await fetch(`${apiBase}/api/miniapp/feed/training/thread/like`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ init_data: initData, thread_reply_id: threadReplyId }),
          });
          const j = (await res.json().catch(() => ({}))) as { error?: string; thread?: PackFeedThreadReplyDTO[] };
          if (!res.ok) {
            applyToggle();
            showAlert(j.error ?? `Ошибка ${res.status}`);
            return;
          }
          const updated = j.thread;
          if (Array.isArray(updated)) {
            setFeedItems((prev) =>
              prev.map((it) => (it.id === trainingUserMessageId ? { ...it, thread: updated } : it)),
            );
          } else {
            void syncFeed({ full: true, silent: true });
          }
        } catch (e) {
          applyToggle();
          showAlert(e instanceof Error ? e.message : "Сеть");
        }
      })();
    },
    [apiBase, initData, syncFeed, showAlert],
  );

  useEffect(() => {
    maxFeedIdRef.current = 0;
    loadedOnceRef.current = false;
    if (active) void syncFeed({ full: true });
  }, [refreshToken, syncFeed, active]);

  useEffect(() => {
    if (!active || loadedOnceRef.current) return;
    void syncFeed({ full: true });
  }, [active, syncFeed]);

  useEffect(() => {
    // Поллим ленту только когда открыта именно подвкладка «Лента»: на подвкладке
    // общего чата это лишняя сеть/батарея и фоновые апдейты состояния.
    if (!active || sub !== "activity" || !apiBase || !inTelegram || !initData.trim()) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const tick = async () => {
      if (cancelled) return;
      // Полный синк на каждом опросе: только так подтягиваются новые комментарии
      // (тред) и реакции к УЖЕ существующим постам. Инкрементальный since_id
      // возвращает лишь новые посты и не обновляет треды старых.
      await syncFeed({ full: true, silent: true });
      if (cancelled) return;
      timer = setTimeout(tick, FEED_POLL_MS);
    };

    timer = setTimeout(tick, FEED_POLL_MS);
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [active, sub, inTelegram, initData, syncFeed]);

  useLayoutEffect(() => {
    const sticky = feedHeaderRef.current;
    const feedRoot = sticky?.closest<HTMLElement>(".feed");
    if (!sticky || !feedRoot) return;
    // Реальный зазор: низ fixed-шапки минус верх .feed (учитывает safe-area, app padding, border).
    const write = () => {
      const feedTop = feedRoot.getBoundingClientRect().top;
      const headerBottom = sticky.getBoundingClientRect().bottom;
      const offset = headerBottom - feedTop;
      feedRoot.style.setProperty("--feed-header-h", `${Math.max(0, Math.ceil(offset))}px`);
      // Абсолютный низ fixed-шапки от viewport — для position:fixed чата (без safe-top + offset).
      feedRoot.style.setProperty("--feed-sticky-bottom", `${Math.max(0, Math.ceil(headerBottom))}px`);
    };
    write();
    const ro = new ResizeObserver(write);
    ro.observe(sticky);
    const onLoad = () => write();
    window.addEventListener("load", onLoad);
    requestAnimationFrame(write);
    return () => {
      ro.disconnect();
      window.removeEventListener("load", onLoad);
      feedRoot.style.removeProperty("--feed-header-h");
      feedRoot.style.removeProperty("--feed-sticky-bottom");
    };
  }, [sub, feedCategoryIds.length, feedOnlyMine]);

  useEffect(() => {
    let raf = 0;

    const readViewport = () => {
      raf = 0;
      const vv = window.visualViewport;
      // Layout viewport (window.innerHeight) НЕ ужимается при открытии клавиатуры на iOS —
      // это именно то, что нужно для max-height карточки треда: размер карточки должен
      // остаться стабильным, пока пользователь печатает. Если использовать vv.height,
      // карточка резко сжимается в момент тапа по textarea и iOS не успевает корректно
      // докрутить поле над клавиатурой.
      const layoutH = Math.max(320, Math.floor(window.innerHeight || vv?.height || 320));
      const bottomNavH = Math.ceil(
        document.querySelector<HTMLElement>(".bottom-nav")?.getBoundingClientRect().height ?? 0,
      );

      setViewportStyle((prev) => {
        const next: FeedViewportStyle = {
          "--feed-vvh": `${layoutH}px`,
          "--feed-bottom-nav-h": `${bottomNavH}px`,
        };
        if (
          prev["--feed-vvh"] === next["--feed-vvh"] &&
          prev["--feed-bottom-nav-h"] === next["--feed-bottom-nav-h"]
        ) {
          return prev;
        }
        return next;
      });
    };

    const scheduleRead = () => {
      if (raf) return;
      raf = requestAnimationFrame(readViewport);
    };

    readViewport();
    const vv = window.visualViewport;
    vv?.addEventListener("resize", scheduleRead);
    vv?.addEventListener("scroll", scheduleRead);
    window.addEventListener("orientationchange", scheduleRead);
    const tg = window.Telegram?.WebApp as { onEvent?: (e: string, fn: () => void) => void } | undefined;
    tg?.onEvent?.("viewportChanged", scheduleRead);
    return () => {
      if (raf) cancelAnimationFrame(raf);
      vv?.removeEventListener("resize", scheduleRead);
      vv?.removeEventListener("scroll", scheduleRead);
      window.removeEventListener("orientationchange", scheduleRead);
      const tgOff = window.Telegram?.WebApp as { offEvent?: (e: string, fn: () => void) => void } | undefined;
      tgOff?.offEvent?.("viewportChanged", scheduleRead);
    };
  }, []);

  const handlePullRefresh = useCallback(async () => {
    hapticLight();
    maxFeedIdRef.current = 0;
    await Promise.all([syncFeed({ full: true }), Promise.resolve(onRefreshAll?.())]);
  }, [syncFeed, onRefreshAll, hapticLight]);

  const ptr = usePullToRefresh({
    onRefresh: handlePullRefresh,
    enabled: sub === "activity",
  });

  const ptrStatusText =
    ptr.status === "refreshing"
      ? "Обновление…"
      : ptr.status === "ready"
        ? "Отпусти, чтобы обновить"
        : ptr.status === "pulling"
          ? "Тяни вниз"
          : "";

  return (
    <div className="feed" style={viewportStyle}>
      <div className="feed__sticky" ref={feedHeaderRef}>
        <div className="feed__sticky-inner">
        <header className="feed__header">
          <div className="feed__brand">Fat Leopard</div>
          <div className="feed__hero">
            <div className="feed__streak" aria-label={streakStreakAriaLabel(streak)} title={streakStreakAriaLabel(streak)}>
              <span className="feed__streak-row">
                <span className="feed__streak-word">Стрик</span>
                <span className="feed__streak-num">{streak}</span>
              </span>
            </div>
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
            <span className="feed__subtab-inner">
              Лента
              {feedSubtabBadge(feedThreadUnreadCount) != null ? (
                <span className="feed__subtab-badge" aria-hidden>
                  {feedSubtabBadge(feedThreadUnreadCount)}
                </span>
              ) : null}
            </span>
          </button>
          <button
            type="button"
            className={`feed__subtab ${sub === "room" ? "is-active" : ""}`}
            onClick={() => {
              onPackGroupChatOpened?.();
              setSub("room");
            }}
            role="tab"
            aria-selected={sub === "room"}
          >
            <span className="feed__subtab-inner">
              Чат
              {feedSubtabBadge(packGroupUnreadCount) != null ? (
                <span className="feed__subtab-badge" aria-hidden>
                  {feedSubtabBadge(packGroupUnreadCount)}
                </span>
              ) : null}
            </span>
          </button>
        </div>
        {sub === "activity" && (
          <div className="feed__filters" aria-label="Фильтры ленты" data-no-ptr>
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
            <div
              className="feed__filter-cats"
              ref={filterCatsRef}
              role="group"
              aria-label="Тип тренировки"
            >
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
        )}
        </div>
      </div>
      <div className="feed__pull-shift">
      {sub === "activity" && (ptr.pull > 0 || ptr.status === "refreshing") ? (
        <div
          className={`feed__ptr feed__ptr--${ptr.status}`}
          style={{ height: `${ptr.pull}px` }}
          aria-hidden
        >
          <span className="feed__ptr-icon">
            {ptr.status === "refreshing" ? "↻" : ptr.status === "ready" ? "↑" : "↓"}
          </span>
          {ptrStatusText ? <span className="feed__ptr-label">{ptrStatusText}</span> : null}
        </div>
      ) : null}
      <div className={`feed__subpane${sub !== "room" ? " feed__subpane--hidden" : ""}`}>
        <PackGroupChatPanel
          active={active && sub === "room"}
          initData={initData}
          inTelegram={inTelegram}
          meId={userId}
          showAlert={showAlert}
          onRefreshTabBadges={onRefreshTabBadges}
          onPackGroupChatOpened={onPackGroupChatOpened}
          onHaptic={() => {
            const w = window.Telegram?.WebApp;
            w?.HapticFeedback?.impactOccurred?.("light");
          }}
        />
      </div>
      <div className={`feed__subpane${sub !== "activity" ? " feed__subpane--hidden" : ""}`}>
          <div className="feed__section-row">
            <h2 className="section-title feed__section-title">Тренировки стаи</h2>
            {apiBase && inTelegram && initData && (
              <button
                type="button"
                className="feed__refresh-btn"
                disabled={loading}
                onClick={() => {
                  window.scrollTo({ top: 0, behavior: "smooth" });
                  maxFeedIdRef.current = 0;
                  void syncFeed({ full: true });
                }}
                aria-label="Обновить ленту"
                title="Обновить"
              >
                {loading ? "…" : "↻"}
              </button>
            )}
          </div>
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
                const supportsThread =
                  it.type === "training_done" ||
                  it.type === "sick_leave" ||
                  it.type === "healthy" ||
                  it.type === "admin_post" ||
                  it.type === "admin_poll";
                const supportsReactions =
                  supportsThread ||
                  it.type === "pack_join" ||
                  it.type === "pack_rejoin" ||
                  it.type === "daily_wisdom";
                const isLeoSystemFeed =
                  it.type === "pack_join" ||
                  it.type === "pack_rejoin" ||
                  it.type === "daily_wisdom" ||
                  it.type === "pack_removed" ||
                  it.type === "admin_post" ||
                  it.type === "admin_poll" ||
                  it.type === "inactive_notice";
                const slotClass = `feed__card-slot${it.is_you && !isLeoSystemFeed ? " feed__card-slot--mine" : " feed__card-slot--them"}`;
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
                    timeAgo: formatLocalDateTime(tr.created_at),
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
                const canReportCard = !it.is_you && !isLeoSystemFeed;
                if (!supportsThread) {
                  return (
                    <div key={it.id} className={slotClass}>
                      <ActivityCard
                        {...base}
                        reactions={supportsReactions ? mergeFeedReactionsForType(it.type, it.reactions) : undefined}
                        onReactionClick={
                          supportsReactions ? (emoji) => void postTrainingReact(it.id, emoji) : undefined
                        }
                        onReport={canReportCard ? () => void reportFeedContent(it.id) : undefined}
                        reportPosting={feedReportPosting[it.id] ?? false}
                        poll={
                          it.poll
                            ? {
                                totalVotes: it.poll.total_votes ?? 0,
                                options: (it.poll.options ?? []).map((option, optionIndex) => ({
                                  label: option.label,
                                  votes: option.votes,
                                  selected: it.poll?.my_vote_index === optionIndex,
                                })),
                                onVote: (optionIndex) => void voteFeedPoll(it.id, optionIndex),
                              }
                            : undefined
                        }
                      />
                    </div>
                  );
                }
                return (
                  <div key={it.id} className={slotClass}>
                    <ActivityCard
                      {...base}
                      poll={
                        it.poll
                          ? {
                              totalVotes: it.poll.total_votes ?? 0,
                              options: (it.poll.options ?? []).map((option, optionIndex) => ({
                                label: option.label,
                                votes: option.votes,
                                selected: it.poll?.my_vote_index === optionIndex,
                              })),
                              onVote: (optionIndex) => void voteFeedPoll(it.id, optionIndex),
                            }
                          : undefined
                      }
                      reactions={mergeFeedReactionsForType(it.type, it.reactions)}
                      onReactionClick={(emoji) => void postTrainingReact(it.id, emoji)}
                      hasUnreadThread={unreadFeedCardIds.has(it.id)}
                      onThreadOpened={() => markFeedCardThreadRead(it.id)}
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
                      onReport={canReportCard ? () => void reportFeedContent(it.id) : undefined}
                      reportPosting={feedReportPosting[it.id] ?? false}
                      onThreadReplyReport={(replyId) => void reportFeedContent(it.id, replyId)}
                      threadReplyReporting={threadReplyReporting}
                    />
                  </div>
                );
              })}
          </div>
      </div>
      </div>
    </div>
  );
}
