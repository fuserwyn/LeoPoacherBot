import { useRef, useState, useEffect, useLayoutEffect, useCallback, useMemo } from "react";
import { createPortal } from "react-dom";
import { LikersPopover, useChipPress, useLikersPopover, useLongPress, type Liker, type LikerGroup } from "./Likers";
import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import { resolveFeedAvatarUrl, type VoterDTO } from "../lib/packFeed";
import { streakStreakAriaLabel } from "../lib/streakLabel";
import "./ActivityCard.css";

/** Голоса с бэкенда → строки списка лайкнувших (имя + отрезолвленный URL аватара).
    Работает с обоими форматами: новый {name, photo_url} и старый строка. */
function votersToLikers(voters?: VoterDTO[] | string[]): Liker[] {
  if (!voters) return [];
  return voters.map((v) => {
    if (typeof v === "string") {
      return { name: v, photoUrl: undefined };
    }
    return {
      name: v.name,
      photoUrl: v.photo_url ? resolveFeedAvatarUrl(v.photo_url) : undefined,
    };
  });
}

function avatarLooksLikeImageSrc(avatar: string): boolean {
  const t = avatar.trim();
  if (t.startsWith("/") || t.startsWith("http://") || t.startsWith("https://")) return true;
  return /\.[a-z0-9]{2,4}(\?|$)/i.test(t);
}

