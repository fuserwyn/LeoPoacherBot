import { useRef, useState, useEffect, useLayoutEffect, useCallback } from "react";
import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import { streakStreakAriaLabel } from "../lib/streakLabel";
import "./ActivityCard.css";

function avatarLooksLikeImageSrc(avatar: string): boolean {
  const t = avatar.trim();
  if (t.startsWith("/") || t.startsWith("http://") || t.startsWith("https://")) return true;
  return /\.[a-z0-9]{2,4}(\?|$)/i.test(t);
}

export type ActivityCardThreadReply = {
  id: number;
  author: string;
  text: string;
  timeAgo: string;
  isYou: boolean;
  /** Ответ Лео из треда ленты (тот же текст, что персонально в ЛС). */
  isLeo?: boolean;
  /** Аватар участника (TG), если уже синхронился с сервера. */
  authorPhotoUrl?: string;
  /** Цитата родителя (reply). */
  replyTo?: { author: string; text: string; isLeo?: boolean };
  likeCount?: number;
  likeMe?: boolean;
};

export type ActivityCardThreadComposer = {
  draft: string;
  onDraftChange: (v: string) => void;
  /** Текст берём из поля ввода в момент отправки (надёжнее в Telegram WebView, чем только React state). */
  onSubmit: (text: string) => void;
  posting: boolean;
};

/** Минимум реакций в строке до кнопки «⋯», если типов больше (остальное — в попапе); строка может переноситься (flex-wrap). */
const TRAINING_REACTIONS_MIN_INLINE = 5;

/** Ширина строки + кнопка «ещё»: оценка px на кнопку (эмодзи + отступы + счётчик). */
function trainingReactionVisibleCount(rowWidth: number, total: number): number {
  if (total <= 0) return 0;
  if (rowWidth <= 0) return total;
  const chip = 40;
  const gap = 6;
  const moreBtn = 38;
  const widthAll = total * chip + Math.max(0, total - 1) * gap;
  if (widthAll <= rowWidth) return total;

  let best = 1;
  for (let k = total - 1; k >= 1; k--) {
    const used = k * chip + Math.max(0, k - 1) * gap + gap + moreBtn;
    if (used <= rowWidth) {
      best = k;
      break;
    }
  }
  if (total <= TRAINING_REACTIONS_MIN_INLINE) {
    return Math.min(total, best);
  }
  return Math.min(total, Math.max(TRAINING_REACTIONS_MIN_INLINE, best));
}

function ReactionChip({
  r,
  disabled,
  onPick,
}: {
  r: { emoji: string; count: number; me?: boolean; voters?: string[] };
  disabled?: boolean;
  onPick: (emoji: string) => void;
}) {
  return (
    <button
      type="button"
      className={`act-card__react-btn${r.me ? " act-card__react-btn--mine" : ""}`}
      disabled={disabled}
      onClick={() => onPick(r.emoji)}
      title={Array.isArray(r.voters) && r.voters.length > 0 ? `Лайкнули: ${r.voters.join(", ")}` : undefined}
    >
      {r.emoji}
      {r.count > 0 && <span className="act-card__react-cnt">{r.count}</span>}
    </button>
  );
}

