import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { formatChatTime } from "../lib/timeAgo";
import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import { moderationUserMessage, isModerationError } from "../lib/moderationMessages";
import { clearPackGroupUnread, fetchPackGroupUnreadSummary } from "../lib/packGroupUnread";
import {
  mergeTrainingFeedReactions,
  optimisticTogglePackFeedReaction,
  resolveFeedAvatarUrl,
  type PackFeedReactionDTO,
} from "../lib/packFeed";
import "./ActivityCard.css";
import "./PackGroupChatPanel.css";

function PackGroupMessageMenu({
  reactions,
  onReaction,
  canReport,
  onReport,
  reporting = false,
}: {
  reactions: { emoji: string; count: number; me?: boolean }[];
  onReaction: (emoji: string) => void;
  canReport?: boolean;
  onReport?: () => void;
  reporting?: boolean;
}) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent | TouchEvent) => {
      const t = e.target as Node;
      if (wrapRef.current?.contains(t)) return;
      setOpen(false);
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", close);
    document.addEventListener("touchstart", close);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("touchstart", close);
      document.removeEventListener("keydown", esc);
    };
  }, [open]);

  return (
    <div
      className={`packroom__more${open ? " packroom__more--open" : ""}`}
      ref={wrapRef}
    >
      <button
        type="button"
        className={`packroom__more-toggle${open ? " packroom__more-toggle--open" : ""}`}
        aria-expanded={open}
        aria-haspopup="menu"
        aria-label="Реакции и действия"
        disabled={reporting}
        onClick={() => setOpen((v) => !v)}
      >
        {reporting ? "…" : "⋯"}
      </button>
      {open && !reporting && (
        <div className="packroom__more-popover" role="menu">
          <div className="packroom__more-emojis" role="group" aria-label="Реакции">
            {reactions.map((r) => (
              <button
                key={r.emoji}
                type="button"
                className={`act-card__react-btn${r.me ? " act-card__react-btn--mine" : ""}`}
                onClick={() => {
                  onReaction(r.emoji);
                  setOpen(false);
                }}
              >
                {r.emoji}
                {r.count > 0 && <span className="act-card__react-cnt">{r.count}</span>}
              </button>
            ))}
          </div>
          {canReport && onReport != null && (
            <>
              <div className="packroom__more-divider" aria-hidden />
              <button
                type="button"
                className="packroom__more-item"
                role="menuitem"
                onClick={() => {
                  setOpen(false);
                  onReport();
                }}
              >
                Пожаловаться на сообщение
              </button>
            </>
          )}
        </div>
      )}
    </div>
  );
}

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type PackGroupMessage = {
  id: number;
  user_id: number;
  username: string;
  text: string;
  created_at: string;
  is_leo: boolean;
  author_photo_url?: string;
  reply_to_id?: number;
  reply_to_username?: string;
  reply_to_text?: string;
  reply_to_is_leo?: boolean;
  reactions?: PackFeedReactionDTO[];
};

type ReplyIntent = {
  replyToMessageId: number;
  authorLabel: string;
  excerpt: string;
};

function excerptText(text: string, max = 100): string {
  const t = text.trim();
  if (t.length <= max) return t;
  return `${t.slice(0, max - 1).trim()}…`;
}

function resolveReplyQuote(
  m: PackGroupMessage,
  byId: Map<number, PackGroupMessage>,
): { author: string; text: string; parentId: number } | null {
  const parentId = m.reply_to_id ?? 0;
  if (parentId <= 0) return null;

  const serverAuthor = m.reply_to_is_leo ? "Лео" : (m.reply_to_username || "").trim();
  const serverText = (m.reply_to_text || "").trim();
  if (serverAuthor !== "" || serverText !== "") {
    return { author: serverAuthor || "Сообщение", text: serverText, parentId };
  }

  const parent = byId.get(parentId);
  if (parent) {
    return {
      author: parent.is_leo ? "Лео" : parent.username,
      text: excerptText(parent.text),
      parentId,
    };
  }
  return null;
}

