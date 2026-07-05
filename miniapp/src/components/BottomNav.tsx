import { useLayoutEffect, useRef, useState } from "react";
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
  /** Отправка текстового сообщения в общую ленту (компоуз-поле). Возвращает true при успехе. */
  onSendMessage?: (text: string) => Promise<boolean> | boolean;
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
  onSendMessage,
}: Props) {
  const leoBadge = leoBadgeCount > 0 ? (leoBadgeCount > 9 ? "9+" : String(leoBadgeCount)) : null;
  const feedBadge = feedBadgeCount > 0 ? (feedBadgeCount > 9 ? "9+" : String(feedBadgeCount)) : null;
  const navRef = useRef<HTMLElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [msg, setMsg] = useState("");
  const [sending, setSending] = useState(false);

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

  // Пока сфокусировано компоуз-поле — держим таббар видимым и над клавиатурой
  // (иначе html.app-keyboard-open прячет весь бар вместе с полем ввода).
  const onComposeFocus = () => {
    document.documentElement.classList.add("app-nav-compose-open");
  };
  const onComposeBlur = () => {
    document.documentElement.classList.remove("app-nav-compose-open");
  };

  const submitMessage = async () => {
    const t = msg.trim();
    if (!t || sending || !onSendMessage) return;
    setSending(true);
    try {
      const ok = await onSendMessage(t);
      if (ok) setMsg("");
    } finally {
      setSending(false);
    }
  };

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
            <span className="bottom-nav__badge bottom-nav__badge--feed" title="Новые ответы в ленте стаи">
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

      {/* Компоуз: поле ввода + «+» — единый блок публикации в ленту.
          Текст → сообщение в ленту; «+» → форма тренировки. */}
      <form
        className="bottom-nav__compose"
        onSubmit={(e) => {
          e.preventDefault();
          void submitMessage();
        }}
      >
        <input
          ref={inputRef}
          className="bottom-nav__compose-input"
          value={msg}
          onChange={(e) => setMsg(e.target.value)}
          onFocus={onComposeFocus}
          onBlur={onComposeBlur}
          placeholder="Написать в ленту…"
          aria-label="Сообщение в ленту стаи"
          maxLength={4000}
          autoComplete="off"
          enterKeyHint="send"
          disabled={sending}
        />
        {msg.trim() ? (
          <button
            type="submit"
            className="bottom-nav__compose-send"
            disabled={sending}
            aria-label="Отправить сообщение в ленту"
          >
            {sending ? "…" : "➤"}
          </button>
        ) : (
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
        )}
      </form>

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
