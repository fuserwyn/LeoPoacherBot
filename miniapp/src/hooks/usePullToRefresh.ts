import { useEffect, useRef, useState } from "react";

type Status = "idle" | "pulling" | "ready" | "refreshing";

type Options = {
  /** Когда выполнен — сбросить состояние. */
  onRefresh: () => Promise<void> | void;
  /** Минимальная дистанция, после которой релиз триггерит refresh. */
  threshold?: number;
  /** Максимум визуального оттяга. */
  maxPull?: number;
  /** Если false — хук не подключается (например, не активная вкладка). */
  enabled?: boolean;
};

/**
 * Pull-to-refresh поверх window.scroll. Работает только когда window.scrollY === 0
 * и палец движется вниз. Активный (non-passive) touchmove-листенер — чтобы зарубить
 * нативный iOS bounce и не давать странице ехать вниз во время оттяга.
 */
export function usePullToRefresh({ onRefresh, threshold = 70, maxPull = 140, enabled = true }: Options) {
  const [status, setStatus] = useState<Status>("idle");
  const [pull, setPull] = useState(0);
  const startYRef = useRef<number | null>(null);
  const trackingRef = useRef(false);
  const refreshingRef = useRef(false);

  useEffect(() => {
    if (!enabled) return;

    const onTouchStart = (e: TouchEvent) => {
      if (refreshingRef.current) return;
      if (window.scrollY > 0) return;
      const t = e.touches[0];
      if (!t) return;
      startYRef.current = t.clientY;
      trackingRef.current = true;
    };

    const onTouchMove = (e: TouchEvent) => {
      if (!trackingRef.current || refreshingRef.current) return;
      const t = e.touches[0];
      if (!t || startYRef.current == null) return;
      const dy = t.clientY - startYRef.current;
      if (dy <= 0) {
        // Палец вверх — не наша зона, передаём управление странице.
        if (status !== "idle") {
          setStatus("idle");
          setPull(0);
        }
        return;
      }
      if (window.scrollY > 0) {
        trackingRef.current = false;
        if (status !== "idle") {
          setStatus("idle");
          setPull(0);
        }
        return;
      }
      // Демпфирование, чтобы оттяг ощущался как «резинка».
      const damped = Math.min(maxPull, dy * 0.5);
      setPull(damped);
      setStatus(damped >= threshold ? "ready" : "pulling");
      // Гасим нативный bounce только когда реально тянем.
      if (e.cancelable) e.preventDefault();
    };

    const finish = async () => {
      if (!trackingRef.current) return;
      trackingRef.current = false;
      const cur = pullRef.current;
      const reachedThreshold = cur >= threshold;
      if (reachedThreshold) {
        refreshingRef.current = true;
        setStatus("refreshing");
        setPull(threshold);
        try {
          await onRefresh();
        } finally {
          refreshingRef.current = false;
          setStatus("idle");
          setPull(0);
        }
      } else {
        setStatus("idle");
        setPull(0);
      }
      startYRef.current = null;
    };

    const onTouchEnd = () => void finish();
    const onTouchCancel = () => {
      trackingRef.current = false;
      startYRef.current = null;
      if (!refreshingRef.current) {
        setStatus("idle");
        setPull(0);
      }
    };

    document.addEventListener("touchstart", onTouchStart, { passive: true });
    document.addEventListener("touchmove", onTouchMove, { passive: false });
    document.addEventListener("touchend", onTouchEnd, { passive: true });
    document.addEventListener("touchcancel", onTouchCancel, { passive: true });

    return () => {
      document.removeEventListener("touchstart", onTouchStart);
      document.removeEventListener("touchmove", onTouchMove);
      document.removeEventListener("touchend", onTouchEnd);
      document.removeEventListener("touchcancel", onTouchCancel);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, threshold, maxPull, onRefresh]);

  // Поддерживаем актуальный pull в ref для finish() без замыкания на старое значение.
  const pullRef = useRef(0);
  useEffect(() => {
    pullRef.current = pull;
  }, [pull]);

  return { status, pull, threshold };
}