/** Запасной глиф, когда фото профиля не загрузилось (нет аватара в Telegram / приватность). */
function avatarFallbackGlyph(name: string): string {
  const t = (name || "").trim();
  const first = t[0] ?? "";
  if (first && /[a-zA-Zа-яА-Я0-9]/.test(first)) return first.toUpperCase();
  if (first && !/[@#]/.test(first)) return first; // уже эмодзи
  return "🐾";
}

/** Аватар участника в треде: при ошибке загрузки фото показывает инициал, а не «битую картинку». */
function ThreadMemberAvatar({ src, name }: { src: string; name: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [src]);
  if (failed) {
    return (
      <span className="act-card__thread-avatar act-card__thread-avatar--member act-card__thread-avatar--fallback" aria-hidden>
        {avatarFallbackGlyph(name)}
      </span>
    );
  }
  return (
    <img
      className="act-card__thread-avatar act-card__thread-avatar--member"
      src={src}
      alt=""
      loading="lazy"
      onError={() => setFailed(true)}
    />
  );
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
  /** Имена лайкнувших комментарий (для поповера «кто лайкнул»). */
  likeVoters?: VoterDTO[] | string[];
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
  onLongPress,
}: {
  r: { emoji: string; count: number; me?: boolean; voters?: VoterDTO[] | string[] };
  disabled?: boolean;
  onPick: (emoji: string) => void;
  /** Зажатие чипа (или hover на десктопе) — показать список всех отреагировавших. */
  onLongPress?: () => void;
}) {
  const press = useChipPress(() => onPick(r.emoji), onLongPress, disabled);
  return (
    <button
      type="button"
      className={`act-card__react-btn${r.me ? " act-card__react-btn--mine" : ""}`}
      disabled={disabled}
      {...press}
    >
      {r.emoji}
      {r.count > 0 && <span className="act-card__react-cnt">{r.count}</span>}
    </button>
  );
}

/** Лайк комментария: тап = поставить/снять, зажатие = показать, кто лайкнул. */
function ThreadLikeButton({
  count,
  mine,
  voters,
  onToggle,
}: {
  count: number;
  mine?: boolean;
  voters?: VoterDTO[] | string[];
  onToggle: () => void;
}) {
  const anchorRef = useRef<HTMLSpanElement>(null);
  const { open, setOpen, style, popRef } = useLikersPopover(anchorRef);
  const hasVoters = Array.isArray(voters) && voters.length > 0;
  const groups: LikerGroup[] = hasVoters ? [{ emoji: "❤️", voters: votersToLikers(voters) }] : [];
  const press = useChipPress(onToggle, hasVoters ? () => setOpen(true) : undefined);
  return (
    <span className="act-card__thread-like-wrap" ref={anchorRef}>
      <button type="button" className={`act-card__thread-like${mine ? " act-card__thread-like--mine" : ""}`} {...press}>
        ❤️ {count}
      </button>
      {open && hasVoters && <LikersPopover groups={groups} popRef={popRef} style={style} label="Кто лайкнул" />}
    </span>
  );
}

export function TrainingReactionsBar({
  reactions,
  onReactionClick,
}: {
  reactions: { emoji: string; count: number; me: boolean; voters?: VoterDTO[] | string[] }[];
  onReactionClick?: (emoji: string) => void;
}) {
  const rowRef = useRef<HTMLDivElement>(null);
  const moreRef = useRef<HTMLDivElement>(null);
  const [visibleCount, setVisibleCount] = useState(reactions.length);
  const { open: likersOpen, setOpen: setLikersOpen, style: likersStyle, popRef } = useLikersPopover(rowRef);
  // Меню доп. реакций «⋯» — портал с тем же позиционированием (не обрезается экраном).
  const { open: menuOpen, setOpen: setMenuOpen, style: menuStyle, popRef: menuPopRef } = useLikersPopover(moreRef);

  // Группы «кто отреагировал»: только эмодзи с реальными голосами.
  const likerGroups: LikerGroup[] = reactions
    .filter((r) => r.count > 0 && Array.isArray(r.voters) && r.voters.length > 0)
    .map((r) => ({ emoji: r.emoji, voters: votersToLikers(r.voters) }));
  const hasLikers = likerGroups.length > 0;

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

  const n = reactions.length;
  const showAll = visibleCount >= n;
  const visible = showAll ? reactions : reactions.slice(0, visibleCount);
  const hidden = showAll ? [] : reactions.slice(visibleCount);

  const pick = (emoji: string) => {
    onReactionClick?.(emoji);
    setMenuOpen(false);
  };

  const openLikers = () => {
    if (hasLikers) setLikersOpen(true);
  };

  return (
    <div className="act-card__react-inner" ref={rowRef}>
      <div className="act-card__react-inline">
        {visible.map((r) => (
          <ReactionChip
            key={r.emoji}
            r={r}
            disabled={onReactionClick == null}
            onPick={pick}
            onLongPress={hasLikers ? openLikers : undefined}
          />
        ))}
        {hidden.length > 0 && (
          <div className="act-card__react-more" ref={moreRef}>
            <button
              type="button"
              className={`act-card__react-toggle${menuOpen ? " act-card__react-toggle--open" : ""}`}
              aria-expanded={menuOpen}
              aria-haspopup="menu"
              aria-label="Ещё реакции"
              onClick={() => setMenuOpen(!menuOpen)}
            >
              ⋯
            </button>
            {menuOpen &&
              createPortal(
                <div className="act-card__react-popover" role="menu" ref={menuPopRef} style={menuStyle}>
                  {hidden.map((r) => (
                    <ReactionChip
                      key={r.emoji}
                      r={r}
                      disabled={onReactionClick == null}
                      onPick={pick}
                      onLongPress={hasLikers ? openLikers : undefined}
                    />
                  ))}
                </div>,
                document.body,
              )}
          </div>
        )}
      </div>
      {likersOpen && hasLikers && <LikersPopover groups={likerGroups} popRef={popRef} style={likersStyle} />}
    </div>
  );
}

export function ReportActionMenu({
  items,
  posting = false,
  className = "",
}: {
  items: { label: string; onClick: () => void; danger?: boolean }[];
  posting?: boolean;
  className?: string;
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

  if (items.length === 0) return null;

  return (
    <div className={`act-card__menu${className ? ` ${className}` : ""}`} ref={wrapRef}>
      <button
        type="button"
        className={`act-card__menu-toggle${open ? " act-card__menu-toggle--open" : ""}`}
        aria-expanded={open}
        aria-haspopup="menu"
        aria-label="Ещё"
        disabled={posting}
        onClick={() => setOpen((v) => !v)}
      >
        {posting ? "…" : "⋯"}
      </button>
      {open && !posting && (
        <div className="act-card__menu-popover" role="menu">
          {items.map((item) => (
            <button
              key={item.label}
              type="button"
              className={`act-card__menu-item${item.danger ? " act-card__menu-item--danger" : ""}`}
              role="menuitem"
              onClick={() => {
                setOpen(false);
                item.onClick();
              }}
            >
              {item.label}
            </button>
          ))}
        </div>
      )}
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
  reactions?: { emoji: string; count: number; me: boolean; voters?: VoterDTO[] | string[] }[];
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
  /** Удалить пост (админ). */
  onAdminDelete?: () => void;
  adminDeletePosting?: boolean;
  /** Зритель — админ: может удалять чужие комментарии в треде (модерация). */
  isAdmin?: boolean;
  /** Пожаловаться на комментарий в треде. */
  onThreadReplyReport?: (threadReplyId: number) => void;
  threadReplyReporting?: Record<number, boolean>;
  /** Есть непрочитанные ответы в треде (участник или Лео). */
  hasUnreadThread?: boolean;
  /** Пользователь открыл тред — сбросить локальный/серверный unread. */
  onThreadOpened?: () => void;
  /** Лео-реплай показан пользователю (тред раскрыт) — для аналитики leo_comment_displayed. */
  onLeoReplyDisplayed?: (threadReplyId: number) => void;
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
  isAdmin = false,
  onLeoReplyDisplayed,
  trainingPhotoUrl,
  onReport,
  reportPosting = false,
  onAdminDelete,
  adminDeletePosting = false,
  onThreadReplyReport,
  threadReplyReporting = {},
  hasUnreadThread = false,
  onThreadOpened,
}: ActivityCardProps) {
  const threadBodyRef = useRef<HTMLDivElement>(null);
  const threadComposeRef = useRef<HTMLDivElement>(null);
  const threadInputRef = useRef<HTMLTextAreaElement>(null);
  const [threadOpen, setThreadOpen] = useState(false);
  const [threadInputFocused, setThreadInputFocused] = useState(false);
  const [lightboxOpen, setLightboxOpen] = useState(false);
  const [photoFailed, setPhotoFailed] = useState(false);
  const [avatarFailed, setAvatarFailed] = useState(false);
  const prevThreadLen = useRef(threadReplies.length);
  const prevThreadOpenRef = useRef(false);

  useEffect(() => {
    if (threadOpen && !prevThreadOpenRef.current) {
      onThreadOpened?.();
      // Тред раскрыт — Лео-реплаи теперь видны. Дедуп по id на стороне родителя.
      if (onLeoReplyDisplayed) {
        for (const tr of threadReplies) {
          if (tr.isLeo && tr.id > 0) onLeoReplyDisplayed(tr.id);
        }
      }
    }
    prevThreadOpenRef.current = threadOpen;
  }, [threadOpen, onThreadOpened, onLeoReplyDisplayed, threadReplies]);

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
    setAvatarFailed(false);
  }, [avatar]);

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
  const cardHeadMenuItems = useMemo(() => {
    const items: { label: string; onClick: () => void; danger?: boolean }[] = [];
    if (onAdminDelete) {
      items.push({ label: "Удалить пост", onClick: onAdminDelete, danger: true });
    }
    if (onReport) {
      items.push({ label: "Пожаловаться на публикацию", onClick: onReport });
    }
    return items;
  }, [onAdminDelete, onReport]);
  const cardHeadMenuPosting = reportPosting || adminDeletePosting;

  // Долгое нажатие на весь пост — показать, кто его лайкнул (все реакции).
  const cardRef = useRef<HTMLElement>(null);
  const cardLikers = useLikersPopover(cardRef);
  const cardLikerGroups: LikerGroup[] = reactions
    .filter((r) => r.count > 0 && Array.isArray(r.voters) && r.voters.length > 0)
    .map((r) => ({ emoji: r.emoji, voters: votersToLikers(r.voters) }));
  const cardPress = useLongPress(() => {
    if (cardLikerGroups.length > 0) cardLikers.setOpen(true);
  });

  return (
    <article
      ref={cardRef}
      {...cardPress}
      className={`act-card${hideStreak ? " act-card--leo" : ""}${lightTone ? " act-card--light" : ""}${threadOpen && hasThread ? " act-card--thread-open" : ""}${trainingPhotoUrl ? " act-card--has-photo" : ""}`}
    >
      {cardLikers.open && cardLikerGroups.length > 0 && (
        <LikersPopover groups={cardLikerGroups} popRef={cardLikers.popRef} style={cardLikers.style} />
      )}
      <header className="act-card__head">
        <div className="act-card__avatar" aria-hidden>
          {avatarLooksLikeImageSrc(avatar) && !avatarFailed ? (
            <img
              className="act-card__avatar-img"
              src={avatar.trim()}
              alt=""
              loading="lazy"
              onError={() => setAvatarFailed(true)}
            />
          ) : avatarLooksLikeImageSrc(avatar) ? (
            avatarFallbackGlyph(name)
          ) : (
            avatar
          )}
        </div>
        <div className="act-card__meta">
          <div className="act-card__row act-card__row--name">
            <span className="act-card__name">{name}</span>
          </div>
          <div className="act-card__row act-card__row--sub">
            <p className="act-card__time">{timeAgo}</p>
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
        </div>
        {cardHeadMenuItems.length > 0 && (
          <ReportActionMenu
            className="act-card__menu--head"
            items={cardHeadMenuItems}
            posting={cardHeadMenuPosting}
          />
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
              className={`act-card__thread-toggle${hasUnreadThread && !threadOpen ? " act-card__thread-toggle--unread" : ""}`}
              aria-expanded={threadOpen}
              onClick={() => setThreadOpen((v) => !v)}
            >
              <span>
                {threadOpen
                  ? "Свернуть комментарии"
                  : threadCount > 0
                    ? `Комментарии · ${threadCount}`
                    : "Комментарии"}
                {hasUnreadThread && !threadOpen ? (
                  <span className="act-card__thread-unread-dot" aria-label="Непрочитанные ответы" />
                ) : null}
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
                                <ThreadMemberAvatar src={tr.authorPhotoUrl.trim()} name={displayAuthor} />
                              ) : null}
                              <div className="act-card__thread-item-main">
                                <div className="act-card__thread-item-head">
                                  <div className="act-card__thread-item-meta">
                                    <span className="act-card__thread-author">{displayAuthor}</span>
                                    <span className="act-card__thread-time muted">{tr.timeAgo}</span>
                                  </div>
                                  <div className="act-card__thread-head-actions">
                                    {!tr.isYou && !leo && onThreadReplyReport != null && (
                                      <ReportActionMenu
                                        className="act-card__menu--thread"
                                        items={[
                                          {
                                            label: "Пожаловаться на комментарий",
                                            onClick: () => onThreadReplyReport(tr.id),
                                          },
                                        ]}
                                        posting={Boolean(threadReplyReporting[tr.id])}
                                      />
                                    )}
                                    {onThreadReplyDelete != null && (tr.isYou || isAdmin) && (
                                      <button
                                        type="button"
                                        className="act-card__thread-del"
                                        disabled={Boolean(threadReplyDeleting[tr.id])}
                                        onClick={() => onThreadReplyDelete(tr.id)}
                                        title={!tr.isYou ? "Удалить как админ" : undefined}
                                      >
                                        {threadReplyDeleting[tr.id] ? "…" : "Удалить"}
                                      </button>
                                    )}
                                  </div>
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
                                {(onThreadReplyLike != null || onThreadReplyIntent != null) && (
                                  <div className="act-card__thread-actions">
                                    {onThreadReplyLike != null && (
                                      <ThreadLikeButton
                                        count={tr.likeCount ?? 0}
                                        mine={tr.likeMe}
                                        voters={tr.likeVoters}
                                        onToggle={() => onThreadReplyLike(tr.id)}
                                      />
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
                      maxLength={500}
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