function TrainingReactionsBar({
  reactions,
  onReactionClick,
}: {
  reactions: { emoji: string; count: number; me?: boolean; voters?: string[] }[];
  onReactionClick?: (emoji: string) => void;
}) {
  const rowRef = useRef<HTMLDivElement>(null);
  const moreRef = useRef<HTMLDivElement>(null);
  const [visibleCount, setVisibleCount] = useState(reactions.length);
  const [menuOpen, setMenuOpen] = useState(false);

  useLayoutEffect(() => {
    const el = rowRef.current;
    if (!el) return;
    const measure = () => {
      const w = el.clientWidth;
      if (w <= 0) return;
      setVisibleCount(trainingReactionVisibleCount(w, reactions.length));
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [reactions.length]);

  useEffect(() => {
    if (!menuOpen) return;
    const close = (e: MouseEvent | TouchEvent) => {
      const t = e.target as Node;
      if (moreRef.current?.contains(t)) return;
      setMenuOpen(false);
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    document.addEventListener("mousedown", close);
    document.addEventListener("touchstart", close);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("touchstart", close);
      document.removeEventListener("keydown", esc);
    };
  }, [menuOpen]);

  const n = reactions.length;
  const showAll = visibleCount >= n;
  const visible = showAll ? reactions : reactions.slice(0, visibleCount);
  const hidden = showAll ? [] : reactions.slice(visibleCount);

  const pick = (emoji: string) => {
    onReactionClick?.(emoji);
    setMenuOpen(false);
  };

  return (
    <div className="act-card__react-inner" ref={rowRef}>
      <div className="act-card__react-inline">
        {visible.map((r) => (
          <ReactionChip key={r.emoji} r={r} disabled={onReactionClick == null} onPick={pick} />
        ))}
        {hidden.length > 0 && (
          <div className="act-card__react-more" ref={moreRef}>
            <button
              type="button"
              className={`act-card__react-toggle${menuOpen ? " act-card__react-toggle--open" : ""}`}
              aria-expanded={menuOpen}
              aria-haspopup="menu"
              aria-label="Ещё реакции"
              onClick={() => setMenuOpen((v) => !v)}
            >
              ⋯
            </button>
            {menuOpen && (
              <div className="act-card__react-popover" role="menu">
                {hidden.map((r) => (
                  <ReactionChip key={r.emoji} r={r} disabled={onReactionClick == null} onPick={pick} />
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

export type ActivityCardProps = {
  avatar: string;
  name: string;
  streak: number;
  /** Для системных карточек (приветствие Лео) — не показывать стрик. */
  hideStreak?: boolean;
  /** Светлая подсветка для специальных карточек админа/системы. */
  lightTone?: boolean;
  timeAgo: string;
  emoji: string;
  activity: string;
  details: string;
  comment?: string;
  poll?: {
    totalVotes: number;
    options: { label: string; votes: number; selected?: boolean }[];
    onVote?: (optionIndex: number) => void;
  };
  aiText?: string;
  reactions?: { emoji: string; count: number; me?: boolean; voters?: string[] }[];
  /** Клик по эмодзи (лента training_done). */
  onReactionClick?: (emoji: string) => void;
  /** Тред под отчётом о тренировке. */
  threadReplies?: ActivityCardThreadReply[];
  threadComposer?: ActivityCardThreadComposer;
  /** Удалить свою реплику в треде (id строки miniapp_training_feed_thread). */
  onThreadReplyDelete?: (threadReplyId: number) => void;
  /** Выбрать сообщение для ответа (как Reply в Telegram). */
  onThreadReplyIntent?: (payload: { replyToThreadId: number; authorLabel: string; excerpt: string }) => void;
  onThreadReplyLike?: (threadReplyId: number) => void;
  /** Режим ответа на сообщение в треде (до отправки). */
  threadReplyIntent?: { replyToThreadId: number; authorLabel: string; excerpt: string } | null;
  onCancelThreadReplyIntent?: () => void;
  /** Фото к отчёту #training_done (URL с бэкенда). */
  trainingPhotoUrl?: string;
  /** Состояние удаления строк треда (id → отправка). */
  threadReplyDeleting?: Record<number, boolean>;
  /** Пожаловаться на пост (не свой). */
  onReport?: () => void;
  reportPosting?: boolean;
  /** Пожаловаться на комментарий в треде. */
  onThreadReplyReport?: (threadReplyId: number) => void;
  threadReplyReporting?: Record<number, boolean>;
};

export function ActivityCard({
  avatar,
  name,
  streak,
  hideStreak = false,
  lightTone = false,
  timeAgo,
  emoji,
  activity,
  details,
  comment,
  poll,
  aiText,
  reactions = [],
  onReactionClick,
  threadReplies = [],
  threadComposer,
  onThreadReplyDelete,
  onThreadReplyIntent,
  onThreadReplyLike,
  threadReplyIntent,
  onCancelThreadReplyIntent,
  threadReplyDeleting = {},
  trainingPhotoUrl,
  onReport,
  reportPosting = false,
  onThreadReplyReport,
  threadReplyReporting = {},
}: ActivityCardProps) {
  const threadBodyRef = useRef<HTMLDivElement>(null);
  const threadComposeRef = useRef<HTMLDivElement>(null);
  const threadInputRef = useRef<HTMLTextAreaElement>(null);
  const [threadOpen, setThreadOpen] = useState(false);
  const [threadInputFocused, setThreadInputFocused] = useState(false);
  const [lightboxOpen, setLightboxOpen] = useState(false);
  const [photoFailed, setPhotoFailed] = useState(false);
  const prevThreadLen = useRef(threadReplies.length);

  const scrollThreadComposeIntoView = useCallback(() => {
    const target = threadComposeRef.current ?? threadInputRef.current;
    if (!target) return;
    const scroll = () => {
      target.scrollIntoView({ block: "end", inline: "nearest" });
    };
    scroll();
    window.setTimeout(scroll, 80);
    window.setTimeout(scroll, 280);
  }, []);

  const onThreadComposeFocus = useCallback(() => {
    if (!threadOpen) setThreadOpen(true);
    setThreadInputFocused(true);
    scrollThreadComposeIntoView();
  }, [scrollThreadComposeIntoView, threadOpen]);

  const onThreadComposeBlur = useCallback(() => {
    window.setTimeout(() => {
      const active = document.activeElement;
      if (active === threadInputRef.current || threadComposeRef.current?.contains(active)) return;
      setThreadInputFocused(false);
    }, 120);
  }, []);

  useEffect(() => {
    if (!threadInputFocused) return;
    const vv = window.visualViewport;
    if (!vv) return;
    const onViewportChange = () => scrollThreadComposeIntoView();
    vv.addEventListener("resize", onViewportChange);
    vv.addEventListener("scroll", onViewportChange);
    return () => {
      vv.removeEventListener("resize", onViewportChange);
      vv.removeEventListener("scroll", onViewportChange);
    };
  }, [threadInputFocused, scrollThreadComposeIntoView]);

  useEffect(() => {
    setPhotoFailed(false);
  }, [trainingPhotoUrl]);

  useEffect(() => {
    if (threadOpen && threadReplies.length > prevThreadLen.current) {
      requestAnimationFrame(() => {
        const body = threadBodyRef.current;
        if (body) body.scrollTop = body.scrollHeight;
      });
    }
    prevThreadLen.current = threadReplies.length;
  }, [threadOpen, threadReplies.length]);

  useEffect(() => {
    if (!lightboxOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setLightboxOpen(false);
    };
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prev;
    };
  }, [lightboxOpen]);

  const showReact = reactions.length > 0 || onReactionClick != null;
  const hasThread = threadReplies.length > 0 || threadComposer != null;
  const threadCount = threadReplies.length;
  const showStreak = !hideStreak && name.trim() !== "Админ";
  return (
    <article
      className={`act-card${hideStreak ? " act-card--leo" : ""}${lightTone ? " act-card--light" : ""}${threadOpen && hasThread ? " act-card--thread-open" : ""}${trainingPhotoUrl ? " act-card--has-photo" : ""}`}
    >
      <header className="act-card__head">
        <div className="act-card__avatar" aria-hidden>
          {avatarLooksLikeImageSrc(avatar) ? (
            <img className="act-card__avatar-img" src={avatar.trim()} alt="" loading="lazy" />
          ) : (
            avatar
          )}
        </div>
        <div className="act-card__meta">
          <div className="act-card__row">
            <span className="act-card__name">{name}</span>
            {showStreak && (
              <span
                className="pill pill--streak"
                aria-label={streakStreakAriaLabel(streak)}
                title={streakStreakAriaLabel(streak)}
              >
                <span className="pill__streak-word">Стрик</span>
                <span className="pill__streak-num">{streak}</span>
              </span>
            )}
          </div>
          <p className="act-card__time">{timeAgo}</p>
        </div>
        {onReport != null && (
          <button
            type="button"
            className="act-card__report"
            disabled={reportPosting}
            onClick={() => onReport()}
            aria-label="Пожаловаться"
            title="Пожаловаться"
          >
            {reportPosting ? "…" : "⚑"}
          </button>
        )}
      </header>
      <div className="act-card__body">
        <p className="act-card__type">
          <span className="act-card__type-ico">{emoji}</span> {activity}
        </p>
        {details.trim() !== "" && <p className="act-card__details">{details}</p>}
        {comment && <p className="act-card__comment">{comment}</p>}
        {poll && poll.options.length > 0 && (
          <div className="act-card__poll" role="group" aria-label="Опрос">
            {poll.options.map((option, optionIndex) => (
              <button
                key={`${optionIndex}-${option.label}`}
                type="button"
                className={`act-card__poll-option${option.selected ? " act-card__poll-option--selected" : ""}`}
                onClick={() => poll.onVote?.(optionIndex)}
              >
                <span className="act-card__poll-label">{option.label}</span>
                <span className="act-card__poll-votes">{option.votes}</span>
              </button>
            ))}
            <p className="act-card__poll-total">{poll.totalVotes} голосов</p>
          </div>
        )}
        {trainingPhotoUrl && !photoFailed ? (
          <div
            className="act-card__photo-wrap"
            role="button"
            tabIndex={0}
            aria-label="Открыть фото"
            onClick={() => setLightboxOpen(true)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                setLightboxOpen(true);
              }
            }}
          >
            <img
              className="act-card__photo"
              src={trainingPhotoUrl}
              alt=""
              loading="lazy"
              referrerPolicy="no-referrer"
              onError={() => setPhotoFailed(true)}
            />
          </div>
        ) : null}
        {aiText && (
          <div className="act-card__ai">
            <div className="act-card__ai-label">AI</div>
            <p className="act-card__ai-text">{aiText}</p>
          </div>
        )}
      </div>
      {showReact && (
        <div className="act-card__react" role="group" aria-label="Реакции">
          <TrainingReactionsBar reactions={reactions} onReactionClick={onReactionClick} />
        </div>
      )}
      {hasThread && (
        <div className="act-card__thread">
            <button
              type="button"
              className="act-card__thread-toggle"
              aria-expanded={threadOpen}
              onClick={() => setThreadOpen((v) => !v)}
            >
              <span>
                {threadOpen
                  ? "Свернуть комментарии"
                  : threadCount > 0
                    ? `Комментарии · ${threadCount}`
                    : "Комментарии"}
              </span>
              <span className="act-card__thread-chevron" aria-hidden>
                {threadOpen ? "▲" : "▼"}
              </span>
            </button>
            {threadOpen && (
              <>
                <div className="act-card__thread-body" ref={threadBodyRef}>
                  {threadReplies.length > 0 && (
                    <ul className="act-card__thread-list">
                      {threadReplies.map((tr) => {
                        const leo = Boolean(tr.isLeo);
                        const displayAuthor = leo ? "Лео" : tr.isYou ? "Ты" : tr.author;
                        return (
                          <li
                            key={tr.id}
                            className={`act-card__thread-item${tr.isYou ? " act-card__thread-item--you" : ""}${leo ? " act-card__thread-item--leo" : ""}`}
                          >
                            <div className={`act-card__thread-item-inner${leo || (!leo && tr.authorPhotoUrl?.trim()) ? " act-card__thread-item-inner--has-avatar" : ""}`}>
                              {leo ? (
                                <img
                                  className="act-card__thread-avatar"
                                  src={LEO_AVATAR_URL}
                                  alt=""
                                  loading="lazy"
                                />
                              ) : tr.authorPhotoUrl?.trim() ? (
                                <img
                                  className="act-card__thread-avatar act-card__thread-avatar--member"
                                  src={tr.authorPhotoUrl.trim()}
                                  alt=""
                                  loading="lazy"
                                />
                              ) : null}
                              <div className="act-card__thread-item-main">
                                <div className="act-card__thread-item-head">
                                  <div className="act-card__thread-item-meta">
                                    <span className="act-card__thread-author">{displayAuthor}</span>
                                    <span className="act-card__thread-time muted">{tr.timeAgo}</span>
                                  </div>
                                  {tr.isYou && !leo && onThreadReplyDelete != null && (
                                    <button
                                      type="button"
                                      className="act-card__thread-del"
                                      disabled={Boolean(threadReplyDeleting[tr.id])}
                                      onClick={() => onThreadReplyDelete(tr.id)}
                                    >
                                      {threadReplyDeleting[tr.id] ? "…" : "Удалить"}
                                    </button>
                                  )}
                                </div>
                                {tr.replyTo != null &&
                                  ((tr.replyTo.text || "").trim() !== "" || (tr.replyTo.author || "").trim() !== "") && (
                                    <div className="act-card__thread-quote" aria-label="Ответ на сообщение">
                                      <span className="act-card__thread-quote-author muted">
                                        {tr.replyTo.isLeo ? "Лео" : tr.replyTo.author}
                                      </span>
                                      {(tr.replyTo.text || "").trim() !== "" && (
                                        <p className="act-card__thread-quote-text">{(tr.replyTo.text || "").trim()}</p>
                                      )}
                                    </div>
                                  )}
                                <p className="act-card__thread-text">{tr.text}</p>
                                {(onThreadReplyIntent != null || onThreadReplyReport != null) && (
                                  <div className="act-card__thread-actions">
                                    {onThreadReplyLike != null && (
                                      <button
                                        type="button"
                                        className={`act-card__thread-like${tr.likeMe ? " act-card__thread-like--mine" : ""}`}
                                        onClick={() => onThreadReplyLike(tr.id)}
                                      >
                                        ❤️ {tr.likeCount ?? 0}
                                      </button>
                                    )}
                                    {onThreadReplyIntent != null && (
                                      <button
                                        type="button"
                                        className="act-card__thread-answer"
                                        onClick={() => {
                                          onThreadReplyIntent({
                                            replyToThreadId: tr.id,
                                            authorLabel: displayAuthor,
                                            excerpt:
                                              tr.text.length > 100 ? `${tr.text.slice(0, 99).trim()}…` : tr.text.trim(),
                                          });
                                          window.setTimeout(() => {
                                            threadInputRef.current?.focus();
                                          }, 80);
                                        }}
                                      >
                                        Ответить
                                      </button>
                                    )}
                                    {onThreadReplyReport != null && !tr.isYou && !leo && (
                                      <button
                                        type="button"
                                        className="act-card__thread-report"
                                        disabled={Boolean(threadReplyReporting[tr.id])}
                                        onClick={() => onThreadReplyReport(tr.id)}
                                      >
                                        {threadReplyReporting[tr.id] ? "…" : "Пожаловаться"}
                                      </button>
                                    )}
                                  </div>
                                )}
                              </div>
                            </div>
                          </li>
                        );
                      })}
                    </ul>
                  )}
                </div>
                {threadComposer && (
                    <div className="act-card__thread-compose" ref={threadComposeRef}>
                    {threadReplyIntent != null && (
                      <div className="act-card__reply-intent">
                        <div className="act-card__reply-intent-row">
                          <span className="act-card__reply-intent-label">
                            Ответ <strong>{threadReplyIntent.authorLabel}</strong>
                          </span>
                          {onCancelThreadReplyIntent != null && (
                            <button
                              type="button"
                              className="act-card__reply-intent-cancel"
                              aria-label="Отменить ответ"
                              onClick={() => onCancelThreadReplyIntent()}
                            >
                              ✕
                            </button>
                          )}
                        </div>
                        {(threadReplyIntent.excerpt || "").trim() !== "" && (
                          <p className="act-card__reply-intent-snippet muted">{threadReplyIntent.excerpt}</p>
                        )}
                      </div>
                    )}
                    <textarea
                      ref={threadInputRef}
                      className="act-card__thread-input"
                      rows={2}
                      placeholder={
                        threadReplyIntent
                          ? `Сообщение для ${threadReplyIntent.authorLabel}…`
                          : "Написать комментарий…"
                      }
                      value={threadComposer.draft}
                      onChange={(e) => threadComposer.onDraftChange(e.target.value)}
                      maxLength={2000}
                      onFocus={onThreadComposeFocus}
                      onBlur={onThreadComposeBlur}
                      onKeyDown={(e) => {
                        if (e.key !== "Enter" || (!e.ctrlKey && !e.metaKey)) return;
                        e.preventDefault();
                        const raw = threadInputRef.current?.value ?? threadComposer.draft;
                        if (threadComposer.posting || raw.trim() === "") return;
                        threadComposer.onSubmit(raw);
                      }}
                    />
                    <button
                      type="button"
                      className="act-card__thread-send"
                      disabled={threadComposer.posting}
                      onClick={() => {
                        const raw = threadInputRef.current?.value ?? threadComposer.draft;
                        threadComposer.onSubmit(raw);
                      }}
                    >
                      {threadComposer.posting ? "…" : "Отправить"}
                    </button>
                  </div>
                )}
              </>
            )}
          </div>
        )}
      {lightboxOpen && trainingPhotoUrl ? (
        <div
          className="act-card__lightbox"
          role="dialog"
          aria-modal="true"
          onClick={() => setLightboxOpen(false)}
        >
          <button
            type="button"
            className="act-card__lightbox-close"
            aria-label="Закрыть"
            onClick={(e) => {
              e.stopPropagation();
              setLightboxOpen(false);
            }}
          >
            ✕
          </button>
          <img
            className="act-card__lightbox-img"
            src={trainingPhotoUrl}
            alt=""
            referrerPolicy="no-referrer"
            onClick={(e) => e.stopPropagation()}
          />
        </div>
      ) : null}
    </article>
  );
}
