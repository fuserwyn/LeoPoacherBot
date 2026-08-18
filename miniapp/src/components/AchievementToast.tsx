import { useCallback, useEffect, useMemo, useState, type CSSProperties, type ReactElement } from "react";

/** Telegram mini-app phone width — above this the toast stays a compact row. */
export const NARROW_TOAST_MAX_WIDTH_PX = 390;
export const NARROW_TOAST_GUTTER_PX = 12;
export const NARROW_TOAST_MIN_TAP_PX = 44;
export const NARROW_TOAST_TITLE_LINES = 2;
export const NARROW_TOAST_CHEER_LINES = 3;

export type AchievementToastItem = {
  id: string;
  title: string;
  subtitle?: string;
  cups?: number;
  streakDays?: number;
};

export type AchievementToastHideReason = "ok" | "empty" | "repeat";

export type AchievementToastNarrowPath = {
  visible: boolean;
  reason: AchievementToastHideReason;
  narrow: boolean;
  stacked: boolean;
  gutterPx: number;
  maxWidthPx: number;
  dismissTapPx: number;
  actionFullWidth: boolean;
  titleMaxLines: number;
  cheerMaxLines: number;
  overflowsHorizontally: boolean;
  cheer: string;
};

type Props = {
  achievement?: AchievementToastItem | null;
  onDismiss?: () => void;
  /** Same id after dismiss — stay hidden (повтор). */
  dismissedId?: string | null;
  /** Injected for tests; live UI reads visualViewport / innerWidth. */
  viewportWidth?: number;
};

/** Leo's one-liner. Empty / whitespace title → "" (отказ / пустой ввод). */
export function leoCheer(title: string, streakDays?: number): string {
  const t = (title ?? "").trim();
  if (!t) return "";
  switch (streakDays) {
    case 7:
      return "Неделя без пропусков. Лео рядом — так и держи.";
    case 14:
      return "Две недели. Характер уже не шутка.";
    case 21:
      return "21 день. Привычка села — Лео кивает.";
    case 30:
      return "Месяц в строю. Лео бьёт хвостом от гордости.";
    case 42:
      return "42 дня. Лео довольно урчит.";
    case 50:
      return "50 дней. Это уже не случайность.";
    case 60:
      return "60 дней. Лео ставит тебе метку уважения.";
    case 90:
      return "Квартал. Стая видит — Лео тоже.";
    case 100:
      return "100 дней. Лео рычит от гордости.";
    case 180:
      return "Полгода. Мало кто доходит — ты дошёл.";
    case 200:
      return "200 дней. Лео снимает шляпу.";
    case 240:
      return "240 дней. Лео считает тебя своим.";
    default:
      return `Лео рядом: ${t}`;
  }
}

export function canShowAchievementToast(
  item: AchievementToastItem | null | undefined,
  dismissedId?: string | null,
): { show: boolean; reason: AchievementToastHideReason } {
  if (!item || !(item.id ?? "").trim() || !(item.title ?? "").trim()) {
    return { show: false, reason: "empty" };
  }
  if (dismissedId && dismissedId === item.id) {
    return { show: false, reason: "repeat" };
  }
  return { show: true, reason: "ok" };
}

export function isNarrowAchievementToastViewport(widthPx: number): boolean {
  return Number.isFinite(widthPx) && widthPx > 0 && widthPx <= NARROW_TOAST_MAX_WIDTH_PX;
}

/**
 * Пользовательский путь на узком экране: тост влезает в ширину,
 * текст не уезжает вбок, закрытие не меньше 44px, над нижней навигацией.
 */
export function planAchievementToastNarrowPath(
  viewportWidthPx: number,
  item: AchievementToastItem | null | undefined,
  dismissedId?: string | null,
): AchievementToastNarrowPath {
  const gate = canShowAchievementToast(item, dismissedId);
  const width =
    Number.isFinite(viewportWidthPx) && viewportWidthPx > 0 ? viewportWidthPx : NARROW_TOAST_MAX_WIDTH_PX;
  const narrow = isNarrowAchievementToastViewport(width);
  const gutterPx = NARROW_TOAST_GUTTER_PX;
  const maxWidthPx = Math.max(0, Math.floor(width - gutterPx * 2));
  const cheer = gate.show && item ? leoCheer(item.title, item.streakDays) : "";

  return {
    visible: gate.show,
    reason: gate.reason,
    narrow,
    stacked: narrow,
    gutterPx,
    maxWidthPx,
    dismissTapPx: NARROW_TOAST_MIN_TAP_PX,
    actionFullWidth: narrow,
    titleMaxLines: NARROW_TOAST_TITLE_LINES,
    cheerMaxLines: NARROW_TOAST_CHEER_LINES,
    overflowsHorizontally: false,
    cheer,
  };
}

function clampLines(lines: number): CSSProperties {
  return {
    display: "-webkit-box",
    WebkitLineClamp: lines,
    WebkitBoxOrient: "vertical",
    overflow: "hidden",
    overflowWrap: "anywhere",
    wordBreak: "break-word",
  };
}

