import { useCallback, useEffect, useRef, useState } from "react";
import { WORKOUT_TYPES as TYPES } from "../lib/workoutCategories";
import "./NewWorkoutScreen.css";

/** Высота шторки по видимой области (клавиатура). Debounce + порог: без ложных перерисовок при микродрожании viewport. */
function useVisibleViewportHeight(): number {
  const [h, setH] = useState(() =>
    typeof window !== "undefined" ? window.visualViewport?.height ?? window.innerHeight : 640,
  );

  useEffect(() => {
    let t: ReturnType<typeof setTimeout> | undefined;

    const read = () => {
      const vv = window.visualViewport;
      const raw =
        vv && vv.height > 0
          ? vv.height
          : typeof window.Telegram?.WebApp?.viewportHeight === "number" &&
              window.Telegram.WebApp.viewportHeight > 0
            ? window.Telegram.WebApp.viewportHeight
            : window.innerHeight;
      // Игнорируем микродрожание (IME, полосы прокрутки) — не триггерим лишний layout
      setH((prev) => (Math.abs(prev - raw) < 12 ? prev : raw));
    };

    const schedule = () => {
      if (t) window.clearTimeout(t);
      t = window.setTimeout(read, 100);
    };

    read();
    const vv = window.visualViewport;
    vv?.addEventListener("resize", schedule);
    window.addEventListener("orientationchange", read);
    const tg = window.Telegram?.WebApp as { onEvent?: (e: string, fn: () => void) => void } | undefined;
    tg?.onEvent?.("viewportChanged", schedule);
    return () => {
      if (t) window.clearTimeout(t);
      vv?.removeEventListener("resize", schedule);
      window.removeEventListener("orientationchange", read);
      const tgOff = window.Telegram?.WebApp as { offEvent?: (e: string, fn: () => void) => void } | undefined;
      tgOff?.offEvent?.("viewportChanged", schedule);
    };
  }, []);

  return h;
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

export function NewWorkoutScreen({ onClose, onSave, showAlert }: Props) {
  const viewportH = useVisibleViewportHeight();
  const bodyRef = useRef<HTMLDivElement>(null);
  const noteTaRef = useRef<HTMLTextAreaElement>(null);
  const noteFocusedRef = useRef(false);

  const [type, setType] = useState<string>("strength");
  const [min, setMin] = useState(15);
  const [minDraft, setMinDraft] = useState("15");
  const [intensity, setIntensity] = useState<1 | 2 | 3 | 4 | 5>(3);
  const [note, setNote] = useState("");
  const [otherLabel, setOtherLabel] = useState("");
  const [busy, setBusy] = useState(false);
  const [photo, setPhoto] = useState<File | null>(null);

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
  }, [viewportH, nudgeTextareaIntoView]);

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

  const bumpMinutes = useCallback((delta: number) => {
    setMin((prev) => {
      const m = clampWorkoutMinutes(prev + delta);
      setMinDraft(String(m));
      return m;
    });
  }, []);
  return (
    <div className="nwo" style={{ height: viewportH, maxHeight: viewportH }}>
      <header className="nwo__head">
        <button type="button" className="nwo__close" onClick={onClose} aria-label="Закрыть">
          ✕
        </button>
        <h1 className="nwo__title">#training_done</h1>
        <span className="nwo__spacer" aria-hidden />
      </header>

      <div className="nwo__body" ref={bodyRef}>
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
              <label className="nwo__other-label muted" htmlFor="nwo-other-type">
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
                    onBlur={() => {
                      if (minDraft === "") applyMinutes(min);
                      else applyMinutes(parseInt(minDraft, 10) || min);
                    }}
                  />
                </div>
              </div>
              <div className="nwo__stepper">
                <button type="button" className="nwo__step" onClick={() => bumpMinutes(-5)}>
                  −5
                </button>
                <button type="button" className="nwo__step" onClick={() => bumpMinutes(-1)}>
                  −1
                </button>
                <button type="button" className="nwo__step" onClick={() => bumpMinutes(1)}>
                  +1
                </button>
                <button type="button" className="nwo__step" onClick={() => bumpMinutes(5)}>
                  +5
                </button>
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

          <div className="nwo__photo-row">
            <h2 className="nwo__sec nwo__sec--inline">Фото</h2>
            <input
              className="nwo__file"
              type="file"
              accept="image/*"
              title="Необязательно — стая увидит снимок в ленте"
              onChange={(e) => setPhoto(e.target.files?.[0] ?? null)}
            />
            {photo ? (
              <span className="nwo__photo-name muted" aria-live="polite">
                {photo.name.length > 18 ? `${photo.name.slice(0, 16)}…` : photo.name}
              </span>
            ) : null}
          </div>
        </div>

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
              window.setTimeout(nudgeTextareaIntoView, 180);
            }}
            onBlur={() => {
              noteFocusedRef.current = false;
            }}
          />
          <p className="nwo__note-cnt muted" aria-live="polite">
            <span className="nwo__note-cnt-inner">
              {note.length}/{NOTE_MAX}
            </span>
          </p>
        </div>
      </div>

      <footer className="nwo__foot">
        <button
          type="button"
          className="nwo__save"
          disabled={busy}
          onClick={async () => {
            if (busy) return;
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
          }}
        >
          {busy ? "Отправка…" : "Отправить отчёт"}
        </button>
      </footer>
    </div>
  );
}
