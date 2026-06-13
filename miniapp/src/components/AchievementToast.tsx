import { useEffect, useState } from "react";
import { achievementLabel, parseAchievementKey, type AchievementKey } from "../lib/achievements";
import "./AchievementToast.css";

type Props = {
  /** Ключ открытой ачивки, напр. "streak-7" / "workout-10". */
  achievementKey: AchievementKey;
  /** Вызывается, когда тост скрылся (по таймеру или тапу) — родитель снимает его с очереди. */
  onDone: () => void;
};

const VISIBLE_MS = 4500;
const EXIT_MS = 280;

/**
 * Празднующий тост «Ачивка получена!». Показывается поверх всего, авто-скрывается,
 * тап закрывает раньше. Очередь и дедуп — на стороне App (по ключам ачивок).
 */
export function AchievementToast({ achievementKey, onDone }: Props) {
  const [leaving, setLeaving] = useState(false);
  const parsed = parseAchievementKey(achievementKey);
  const isStreak = parsed?.kind === "streak";

  useEffect(() => {
    const hide = window.setTimeout(() => setLeaving(true), VISIBLE_MS);
    return () => window.clearTimeout(hide);
  }, []);

  useEffect(() => {
    if (!leaving) return;
    const done = window.setTimeout(onDone, EXIT_MS);
    return () => window.clearTimeout(done);
  }, [leaving, onDone]);

  return (
    <div
      className={`achievement-toast${leaving ? " is-leaving" : ""}`}
      role="status"
      aria-live="polite"
      onClick={() => setLeaving(true)}
    >
      <div className="achievement-toast__icon" aria-hidden>
        {isStreak ? "🐾" : "⭐️"}
      </div>
      <div className="achievement-toast__text">
        <div className="achievement-toast__title">Ачивка получена!</div>
        <div className="achievement-toast__label">{achievementLabel(achievementKey)}</div>
      </div>
    </div>
  );
}
