import "./ActivityCard.css";

export type ActivityCardThreadReply = {
  id: number;
  author: string;
  text: string;
  timeAgo: string;
  isYou: boolean;
};

export type ActivityCardThreadComposer = {
  draft: string;
  onDraftChange: (v: string) => void;
  onSubmit: () => void;
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
}: ActivityCardProps) {
  const showReact = reactions.length > 0 || onReactionClick != null;
  return (
    <article className={`act-card${hideStreak ? " act-card--leo" : ""}`}>
      <header className="act-card__head">
        <div className="act-card__avatar" aria-hidden>
          {avatar}
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
        {(threadReplies.length > 0 || threadComposer) && (
          <div className="act-card__thread">
            <div className="act-card__thread-label">Комментарии</div>
            {threadReplies.length > 0 && (
              <ul className="act-card__thread-list">
                {threadReplies.map((tr) => (
                  <li key={tr.id} className={`act-card__thread-item${tr.isYou ? " act-card__thread-item--you" : ""}`}>
                    <span className="act-card__thread-author">{tr.isYou ? "Ты" : tr.author}</span>
                    <span className="act-card__thread-time muted">{tr.timeAgo}</span>
                    <p className="act-card__thread-text">{tr.text}</p>
                  </li>
                ))}
              </ul>
            )}
            {threadComposer && (
              <div className="act-card__thread-compose">
                <textarea
                  className="act-card__thread-input"
                  rows={2}
                  placeholder="Написать комментарий…"
                  value={threadComposer.draft}
                  onChange={(e) => threadComposer.onDraftChange(e.target.value)}
                  maxLength={2000}
                />
                <button
                  type="button"
                  className="act-card__thread-send"
                  disabled={threadComposer.posting || threadComposer.draft.trim() === ""}
                  onClick={() => threadComposer.onSubmit()}
                >
                  {threadComposer.posting ? "…" : "Отправить"}
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </article>
  );
}
