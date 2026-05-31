import { useEffect, useRef } from "react";

const DRAG_THRESHOLD_PX = 5;

/**
 * Горизонтальный «барабан» на pointer events: надёжнее overflow-x + кнопки в iOS WebView.
 * touch-action: pan-y на контейнере — вертикаль уходит в скролл страницы, горизонталь крутим сами.
 */
export function useHorizontalDragScroll<T extends HTMLElement>() {
  const ref = useRef<T>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    let activePointer: number | null = null;
    let startX = 0;
    let startScrollLeft = 0;
    let dragging = false;

    const finish = (pointerId: number) => {
      if (activePointer !== pointerId) return;
      try {
        el.releasePointerCapture(pointerId);
      } catch {
        /* already released */
      }
      el.classList.remove("is-dragging");
      activePointer = null;
      dragging = false;
    };

    const onPointerDown = (e: PointerEvent) => {
      if (e.pointerType === "mouse" && e.button !== 0) return;
      activePointer = e.pointerId;
      startX = e.clientX;
      startScrollLeft = el.scrollLeft;
      dragging = false;
      el.setPointerCapture(e.pointerId);
    };

    const onPointerMove = (e: PointerEvent) => {
      if (activePointer !== e.pointerId) return;
      const dx = e.clientX - startX;
      if (!dragging) {
        if (Math.abs(dx) < DRAG_THRESHOLD_PX) return;
        dragging = true;
        el.classList.add("is-dragging");
      }
      el.scrollLeft = startScrollLeft - dx;
      e.preventDefault();
      e.stopPropagation();
    };

    const onPointerUp = (e: PointerEvent) => {
      const wasDragging = dragging;
      finish(e.pointerId);
      if (wasDragging) {
        const blockClick = (ev: MouseEvent) => {
          ev.preventDefault();
          ev.stopPropagation();
        };
        el.addEventListener("click", blockClick, { capture: true, once: true });
      }
    };

    el.addEventListener("pointerdown", onPointerDown);
    el.addEventListener("pointermove", onPointerMove);
    el.addEventListener("pointerup", onPointerUp);
    el.addEventListener("pointercancel", onPointerUp);
    return () => {
      el.removeEventListener("pointerdown", onPointerDown);
      el.removeEventListener("pointermove", onPointerMove);
      el.removeEventListener("pointerup", onPointerUp);
      el.removeEventListener("pointercancel", onPointerUp);
    };
  }, []);

  return ref;
}
