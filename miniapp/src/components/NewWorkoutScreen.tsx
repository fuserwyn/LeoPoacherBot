import { useCallback, useEffect, useRef, useState } from "react";
import { WORKOUT_TYPES as TYPES, type WorkoutCategoryId } from "../lib/workoutCategories";
import { PhotoCropper } from "./PhotoCropper";
import "./NewWorkoutScreen.css";

type ViewportMetrics = {
  /** Высота видимой области (над клавиатурой). */
  visualH: number;
  /** Высота layout viewport — не сжимается при клавиатуре на iOS. */
  layoutH: number;
  /** Отступ снизу до клавиатуры (для fixed-кнопки «Отправить»). */
  keyboardBottom: number;
};

/** Видимая область + inset клавиатуры. Debounce + порог: без ложных перерисовок. */
function useViewportMetrics(): ViewportMetrics {
  const [m, setM] = useState<ViewportMetrics>(() => {
    if (typeof window === "undefined") {
      return { visualH: 640, layoutH: 640, keyboardBottom: 0 };
    }
    const layoutH = window.innerHeight;
    const visualH = window.visualViewport?.height ?? layoutH;
    return { visualH, layoutH, keyboardBottom: 0 };
  });

  useEffect(() => {
    let t: ReturnType<typeof setTimeout> | undefined;

    const read = () => {
      const vv = window.visualViewport;
      const layoutH = Math.max(
        320,
        window.innerHeight ||
          (typeof window.Telegram?.WebApp?.viewportHeight === "number"
            ? window.Telegram.WebApp.viewportHeight
            : 0) ||
          320,
      );
      const visualH = Math.max(
        200,
        vv && vv.height > 0
          ? vv.height
          : typeof window.Telegram?.WebApp?.viewportHeight === "number" &&
              window.Telegram.WebApp.viewportHeight > 0
            ? window.Telegram.WebApp.viewportHeight
            : layoutH,
      );
      const visualOffsetTop = Math.floor(vv?.offsetTop ?? 0);
      const keyboardBottom = Math.max(0, Math.floor(layoutH - visualH - visualOffsetTop));

      setM((prev) => {
        if (
          Math.abs(prev.visualH - visualH) < 12 &&
          Math.abs(prev.layoutH - layoutH) < 12 &&
          Math.abs(prev.keyboardBottom - keyboardBottom) < 8
        ) {
          return prev;
        }
        return { visualH, layoutH, keyboardBottom };
      });
    };

    const schedule = () => {
      if (t) window.clearTimeout(t);
      t = window.setTimeout(read, 100);
    };

    read();
    const vv = window.visualViewport;
    vv?.addEventListener("resize", schedule);
    vv?.addEventListener("scroll", schedule);
    window.addEventListener("orientationchange", read);
    const tg = window.Telegram?.WebApp as { onEvent?: (e: string, fn: () => void) => void } | undefined;
    tg?.onEvent?.("viewportChanged", schedule);
    return () => {
      if (t) window.clearTimeout(t);
      vv?.removeEventListener("resize", schedule);
      vv?.removeEventListener("scroll", schedule);
      window.removeEventListener("orientationchange", read);
      const tgOff = window.Telegram?.WebApp as { offEvent?: (e: string, fn: () => void) => void } | undefined;
      tgOff?.offEvent?.("viewportChanged", schedule);
    };
  }, []);

  return m;
}

const INTENSITIES: { v: 1 | 2 | 3 | 4 | 5; label: string }[] = [
  { v: 1, label: "1 · Разминка" },
  { v: 2, label: "2 · Комфортно" },
  { v: 3, label: "3 · Рабочий темп" },
  { v: 4, label: "4 · Тяжело" },
  { v: 5, label: "5 · Макс" },
];

/** Пресеты минут (до 120); произвольное значение — поле «Своё», лимит как в формуле кубков на бэке. */
const PRESET_MIN = [5, 15, 30, 45, 60, 75, 90, 105, 120] as const;
const WORKOUT_MIN_MIN = 1;
const WORKOUT_MIN_MAX = 480;

