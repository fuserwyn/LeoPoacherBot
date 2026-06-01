import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type RefObject,
  type TouchEvent as ReactTouchEvent,
} from "react";

/** Группа отреагировавших по одной эмодзи. */
export type LikerGroup = { emoji: string; voters: string[] };

/**
 * Управляет поповером «кто отреагировал»: открытие/закрытие, закрытие по тапу вне
 * (кроме якоря), Esc и горизонтальный сдвиг, чтобы окно целиком влезало в экран.
 * `anchorRef` — контейнер-триггер (на нём должен быть position: relative).
 */
export function useLikersPopover(anchorRef: RefObject<HTMLElement | null>) {
  const [open, setOpen] = useState(false);
  const [shift, setShift] = useState(0);
  const popRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent | TouchEvent) => {
      if (anchorRef.current?.contains(e.target as Node)) return;
      setOpen(false);
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", close);
    document.addEventListener("touchstart", close);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("touchstart", close);
      document.removeEventListener("keydown", esc);
    };
  }, [open, anchorRef]);

  useLayoutEffect(() => {
    if (!open) {
      setShift(0);
      return;
    }
    const el = popRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const margin = 8;
    let s = 0;
    if (rect.left < margin) s = margin - rect.left;
    else if (rect.right > window.innerWidth - margin) s = window.innerWidth - margin - rect.right;
    if (s !== 0) setShift(s);
  }, [open]);

  return { open, setOpen, shift, popRef };
}

/** Поповер со списком отреагировавших (сгруппирован по эмодзи, со скроллом). */
export function LikersPopover({
  groups,
  popRef,
  shift,
  label = "Кто отреагировал",
}: {
  groups: LikerGroup[];
  popRef: RefObject<HTMLDivElement | null>;
  shift: number;
  label?: string;
}) {
  return (
    <div
      className="act-card__likers"
      role="dialog"
      aria-label={label}
      ref={popRef}
      style={{ "--pop-shift": `${shift}px` } as CSSProperties}
    >
      <div className="act-card__likers-head">{label}</div>
      <div className="act-card__likers-scroll">
        {groups.map((g) => (
          <div key={g.emoji} className="act-card__likers-grp">
            <div className="act-card__likers-grp-head">
              <span className="act-card__likers-grp-emoji">{g.emoji}</span>
              <span className="act-card__likers-count">{g.voters.length}</span>
            </div>
            {g.voters.map((name, i) => (
              <div key={`${name}-${i}`} className="act-card__likers-item">
                {name}
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * Пропсы для чипа/кнопки реакции: короткий тап = `onTap` (поставить/снять),
 * зажатие (~350мс) или hover на десктопе = `onLongPress` (просмотр списка).
 * preventDefault в touchend гасит синтетический click, поэтому реакция не двоится
 * и не переключается при долгом нажатии.
 */
export function useChipPress(onTap: () => void, onLongPress?: () => void, disabled?: boolean) {
  const lpTimer = useRef<number | null>(null);
  const longPressed = useRef(false);
  const moved = useRef(false);
  const startXY = useRef<{ x: number; y: number } | null>(null);
  const recentTouch = useRef(0);
  const canLong = typeof onLongPress === "function";

  const clearTimer = () => {
    if (lpTimer.current != null) {
      window.clearTimeout(lpTimer.current);
      lpTimer.current = null;
    }
  };

  return {
    onClick: () => {
      if (Date.now() - recentTouch.current < 600) return; // синтетический click после тача
      if (!disabled) onTap();
    },
    onTouchStart: (e: ReactTouchEvent) => {
      longPressed.current = false;
      moved.current = false;
      const t = e.touches[0];
      startXY.current = t ? { x: t.clientX, y: t.clientY } : null;
      clearTimer();
      if (canLong) {
        lpTimer.current = window.setTimeout(() => {
          longPressed.current = true;
          onLongPress?.();
        }, 350);
      }
    },
    onTouchMove: (e: ReactTouchEvent) => {
      const t = e.touches[0];
      const s = startXY.current;
      if (t && s && (Math.abs(t.clientX - s.x) > 8 || Math.abs(t.clientY - s.y) > 8)) {
        moved.current = true;
        clearTimer();
      }
    },
    onTouchEnd: (e: ReactTouchEvent) => {
      clearTimer();
      recentTouch.current = Date.now();
      if (longPressed.current) {
        e.preventDefault();
        return;
      }
      if (moved.current) return;
      e.preventDefault();
      if (!disabled) onTap();
    },
    onTouchCancel: clearTimer,
    onMouseEnter: () => {
      if (Date.now() - recentTouch.current < 600) return;
      onLongPress?.();
    },
  };
}
