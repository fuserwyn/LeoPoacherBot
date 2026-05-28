import { useLayoutEffect, useRef } from "react";
import { measureBottomNavOffset } from "../lib/bottomNavOffset";
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
  onAddWorkout: () => void;
  onRules: () => void;
  onProfile: () => void;
};

export function BottomNav({
  active,
  leoBadgeCount = 0,
  feedBadgeCount = 0,
  onChat,
  onFeed,
  onAddWorkout,
  onRules,
  onProfile,
}: Props) {
  const leoBadge = leoBadgeCount > 0 ? (leoBadgeCount > 9 ? "9+" : String(leoBadgeCount)) : null;
  const feedBadge = feedBadgeCount > 0 ? (feedBadgeCount > 9 ? "9+" : String(feedBadgeCount)) : null;
  const navRef = useRef<HTMLElement>(null);

  // Публикуем высоту таббара в --bottom-nav-h, чтобы оверлеи (например, #training_done) могли её зарезервировать.
  useLayoutEffect(() => {
    const el = navRef.current;
    if (!el) return;
    const root = document.documentElement;
    const write = () => {
      root.style.setProperty("--bottom-nav-h", `${Math.ceil(el.getBoundingClientRect().height)}px`);
      root.style.setProperty("--bottom-nav-stack", `${measureBottomNavOffset(el)}px`);
    };
    write();
    const ro = new ResizeObserver(write);
    ro.observe(el);
    return () => {
      ro.disconnect();
      root.style.removeProperty("--bottom-nav-h");
      root.style.removeProperty("--bottom-nav-stack");
    };
  }, []);

  return (
    <nav ref={navRef} className="bottom-nav" role="navigation" aria-label="Основное меню">
      <button
        type="button"
        className={`bottom-nav__item ${active === "feed" ? "is-active" : ""}`}
        onClick={onFeed}
        aria-current={active === "feed" ? "page" : undefined}
        aria-label={feedBadge ? `Стая, непрочитанных: ${feedBadgeCount}` : "Стая"}
      >
        <span className="bottom-nav__icon-wrap" aria-hidden>
          <span className="bottom-nav__icon">🐆</span>
          {feedBadge && (
            <span className="bottom-nav__badge bottom-nav__badge--feed" title="Новые ответы в ленте или чате стаи">
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
      <button
        type="button"
        className="bottom-nav__add"
        onClick={onAddWorkout}
        aria-label="Добавить тренировку"
        title="Добавить тренировку"
      >
        <span className="bottom-nav__add-plus" aria-hidden>
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