function clampWorkoutMinutes(n: number): number {
  if (!Number.isFinite(n)) return WORKOUT_MIN_MIN;
  return Math.min(WORKOUT_MIN_MAX, Math.max(WORKOUT_MIN_MIN, Math.floor(n)));
}

type Props = {
  onClose: () => void;
  showAlert?: (message: string) => void;
  /** Сохранение отчёта: верни false, чтобы не закрывать шторку (например, при ошибке сети). */
  onSave: (payload: {
    type: string;
    min: number;
    intensity: 1 | 2 | 3 | 4 | 5;
    /** Свой вид активности, если выбрано «Другое». */
    otherLabel?: string;
    /** Произвольный текст: упражнения, ощущения — попадает в ленту и в контекст Лео. */
    note: string;
    /** Снимок с тренировки — уходит только вместе с отчётом через multipart API. */
    photo: File | null;
  }) => void | boolean | Promise<void | boolean>;
};

const NOTE_MAX = 1500;
const OTHER_LABEL_MAX = 80;

/** Временно скрыть возможность прикреплять фото. Поменяй на true, чтобы включить обратно. */
const PHOTO_ENABLED = false;

export function NewWorkoutScreen({ onClose, onSave, showAlert }: Props) {
  const { visualH, keyboardBottom } = useViewportMetrics();

  useEffect(() => {
    window.scrollTo(0, 0);
    document.documentElement.scrollTop = 0;
    document.body.scrollTop = 0;
  }, []);
  const bodyRef = useRef<HTMLDivElement>(null);
  const noteTaRef = useRef<HTMLTextAreaElement>(null);
  const noteFocusedRef = useRef(false);
  const [inputFocused, setInputFocused] = useState(false);
  const keyboardOpen = keyboardBottom > 0;
  const showKeyboardBar = inputFocused;

  const [type, setType] = useState<WorkoutCategoryId | "">("");
  const [min, setMin] = useState(15);
  const [minDraft, setMinDraft] = useState("15");
  const [intensity, setIntensity] = useState<1 | 2 | 3 | 4 | 5>(3);
  const [note, setNote] = useState("");
  const [otherLabel, setOtherLabel] = useState("");
  const [busy, setBusy] = useState(false);
  const [photo, setPhoto] = useState<File | null>(null);
  const [pendingCrop, setPendingCrop] = useState<File | null>(null);

  /** Только если низ textarea ушёл под низ скролла — чуть увеличить scrollTop. Без scrollIntoView(start): иначе прыжок к началу формы. */
  const nudgeTextareaIntoView = useCallback(() => {
    const ta = noteTaRef.current;
    const body = bodyRef.current;
    if (!ta || !body) return;
    const bodyRect = body.getBoundingClientRect();
    const taRect = ta.getBoundingClientRect();
    const pad = 10;
    if (taRect.bottom > bodyRect.bottom - pad) {
      body.scrollTop += taRect.bottom - (bodyRect.bottom - pad);
    }
  }, []);

  useEffect(() => {
    if (!noteFocusedRef.current) return;
    const t = window.setTimeout(nudgeTextareaIntoView, 120);
    return () => window.clearTimeout(t);
  }, [visualH, nudgeTextareaIntoView]);

  const handleSubmit = useCallback(async () => {
    if (busy) return;
    if (!type) {
      (showAlert ?? window.alert)("Выбери тип тренировки.");
      return;
    }
    if (type === "other" && !otherLabel.trim()) {
      (showAlert ?? window.alert)("Укажи свой тип активности или выбери категорию из списка.");
      return;
    }
    setBusy(true);
    try {
      const r = await onSave({
        type,
        min,
        intensity,
        otherLabel: type === "other" ? otherLabel.trim() : undefined,
        note: note.trim(),
        photo,
      });
      if (r !== false) onClose();
    } finally {
      setBusy(false);
    }
  }, [busy, type, otherLabel, min, intensity, note, photo, onSave, onClose, showAlert]);

  useEffect(() => {
    let raf = 0;
    const onVV = () => {
      if (!noteFocusedRef.current) return;
      if (raf) cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => {
        raf = 0;
        nudgeTextareaIntoView();
      });
    };
    const vv = window.visualViewport;
    vv?.addEventListener("resize", onVV);
    vv?.addEventListener("scroll", onVV);
    const tg = window.Telegram?.WebApp as { onEvent?: (e: string, fn: () => void) => void } | undefined;
    tg?.onEvent?.("viewportChanged", onVV);
    return () => {
      if (raf) cancelAnimationFrame(raf);
      vv?.removeEventListener("resize", onVV);
      vv?.removeEventListener("scroll", onVV);
      const tgOff = window.Telegram?.WebApp as { offEvent?: (e: string, fn: () => void) => void } | undefined;
      tgOff?.offEvent?.("viewportChanged", onVV);
    };
  }, [nudgeTextareaIntoView]);

  const applyMinutes = useCallback((n: number) => {
    const m = clampWorkoutMinutes(n);
    setMin(m);
    setMinDraft(String(m));
  }, []);

  return (
    <div
      className={`nwo${showKeyboardBar ? " nwo--keyboard" : ""}`}
      style={
        keyboardOpen
          ? { height: `${visualH}px`, maxHeight: `${visualH}px` }
          : undefined
      }
    >
      <header className="nwo__head">
        <div className="nwo__head-title-row">
          <button type="button" className="nwo__close" onClick={onClose} aria-label="Закрыть">
            ✕
          </button>
          <h1 className="nwo__title">Тренировка</h1>
        </div>
      </header>

      <div className="nwo__body" ref={bodyRef}>
        <div className="nwo__note-block">
          <h2 className="nwo__sec">Что сделал</h2>
          <textarea
            ref={noteTaRef}
            className="nwo__note"
            value={note}
            rows={6}
            onChange={(e) => setNote(e.target.value.slice(0, NOTE_MAX))}
            maxLength={NOTE_MAX}
            placeholder="Жим, тяга, пресс…"
            enterKeyHint="done"
            autoCorrect="on"
            spellCheck
            onFocus={() => {
              noteFocusedRef.current = true;
              setInputFocused(true);
              window.setTimeout(nudgeTextareaIntoView, 180);
            }}
            onBlur={() => {
              noteFocusedRef.current = false;
              window.setTimeout(() => {
                if (!document.activeElement?.closest(".nwo")) setInputFocused(false);
              }, 80);
            }}
          />
          <p className="nwo__note-cnt" aria-live="polite">
            <span className="nwo__note-cnt-inner">
              {note.length}/{NOTE_MAX}
            </span>
          </p>
        </div>

        <div className="nwo__upper">
          <h2 className="nwo__sec">Тип</h2>
          <div className="nwo__types-scroll" role="group" aria-label="Тип тренировки">
            {TYPES.map((t) => (
              <button
                key={t.id}
                type="button"
                className="chip nwo__type-chip"
                title={t.label}
                aria-pressed={type === t.id}
                onClick={() => setType(t.id)}
              >
                <span className="nwo__type-emoji" aria-hidden>
                  {t.emoji}
                </span>
                <span className="nwo__type-lbl">{t.label}</span>
              </button>
            ))}
          </div>
          {type === "other" && (
            <div className="nwo__other-field">
              <label className="nwo__other-label" htmlFor="nwo-other-type">
                Свой тип
              </label>
              <input
                id="nwo-other-type"
                className="nwo__other-input"
                type="text"
                value={otherLabel}
                onChange={(e) => setOtherLabel(e.target.value.slice(0, OTHER_LABEL_MAX))}
                maxLength={OTHER_LABEL_MAX}
                placeholder="Пилатес, скалолазанье…"
                autoComplete="off"
                enterKeyHint="done"
                onFocus={() => setInputFocused(true)}
                onBlur={() => {
                  window.setTimeout(() => {
                    if (!document.activeElement?.closest(".nwo")) setInputFocused(false);
                  }, 80);
                }}
              />
            </div>
          )}

          <div className="nwo__mid">
            <div className="nwo__dur">
              <h2 className="nwo__sec">Минуты</h2>
              <div className="nwo__dur-top">
                <span className="nwo__big-min">{min}</span>
                <span className="nwo__big-suf">мин</span>
              </div>
              <div className="nwo__presets-row">
                <div className="nwo__presets" role="group" aria-label="Быстрый выбор минут">
                  {PRESET_MIN.map((p) => (
                    <button
                      key={p}
                      type="button"
                      className={`nwo__circle ${min === p ? "is-on" : ""}`}
                      aria-pressed={min === p}
                      onClick={() => applyMinutes(p)}
                    >
                      {p}
                    </button>
                  ))}
                </div>
                <div className="nwo__min-custom">
                  <label className="nwo__min-custom-lbl" htmlFor="nwo-min-custom">
                    Своё
                  </label>
                  <input
                    id="nwo-min-custom"
                    className="nwo__min-custom-input"
                    type="text"
                    inputMode="numeric"
                    autoComplete="off"
                    enterKeyHint="done"
                    maxLength={3}
                    aria-label={`Минуты, от ${WORKOUT_MIN_MIN} до ${WORKOUT_MIN_MAX}`}
                    placeholder={`${WORKOUT_MIN_MIN}–${WORKOUT_MIN_MAX}`}
                    value={minDraft}
                    onChange={(e) => {
                      const s = e.target.value.replace(/\D/g, "").slice(0, 3);
                      setMinDraft(s);
                      if (s !== "") applyMinutes(parseInt(s, 10));
                    }}
                    onFocus={() => setInputFocused(true)}
                    onBlur={() => {
                      if (minDraft === "") applyMinutes(min);
                      else applyMinutes(parseInt(minDraft, 10) || min);
                      window.setTimeout(() => {
                        if (!document.activeElement?.closest(".nwo")) setInputFocused(false);
                      }, 80);
                    }}
                  />
                </div>
              </div>
            </div>

            <div className="nwo__int-wrap">
              <h2 className="nwo__sec">Интенсивность</h2>
              <div className="nwo__intensity-row" role="group" aria-label="Интенсивность">
                {INTENSITIES.map((i) => (
                  <button
                    key={i.v}
                    type="button"
                    className={`nwo__int-btn ${intensity === i.v ? "is-on" : ""}`}
                    aria-pressed={intensity === i.v}
                    aria-label={i.label}
                    title={i.label}
                    onClick={() => setIntensity(i.v)}
                  >
                    {i.v}
                  </button>
                ))}
              </div>
            </div>
          </div>
        </div>

        {PHOTO_ENABLED && (
          <div className="nwo__photo-row">
            <h2 className="nwo__sec nwo__sec--inline">Фото</h2>
            <input
              className="nwo__file"
              type="file"
              accept="image/*"
              title="Необязательно — стая увидит снимок в ленте"
              onChange={(e) => {
                const f = e.target.files?.[0] ?? null;
                if (f) setPendingCrop(f);
                e.target.value = "";
              }}
            />
            {photo ? (
              <>
                <span className="nwo__photo-name" aria-live="polite">
                  {photo.name.length > 18 ? `${photo.name.slice(0, 16)}…` : photo.name}
                </span>
                <button
                  type="button"
                  className="nwo__photo-edit"
                  onClick={() => setPendingCrop(photo)}
                >
                  Обрезать
                </button>
                <button
                  type="button"
                  className="nwo__photo-edit nwo__photo-edit--ghost"
                  onClick={() => setPhoto(null)}
                  aria-label="Убрать фото"
                >
                  ✕
                </button>
              </>
            ) : null}
          </div>
        )}
      </div>

      <footer className="nwo__foot" hidden={showKeyboardBar}>
        <button type="button" className="nwo__save" disabled={busy} onClick={() => void handleSubmit()}>
          {busy ? "Отправка…" : "Отправить отчёт"}
        </button>
      </footer>

      {showKeyboardBar ? (
        <div className="nwo__keyboard-bar" role="toolbar" aria-label="Отправка отчёта">
          <button
            type="button"
            className="nwo__keyboard-send"
            disabled={busy}
            onClick={() => void handleSubmit()}
          >
            {busy ? "Отправка…" : "Отправить"}
          </button>
        </div>
      ) : null}
      {PHOTO_ENABLED && pendingCrop ? (
        <PhotoCropper
          file={pendingCrop}
          onCancel={() => setPendingCrop(null)}
          onConfirm={(cropped) => {
            setPhoto(cropped);
            setPendingCrop(null);
          }}
        />
      ) : null}
    </div>
  );
}
