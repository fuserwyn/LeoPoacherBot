import "./BottomNav.css";

type Tab = "chat" | "feed" | "rules" | "profile";

type Props = {
  active: Tab;
  onChat: () => void;
  onFeed: () => void;
  onRules: () => void;
  onWorkout: () => void;
  onProfile: () => void;
};

export function BottomNav({ active, onChat, onFeed, onRules, onWorkout, onProfile }: Props) {
  return (
    <nav className="bottom-nav" role="navigation" aria-label="Основное меню">
      <button
        type="button"
        className={`bottom-nav__item ${active === "feed" ? "is-active" : ""}`}
        onClick={onFeed}
        aria-current={active === "feed" ? "page" : undefined}
      >
        <span className="bottom-nav__icon" aria-hidden>
          🐆
        </span>
        <span className="bottom-nav__label">Стая</span>
      </button>
      <button
        type="button"
        className={`bottom-nav__item ${active === "chat" ? "is-active" : ""}`}
        onClick={onChat}
        aria-current={active === "chat" ? "page" : undefined}
      >
        <span className="bottom-nav__icon" aria-hidden>
          💬
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