type Props = {
  initData: string;
  inTelegram: boolean;
  meId: number;
  showAlert: (m: string) => void;
  onHaptic?: () => void;
  /** Подвкладка «Чат» в ленте видима (keep-alive). */
  active?: boolean;
  onRefreshTabBadges?: () => void;
  /** Сразу убрать бейдж общего чата из UI (до ответа сервера). */
  onPackGroupChatOpened?: () => void;
};

export function PackGroupChatPanel({
  initData,
  inTelegram,
  meId,
  showAlert,
  onHaptic,
  active = true,
  onRefreshTabBadges,
  onPackGroupChatOpened,
}: Props) {
  const [items, setItems] = useState<PackGroupMessage[]>([]);
  const [text, setText] = useState("");
  const [replyIntent, setReplyIntent] = useState<ReplyIntent | null>(null);
  const [sending, setSending] = useState(false);
  const [reportPosting, setReportPosting] = useState<Record<number, boolean>>({});
  const [err, setErr] = useState<string | null>(null);
  const endRef = useRef<HTMLDivElement | null>(null);
  const logRef = useRef<HTMLDivElement | null>(null);
  const roomRef = useRef<HTMLDivElement | null>(null);
  const formRef = useRef<HTMLFormElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  /** true когда пользователь прокрутил вверх — не перебиваем позицию при поллинге */
  const userScrolledUpRef = useRef(false);
  const didInitialScrollRef = useRef(false);
  /** После отправки — всегда вниз, даже если читал историю выше */
  const forceScrollRef = useRef(false);
  const wasActiveRef = useRef(false);
  const [highlightMessageId, setHighlightMessageId] = useState<number | null>(null);
  const [unreadMessageIds, setUnreadMessageIds] = useState<Set<number>>(() => new Set());
  const highlightTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingUnreadScrollRef = useRef<number | null>(null);

  const isNearBottom = useCallback((el: HTMLDivElement, threshold = 96) => {
    return el.scrollHeight - el.scrollTop - el.clientHeight < threshold;
  }, []);

  const scrollToBottom = useCallback(() => {
    const el = logRef.current;
    if (!el) return;
    const run = () => {
      el.scrollTop = el.scrollHeight;
      endRef.current?.scrollIntoView({ block: "end" });
    };
    run();
    requestAnimationFrame(() => {
      run();
      requestAnimationFrame(run);
    });
  }, []);

  const scrollToBottomIfNear = useCallback(() => {
    const el = logRef.current;
    if (!el) return;
    if (!userScrolledUpRef.current || isNearBottom(el, 160)) {
      userScrolledUpRef.current = false;
      scrollToBottom();
    }
  }, [isNearBottom, scrollToBottom]);

  useLayoutEffect(() => {
    const form = formRef.current;
    const host = roomRef.current;
    if (!form || !host) return;
    const write = () => {
      host.style.setProperty("--pack-composer-h", `${Math.ceil(form.getBoundingClientRect().height)}px`);
    };
    write();
    const ro = new ResizeObserver(write);
    ro.observe(form);
    return () => {
      ro.disconnect();
      host.style.removeProperty("--pack-composer-h");
    };
  }, []);

  /** Пока клавиатура анимируется — держим низ ленты у поля ввода. */
  useEffect(() => {
    if (!active) return;
    const vv = window.visualViewport;
    const onViewportChange = () => {
      const input = document.activeElement;
      if (!(input instanceof HTMLInputElement) || !input.classList.contains("packroom__input")) return;
      userScrolledUpRef.current = false;
      scrollToBottom();
    };
    vv?.addEventListener("resize", onViewportChange);
    vv?.addEventListener("scroll", onViewportChange);
    return () => {
      vv?.removeEventListener("resize", onViewportChange);
      vv?.removeEventListener("scroll", onViewportChange);
    };
  }, [active, scrollToBottom]);

  /** После закрытия клавиатуры (тап по ленте) — не оставлять низ под полем ввода. */
  useEffect(() => {
    if (!active) return;
    const root = document.documentElement;
    let wasOpen = root.classList.contains("app-keyboard-open");
    const mo = new MutationObserver(() => {
      const open = root.classList.contains("app-keyboard-open");
      if (wasOpen && !open) {
        requestAnimationFrame(() => {
          requestAnimationFrame(scrollToBottomIfNear);
        });
        window.setTimeout(scrollToBottomIfNear, 120);
        window.setTimeout(scrollToBottomIfNear, 320);
      }
      wasOpen = open;
    });
    mo.observe(root, { attributes: true, attributeFilter: ["class"] });
    return () => mo.disconnect();
  }, [active, scrollToBottomIfNear]);

  const load = useCallback(async () => {
    if (!apiBase || !inTelegram || !initData) return;
    setErr(null);
    try {
      const res = await fetch(`${apiBase}/api/miniapp/pack-group/feed`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData }),
      });
      const j = (await res.json().catch(() => ({}))) as { ok?: boolean; messages?: PackGroupMessage[]; error?: string };
      if (!res.ok) {
        if (res.status === 403) {
          setErr("Нет доступа к чату стаи.");
          setItems([]);
          return;
        }
        setErr(j.error ?? `Ошибка ${res.status}`);
        return;
      }
      setItems(j.messages ?? []);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Сеть");
    }
  }, [inTelegram, initData]);

  const bootstrapUnreadAndLoad = useCallback(async () => {
    if (!inTelegram || !initData.trim()) return;
    onPackGroupChatOpened?.();
    const summary = await fetchPackGroupUnreadSummary(initData);
    setUnreadMessageIds(new Set(summary.messageIds));
    pendingUnreadScrollRef.current = summary.messageIds[0] ?? null;
    void clearPackGroupUnread(initData).then(() => onRefreshTabBadges?.());
    void load();
  }, [inTelegram, initData, load, onRefreshTabBadges, onPackGroupChatOpened]);

  useEffect(() => {
    if (!active) return;
    void bootstrapUnreadAndLoad();
  }, [active, bootstrapUnreadAndLoad]);

  useEffect(() => {
    if (!active) {
      didInitialScrollRef.current = false;
      userScrolledUpRef.current = false;
      wasActiveRef.current = false;
      return;
    }
    if (!wasActiveRef.current) {
      didInitialScrollRef.current = false;
      userScrolledUpRef.current = false;
      forceScrollRef.current = true;
      wasActiveRef.current = true;
    }
  }, [active]);

  // body--lock только пока открыта подвкладка «Чат» стаи.
  useEffect(() => {
    if (!active) {
      document.body.classList.remove("body--lock");
      return;
    }
    document.body.classList.add("body--lock");
    return () => document.body.classList.remove("body--lock");
  }, [active]);

  useEffect(() => {
    if (!active || !apiBase || !inTelegram || !initData) return;
    const t = setInterval(() => void load(), 5000);
    return () => clearInterval(t);
  }, [load, active, inTelegram, initData]);

  // Отслеживаем, прокрутил ли пользователь вверх, чтобы не перебивать позицию при поллинге.
  useEffect(() => {
    const el = logRef.current;
    if (!el) return;
    const onScroll = () => {
      userScrolledUpRef.current = !isNearBottom(el);
    };
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => el.removeEventListener("scroll", onScroll);
  }, [isNearBottom]);

  // Рост ленты (поллинг, аватары) — держим конец видимым, если пользователь внизу.
  useEffect(() => {
    const el = logRef.current;
    if (!el) return;
    const inner = el.firstElementChild;
    if (!(inner instanceof HTMLElement)) return;

    const ro = new ResizeObserver(() => {
      if (forceScrollRef.current || !userScrolledUpRef.current) {
        scrollToBottom();
      }
    });
    ro.observe(inner);
    const onImgLoad = (e: Event) => {
      if (!(e.target instanceof HTMLImageElement)) return;
      if (!e.target.classList.contains("packroom__avatar")) return;
      if (forceScrollRef.current || !userScrolledUpRef.current) {
        scrollToBottom();
      }
    };
    inner.addEventListener("load", onImgLoad, true);
    return () => {
      ro.disconnect();
      inner.removeEventListener("load", onImgLoad, true);
    };
  }, [items.length, scrollToBottom]);

  const scrollToQuotedMessage = useCallback((messageId: number) => {
    const log = logRef.current;
    if (!log || messageId <= 0) return;
    const target = log.querySelector<HTMLElement>(`[data-pack-msg-id="${messageId}"]`);
    if (!target) return;
    target.scrollIntoView({ block: "center", behavior: "smooth" });
    setHighlightMessageId(messageId);
    if (highlightTimerRef.current) clearTimeout(highlightTimerRef.current);
    highlightTimerRef.current = setTimeout(() => setHighlightMessageId(null), 1600);
  }, []);

  useEffect(() => {
    if (!active) return;
    const el = logRef.current;
    if (!el || items.length === 0) return;

    if (!didInitialScrollRef.current) {
      didInitialScrollRef.current = true;
      userScrolledUpRef.current = false;
      const firstUnread = pendingUnreadScrollRef.current;
      if (firstUnread != null && firstUnread > 0) {
        pendingUnreadScrollRef.current = null;
        window.setTimeout(() => scrollToQuotedMessage(firstUnread), 60);
        return;
      }
      scrollToBottom();
      const t1 = window.setTimeout(scrollToBottom, 0);
      const t2 = window.setTimeout(scrollToBottom, 80);
      const t3 = window.setTimeout(scrollToBottom, 280);
      const t4 = window.setTimeout(scrollToBottom, 600);
      return () => {
        window.clearTimeout(t1);
        window.clearTimeout(t2);
        window.clearTimeout(t3);
        window.clearTimeout(t4);
      };
    }

    if (forceScrollRef.current) {
      forceScrollRef.current = false;
      userScrolledUpRef.current = false;
      scrollToBottom();
      return;
    }

    if (!userScrolledUpRef.current || isNearBottom(el)) {
      userScrolledUpRef.current = false;
      scrollToBottom();
    }
  }, [active, items, isNearBottom, scrollToBottom, scrollToQuotedMessage]);

  const send = useCallback(async () => {
    const t = text.trim();
    if (!t || sending) return;
    if (!apiBase) {
      showAlert("Сборка без VITE_MINIAPP_API_URL.");
      return;
    }
    if (!inTelegram || !initData) {
      showAlert("Открой мини-апп из Telegram.");
      return;
    }
    setSending(true);
    onHaptic?.();
    const replyToId = replyIntent?.replyToMessageId ?? 0;
    setText("");
    setReplyIntent(null);
    forceScrollRef.current = true;
    userScrolledUpRef.current = false;
    try {
      const body: { init_data: string; text: string; reply_to_id?: number } = {
        init_data: initData,
        text: t,
      };
      if (replyToId > 0) body.reply_to_id = replyToId;
      const res = await fetch(`${apiBase}/api/miniapp/pack-group/messages`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const j = (await res.json().catch(() => ({}))) as { error?: string; message?: string; ok?: boolean; reply_text?: string };
      if (!res.ok) {
        if (isModerationError(j.error)) {
          showAlert(moderationUserMessage(j.error, j.message));
          return;
        }
        showAlert(j.error ?? `Ошибка ${res.status}`);
        return;
      }
      await load();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
    } finally {
      setSending(false);
    }
  }, [text, sending, inTelegram, initData, showAlert, load, onHaptic, replyIntent]);

  const reportMessage = useCallback(
    async (messageID: number) => {
      if (!apiBase || !inTelegram || !initData || !messageID) return;
      setReportPosting((p) => ({ ...p, [messageID]: true }));
      try {
        const res = await fetch(`${apiBase}/api/miniapp/pack-group/report`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, message_id: messageID }),
        });
        const j = (await res.json().catch(() => ({}))) as { error?: string };
        if (!res.ok) {
          const errMap: Record<string, string> = {
            not_found: "Сообщение не найдено (обнови чат)",
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
        showAlert("Жалоба отправлена. Админы увидят её в поддержке.");
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      } finally {
        setReportPosting((p) => ({ ...p, [messageID]: false }));
      }
    },
    [inTelegram, initData, showAlert],
  );

  const startReply = useCallback((m: PackGroupMessage) => {
    const authorLabel = m.is_leo ? "Лео" : m.username;
    const excerpt = m.text.length > 100 ? `${m.text.slice(0, 99).trim()}…` : m.text.trim();
    setReplyIntent({ replyToMessageId: m.id, authorLabel, excerpt });
    window.setTimeout(() => inputRef.current?.focus(), 80);
  }, []);

  useEffect(
    () => () => {
      if (highlightTimerRef.current) clearTimeout(highlightTimerRef.current);
    },
    [],
  );

  const removeMine = useCallback(
    async (messageID: number) => {
      if (!apiBase || !inTelegram || !initData || !messageID) return;
      try {
        const res = await fetch(`${apiBase}/api/miniapp/pack-group/messages/delete`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, message_id: messageID }),
        });
        const j = (await res.json().catch(() => ({}))) as { error?: string };
        if (!res.ok) {
          showAlert(j.error ?? `Ошибка ${res.status}`);
          return;
        }
        setItems((prev) => prev.filter((m) => m.id !== messageID));
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      }
    },
    [inTelegram, initData, showAlert],
  );

  const postReaction = useCallback(
    (messageId: number, emoji: string) => {
      if (!apiBase || !inTelegram || !initData || !messageId) return;

      let snapshot: PackGroupMessage[] | null = null;
      setItems((prev) => {
        snapshot = prev;
        return prev.map((m) =>
          m.id === messageId
            ? { ...m, reactions: optimisticTogglePackFeedReaction(m.reactions, emoji) }
            : m,
        );
      });

      void (async () => {
        try {
          const res = await fetch(`${apiBase}/api/miniapp/pack-group/react`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ init_data: initData, message_id: messageId, emoji }),
          });
          const j = (await res.json().catch(() => ({}))) as { error?: string };
          if (!res.ok) {
            if (snapshot) setItems(snapshot);
            showAlert(j.error === "invalid_emoji" ? "Такую реакцию нельзя" : j.error ?? `Ошибка ${res.status}`);
            return;
          }
          await load();
        } catch (e) {
          if (snapshot) setItems(snapshot);
          showAlert(e instanceof Error ? e.message : "Сеть");
        }
      })();
    },
    [inTelegram, initData, showAlert, load],
  );

  const itemsById = useMemo(() => {
    const map = new Map<number, PackGroupMessage>();
    for (const m of items) map.set(m.id, m);
    return map;
  }, [items]);

  if (!apiBase) {
    return <p className="packroom__warn muted">Нет API URL в билде.</p>;
  }

  return (
    <div className="packroom" ref={roomRef}>
      {err && <p className="packroom__err">{err}</p>}
      <div
        className="packroom__log"
        role="log"
        aria-label="Чат стаи"
        ref={logRef}
        onPointerDown={() => {
          window.setTimeout(scrollToBottomIfNear, 120);
          window.setTimeout(scrollToBottomIfNear, 320);
        }}
      >
        <div className="packroom__log-inner">
        {items.map((m) => {
          const mine = !m.is_leo && m.user_id === meId;
          const replyQuote = resolveReplyQuote(m, itemsById);
          const isReply = replyQuote != null;
          const rowTone = m.is_leo ? "packroom__row--leo" : mine ? "packroom__row--me" : "packroom__row--oth";
          const isUnread = unreadMessageIds.has(m.id);
          const canReport = !mine && !m.is_leo;
          const reactions = mergeTrainingFeedReactions(m.reactions);
          const activeReactions = reactions.filter((r) => r.count > 0);
          const avatarUrl = resolveFeedAvatarUrl(m.author_photo_url);
          return (
            <div
              key={m.id}
              data-pack-msg-id={m.id}
              className={`packroom__row ${rowTone}${isReply ? " packroom__row--reply" : ""}${
                highlightMessageId === m.id ? " packroom__row--highlight" : ""
              }${isUnread ? " packroom__row--unread" : ""}`}
            >
              <div className="packroom__row-inner">
                <div className="packroom__ava" aria-hidden>
                  {m.is_leo ? (
                    <img className="packroom__avatar" src={LEO_AVATAR_URL} width={32} height={32} alt="" loading="lazy" />
                  ) : avatarUrl ? (
                    <img
                      className="packroom__avatar"
                      src={avatarUrl}
                      width={32}
                      height={32}
                      alt=""
                      loading="lazy"
                    />
                  ) : (
                    <span className="packroom__avatar-ph">🐾</span>
                  )}
                </div>
                <div className="packroom__content">
                  <div className="packroom__meta-row">
                    <div className="packroom__meta">
                      {m.is_leo ? "Лео" : m.username} · {formatChatTime(m.created_at)}
                    </div>
                  </div>
                  <div className="packroom__bubble-wrap">
                    <div className="packroom__bubble">
                      {isReply && replyQuote != null && (
                        <button
                          type="button"
                          className="packroom__quote"
                          aria-label="Перейти к сообщению, на которое ответили"
                          onClick={() => scrollToQuotedMessage(replyQuote.parentId)}
                        >
                          <span className="packroom__quote-author">{replyQuote.author}</span>
                          {replyQuote.text !== "" && (
                            <span className="packroom__quote-text">{replyQuote.text}</span>
                          )}
                        </button>
                      )}
                      <p className="packroom__bubble-text">{m.text}</p>
                    </div>
                    <div className="packroom__bubble-actions">
                      <button type="button" className="packroom__reply" onClick={() => startReply(m)}>
                        Ответить
                      </button>
                      {mine && (
                        <button
                          type="button"
                          className="packroom__del"
                          onClick={() => {
                            if (!window.confirm("Удалить сообщение?")) return;
                            void removeMine(m.id);
                          }}
                        >
                          Удалить
                        </button>
                      )}
                      <PackGroupMessageMenu
                        reactions={reactions}
                        onReaction={(emoji) => postReaction(m.id, emoji)}
                        canReport={canReport}
                        onReport={() => void reportMessage(m.id)}
                        reporting={Boolean(reportPosting[m.id])}
                      />
                    </div>
                  </div>
                  {activeReactions.length > 0 && (
                    <div className="packroom__react-chips" role="group" aria-label="Реакции на сообщение">
                      {activeReactions.map((r) => (
                        <button
                          key={r.emoji}
                          type="button"
                          className={`act-card__react-btn${r.me ? " act-card__react-btn--mine" : ""}`}
                          onClick={() => postReaction(m.id, r.emoji)}
                          title={Array.isArray(r.voters) && r.voters.length > 0 ? `Лайкнули: ${r.voters.join(", ")}` : undefined}
                        >
                          {r.emoji}
                          {r.count > 0 && <span className="act-card__react-cnt">{r.count}</span>}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          );
        })}
        <div ref={endRef} className="packroom__log-end" aria-hidden />
        </div>
      </div>
      <form
        ref={formRef}
        className="packroom__form"
        onSubmit={(e) => {
          e.preventDefault();
          void send();
        }}
      >
        {replyIntent != null && (
          <div className="packroom__reply-intent">
            <div className="packroom__reply-intent-row">
              <span className="packroom__reply-intent-label">
                Ответ <strong>{replyIntent.authorLabel}</strong>
              </span>
              <button type="button" className="packroom__reply-intent-cancel" onClick={() => setReplyIntent(null)}>
                ✕
              </button>
            </div>
            {replyIntent.excerpt.trim() !== "" && (
              <p className="packroom__reply-intent-excerpt">{replyIntent.excerpt}</p>
            )}
          </div>
        )}
        <div className="packroom__form-row">
        <input
          ref={inputRef}
          className="packroom__input"
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={
            replyIntent != null
              ? `Ответ ${replyIntent.authorLabel}… @leo — чтобы ответил бот`
              : "Сообщение… @leo — чтобы ответил бот"
          }
          maxLength={4000}
          autoComplete="off"
          enterKeyHint="send"
          onFocus={() => {
            userScrolledUpRef.current = false;
            scrollToBottom();
            window.setTimeout(scrollToBottom, 100);
            window.setTimeout(scrollToBottom, 320);
          }}
          onBlur={() => {
            window.setTimeout(scrollToBottomIfNear, 80);
            window.setTimeout(scrollToBottomIfNear, 280);
          }}
        />
        <button type="submit" className="packroom__send" disabled={sending || !text.trim()}>
          {sending ? "…" : "➤"}
        </button>
        </div>
      </form>
    </div>
  );
}