export function AchievementToast({
  achievement,
  onDismiss,
  dismissedId = null,
  viewportWidth,
}: Props): ReactElement | null {
  const [liveWidth, setLiveWidth] = useState(() =>
    typeof window === "undefined"
      ? NARROW_TOAST_MAX_WIDTH_PX
      : Math.floor(window.visualViewport?.width || window.innerWidth || NARROW_TOAST_MAX_WIDTH_PX),
  );

  useEffect(() => {
    if (typeof viewportWidth === "number") return;
    const read = () => {
      setLiveWidth(
        Math.floor(window.visualViewport?.width || window.innerWidth || NARROW_TOAST_MAX_WIDTH_PX),
      );
    };
    read();
    const vv = window.visualViewport;
    vv?.addEventListener("resize", read);
    window.addEventListener("resize", read);
    window.addEventListener("orientationchange", read);
    return () => {
      vv?.removeEventListener("resize", read);
      window.removeEventListener("resize", read);
      window.removeEventListener("orientationchange", read);
    };
  }, [viewportWidth]);

  const width = typeof viewportWidth === "number" ? viewportWidth : liveWidth;
  const path = useMemo(
    () => planAchievementToastNarrowPath(width, achievement, dismissedId),
    [width, achievement, dismissedId],
  );

  const dismiss = useCallback(() => {
    onDismiss?.();
  }, [onDismiss]);

  if (!path.visible || !achievement) return null;

  const cups = typeof achievement.cups === "number" && achievement.cups > 0 ? achievement.cups : 0;

  const shell: CSSProperties = {
    position: "fixed",
    left: path.gutterPx,
    right: path.gutterPx,
    bottom: "calc(var(--feed-bottom-nav-h, 56px) + env(safe-area-inset-bottom, 0px) + 8px)",
    zIndex: 40,
    boxSizing: "border-box",
    maxWidth: path.maxWidthPx,
    width: "auto",
    marginInline: "auto",
    padding: path.stacked ? 12 : "10px 12px",
    display: "flex",
    flexDirection: path.stacked ? "column" : "row",
    alignItems: path.stacked ? "stretch" : "center",
    gap: path.stacked ? 8 : 10,
    overflow: "hidden",
    borderRadius: 14,
    background: "rgba(28, 22, 16, 0.94)",
    color: "#f6efe4",
    boxShadow: "0 8px 24px rgba(0,0,0,0.35)",
  };

  const dismissBtn: CSSProperties = {
    flex: "0 0 auto",
    minWidth: path.dismissTapPx,
    minHeight: path.dismissTapPx,
    width: path.dismissTapPx,
    height: path.dismissTapPx,
    border: 0,
    borderRadius: 10,
    background: "rgba(255,255,255,0.08)",
    color: "#f6efe4",
    fontSize: 18,
    lineHeight: 1,
    cursor: "pointer",
    alignSelf: path.stacked ? "flex-end" : "center",
  };

  const actionBtn: CSSProperties = {
    minHeight: path.dismissTapPx,
    width: path.actionFullWidth ? "100%" : "auto",
    padding: path.actionFullWidth ? "10px 12px" : "8px 14px",
    border: 0,
    borderRadius: 10,
    background: "#e8a317",
    color: "#1c160f",
    fontWeight: 600,
    cursor: "pointer",
  };

  return (
    <div
      className="achievement-toast"
      role="status"
      aria-live="polite"
      data-narrow={path.narrow ? "true" : "false"}
      data-stacked={path.stacked ? "true" : "false"}
      data-reason={path.reason}
      data-overflows={path.overflowsHorizontally ? "true" : "false"}
      style={shell}
    >
      <div
        className="achievement-toast__row"
        style={{
          display: "flex",
          flexDirection: "row",
          alignItems: "flex-start",
          gap: 10,
          minWidth: 0,
          flex: 1,
        }}
      >
        <span className="achievement-toast__emoji" aria-hidden style={{ fontSize: 22, lineHeight: 1.2 }}>
          🏆
        </span>
        <div className="achievement-toast__copy" style={{ minWidth: 0, flex: 1 }}>
          <p
            className="achievement-toast__title"
            style={{ margin: 0, fontWeight: 700, fontSize: path.stacked ? 15 : 16, ...clampLines(path.titleMaxLines) }}
          >
            {achievement.title.trim()}
            {cups > 0 ? ` · +${cups} 🏆` : ""}
          </p>
          {path.cheer ? (
            <p
              className="achievement-toast__cheer"
              style={{ margin: "4px 0 0", fontSize: 13, opacity: 0.9, ...clampLines(path.cheerMaxLines) }}
            >
              {path.cheer}
            </p>
          ) : null}
        </div>
        <button
          type="button"
          className="achievement-toast__dismiss"
          aria-label="Закрыть"
          onClick={dismiss}
          style={dismissBtn}
        >
          ×
        </button>
      </div>
      <button type="button" className="achievement-toast__ack" onClick={dismiss} style={actionBtn}>
        Понятно
      </button>
    </div>
  );
}
