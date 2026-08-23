import { useEffect, useState } from "react";
import { achievementLabel, parseAchievementKey, type AchievementKey } from "../lib/achievements";
import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import "./AchievementToast.css";

type Props = {
  /** Ключ открытой ачивки, напр. "streak-7" / "workout-10". */
  achievementKey: AchievementKey;
  /** Вызывается, когда тост скрылся (по таймеру или тапу) — родитель снимает его с очереди. */
  onDone: () => void;
};

const VISIBLE_MS = 5000;
const EXIT_MS = 320;

/** Тёплая фраза от Лео — добавляет эмоций поздравлению. */
function leoCheer(isStreak: boolean): string {
  return isStreak ? "Так держать — Лео тобой гордится!" : "Ты просто зверь! Лео даёт лапу 🐾";
}

/**
 * Празднующий поп-ап «Ачивка получена!» по центру экрана: аватарка Лео с лапкой,
 * искорки и тёплая фраза. Показывается поверх всего, авто-скрывается, тап закрывает
 * раньше. Очередь и дедуп — на стороне App (по ключам ачивок).
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
      className={`achievement-overlay${leaving ? " is-leaving" : ""}`}
      role="status"
      aria-live="polite"
      onClick={() => setLeaving(true)}
    >
      <div className={`achievement-toast${leaving ? " is-leaving" : ""}`}>
        {/* Искорки-конфетти вокруг лапки */}
        <div className="achievement-toast__sparks" aria-hidden>
          <span className="achievement-toast__spark achievement-toast__spark--1">✨</span>
          <span className="achievement-toast__spark achievement-toast__spark--2">🎉</span>
          <span className="achievement-toast__spark achievement-toast__spark--3">⭐️</span>
          <span className="achievement-toast__spark achievement-toast__spark--4">✨</span>
        </div>

        <div className="achievement-toast__badge">
          <img className="achievement-toast__leo" src={LEO_AVATAR_URL} alt="" />
          <span className="achievement-toast__paw" aria-hidden>
            🐾
          </span>
        </div>

        <div className="achievement-toast__title">Ачивка получена!</div>
        <div className="achievement-toast__label">{achievementLabel(achievementKey)}</div>
        <div className="achievement-toast__cheer">{leoCheer(isStreak)}</div>
      </div>
    </div>
  );
}
