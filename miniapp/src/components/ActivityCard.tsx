import { useRef, useState, useEffect } from "react";
import { LEO_AVATAR_URL } from "../lib/leoAvatar";
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
};

export type ActivityCardThreadComposer = {
  draft: string;
  onDraftChange: (v: string) => void;
  /** Текст берём из поля ввода в момент отправки (надёжнее в Telegram WebView, чем только React state). */
  onSubmit: (text: string) => void;
  posting: boolean;
};

export type ActivityCardProps = {
  avatar: string;
  name: string;
  streak: number;
  /** Для системных карточек (приветствие Лео) — не показывать стрик. */
  hideStreak?: boolean;
  timeAgo: string;
  emoji: string;
  activity: string;
  details: string;
  comment?: string;
  aiText?: string;
  reactions?: { emoji: string; count: number; me?: boolean }[];
  /** Клик по эмодзи (лента training_done). */
  onReactionClick?: (emoji: string) => void;
  /** Тред под отчётом о тренировке. */
  threadReplies?: ActivityCardThreadReply[];
  threadComposer?: ActivityCardThreadComposer;
  /** Удалить свою реплику в треде (id строки miniapp_training_feed_thread). */
  onThreadReplyDelete?: (threadReplyId: number) => void;
  /** Пока идёт DELETE для id реплики. */
  threadReplyDeleting?: Record<number, boolean>;
};

export function ActivityCard({
  avatar,
  name,
  streak,
  hideStreak = false,
  timeAgo,
  emoji,
  activity,
  details,
  comment,
  aiText,
  reactions = [],
  onReactionClick,
  threadReplies = [],
  threadComposer,
  onThreadReplyDelete,
  threadReplyDeleting = {},
}: ActivityCardProps) {
  const threadInputRef = useRef<HTMLTextAreaElement>(null);
  const [threadOpen, setThreadOpen] = useState(false);
  const prevThreadLen = useRef(threadReplies.length);
  useEffect(() => {
    if (threadReplies.length > prevThreadLen.current) {
      setThreadOpen(true);
    }
    prevThreadLen.current = threadReplies.length;
  }, [threadReplies.length]);
  const showReact = reactions.length > 0 || onReactionClick != null;
  const hasThread = threadReplies.length > 0 || threadComposer != null;
  const threadCount = threadReplies.length;
  return (
    <article className={`act-card${hideStreak ? " act-card--leo" : ""}`}>
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
            {!hideStreak && (
              <span className="pill" aria-label={`Серия ${streak} дней`}>
                <span>🔥</span> {streak}
              </span>
            )}
          </div>
          <p className="act-card__time">{timeAgo}</p>
        </div>
      </header>
      <div className="act-card__body">
        <p className="act-card__type">
          <span className="act-card__type-ico">{emoji}</span> {activity}
        </p>
        <p className="act-card__details muted">{details}</p>
        {comment && <p className="act-card__comment">{comment}</p>}
        {aiText && (
          <div className="act-card__ai">
            <div className="act-card__ai-label">AI</div>
            <p className="act-card__ai-text">{aiText}</p>
          </div>
        )}
        {showReact && (
          <div className="act-card__react" role="group" aria-label="Реакции">
            {reactions.map((r) => (
              <button
                type="button"
                className={`act-card__react-btn${r.me ? " act-card__react-btn--mine" : ""}`}
                key={r.emoji}
                disabled={onReactionClick == null}
                onClick={() => onReactionClick?.(r.emoji)}
              >
                {r.emoji}
                {r.count > 0 && <span className="act-card__react-cnt">{r.count}</span>}
              </button>
            ))}
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
                              <p className="act-card__thread-text">{tr.text}</p>
                            </div>
                          </div>
                        </li>
                      );
                    })}
                  </ul>
                )}
                {threadComposer && (
                  <div className="act-card__thread-compose">
                    <textarea
                      ref={threadInputRef}
                      className="act-card__thread-input"
                      rows={2}
                      placeholder="Написать комментарий…"
                      value={threadComposer.draft}
                      onChange={(e) => threadComposer.onDraftChange(e.target.value)}
                      maxLength={2000}
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
      </div>
    </article>
  );
}
