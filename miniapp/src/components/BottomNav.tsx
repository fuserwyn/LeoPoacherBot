import "./BottomNav.css";

type Tab = "chat" | "feed" | "rules" | "profile";

type Props = {
  active: Tab;
  /** Непрочитанные фрагменты из очереди лички Лео (предупреждения, ответы). */
  leoBadgeCount?: number;
  /** Непрочитанные комментарии к твоим отчётам в ленте стаи. */
  feedBadgeCount?: number;
  onChat: () => void;
  onFeed: () => void;
  onRules: () => void;
  onWorkout: () => void;
  onProfile: () => void;
};

export function BottomNav({
  active,
  leoBadgeCount = 0,
  feedBadgeCount = 0,
  onChat,
  onFeed,
  onRules,
  onWorkout,
  onProfile,
}: Props) {
  const leoBadge = leoBadgeCount > 0 ? (leoBadgeCount > 9 ? "9+" : String(leoBadgeCount)) : null;
  const feedBadge = feedBadgeCount > 0 ? (feedBadgeCount > 9 ? "9+" : String(feedBadgeCount)) : null;
  return (
    <nav className="bottom-nav" role="navigation" aria-label="Основное меню">
      <button
        type="button"
        className={`bottom-nav__item ${active === "feed" ? "is-active" : ""}`}
        onClick={onFeed}
        aria-current={active === "feed" ? "page" : undefined}
        aria-label={feedBadge ? `Стая, непрочитанных комментариев: ${feedBadgeCount}` : "Стая"}
      >
        <span className="bottom-nav__icon-wrap" aria-hidden>
          <span className="bottom-nav__icon">🐆</span>
          {feedBadge && (
            <span className="bottom-nav__badge bottom-nav__badge--feed" title="Новый комментарий к твоей тренировке">
              {feedBadge}
            </span>
          )}
        </span>
        <span className="bottom-nav__label">Стая</span>
      </button>
      <button
        type="button"
        className={`bottom-nav__item ${active === "chat" ? "is-active" : ""}`}
        onClick={onChat}
        aria-current={active === "chat" ? "page" : undefined}
        aria-label={leoBadge ? `Лео, непрочитанных: ${leoBadgeCount}` : "Лео"}
      >
        <span className="bottom-nav__icon-wrap" aria-hidden>
          <span className="bottom-nav__icon">💬</span>
          {leoBadge && (
            <span className="bottom-nav__badge" title="Новое от Лео">
              {leoBadge}
            </span>
          )}
        </span>
        <span className="bottom-nav__label">Лео</span>
      </button>
      <button type="button" className="bottom-nav__fab" onClick={onWorkout} aria-label="Отметить тренировку (#training_done)">
        <span className="bottom-nav__fab-plus" aria-hidden>
          +
        </span>
      </button>
      <button
        type="button"
        className={`bottom-nav__item ${active === "rules" ? "is-active" : ""}`}
        onClick={onRules}
        aria-current={active === "rules" ? "page" : undefined}
      >
        <span className="bottom-nav__icon" aria-hidden>
          📜
        </span>
        <span className="bottom-nav__label">Правила</span>
      </button>
      <button
        type="button"
        className={`bottom-nav__item ${active === "profile" ? "is-active" : ""}`}
        onClick={onProfile}
        aria-current={active === "profile" ? "page" : undefined}
      >
        <span className="bottom-nav__icon" aria-hidden>
          👤
        </span>
        <span className="bottom-nav__label">Профиль</span>
      </button>
    </nav>
  );
}
