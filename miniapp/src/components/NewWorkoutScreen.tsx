import { useCallback, useEffect, useRef, useState } from "react";
import { WORKOUT_TYPES as TYPES, type WorkoutCategoryId } from "../lib/workoutCategories";
import { PhotoCropper } from "./PhotoCropper";
import { hapticImpact, hapticNotification } from "../lib/haptics";
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
    let raf = 0;

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
          Math.abs(prev.visualH - visualH) < 4 &&
          Math.abs(prev.layoutH - layoutH) < 4 &&
          Math.abs(prev.keyboardBottom - keyboardBottom) < 4
        ) {
          return prev;
        }
        return { visualH, layoutH, keyboardBottom };
      });
    };

    // Throttle через rAF, а не debounce 100мс: iOS анимирует клавиатуру ~250мс
    // и шлёт серию resize — при дебаунсе замер происходил только после её конца,
    // поэтому кнопка-панель «не сразу» вставала над клавиатурой. Замер на каждом
    // кадре даёт панели отслеживать вьюпорт почти в реальном времени (быстро и плавно).
    const schedule = () => {
      if (raf) cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => {
        raf = 0;
        read();
      });
    };

    read();
    const vv = window.visualViewport;
    vv?.addEventListener("resize", schedule);
    vv?.addEventListener("scroll", schedule);
    window.addEventListener("orientationchange", read);
    const tg = window.Telegram?.WebApp as { onEvent?: (e: string, fn: () => void) => void } | undefined;
    tg?.onEvent?.("viewportChanged", schedule);
    return () => {
      if (raf) cancelAnimationFrame(raf);
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
  /** Клик по «Хочу вносить не только спорт» — best-effort счётчик интереса к фиче. */
  onNonSportInterest?: () => void;
  /** Сохранение отчёта: верни false, чтобы не закрывать шторку (например, при ошибке сети). */
  onSave: (payload: {
    /** Один или несколько видов спорта (мультивыбор). Кубки — за самый эффективный. */
    types: WorkoutCategoryId[];
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

/** Прикрепление фото к отчёту. Фото грузятся в Cloudflare R2 (см. R2_* в env). */
const PHOTO_ENABLED = true;

export function NewWorkoutScreen({ onClose, onSave, showAlert, onNonSportInterest }: Props) {
  const { visualH } = useViewportMetrics();

  useEffect(() => {
    window.scrollTo(0, 0);
    document.documentElement.scrollTop = 0;
    document.body.scrollTop = 0;
  }, []);
  const bodyRef = useRef<HTMLDivElement>(null);
  const noteTaRef = useRef<HTMLTextAreaElement>(null);
  const otherInputRef = useRef<HTMLInputElement>(null);
  const noteFocusedRef = useRef(false);
  // Какое поле сейчас в фокусе — чтобы подматывать над клавиатурой именно его
  // (и «Описание», и «Свой тип»), а не только текстовую заметку.
  const activeFieldRef = useRef<HTMLElement | null>(null);
  const [inputFocused, setInputFocused] = useState(false);
  const showKeyboardBar = inputFocused;

  const [types, setTypes] = useState<WorkoutCategoryId[]>([]);
  const otherSelected = types.includes("other");
  const toggleType = useCallback((id: WorkoutCategoryId) => {
    setTypes((prev) => (prev.includes(id) ? prev.filter((t) => t !== id) : [...prev, id]));
  }, []);
  const [min, setMin] = useState(15);
  const [minDraft, setMinDraft] = useState("15");
  const [intensity, setIntensity] = useState<1 | 2 | 3 | 4 | 5>(3);
  const [note, setNote] = useState("");
  const [otherLabel, setOtherLabel] = useState("");
  const [busy, setBusy] = useState(false);
  const [photo, setPhoto] = useState<File | null>(null);
  const [pendingCrop, setPendingCrop] = useState<File | null>(null);

  /** Только если низ поля ушёл под низ скролла — чуть увеличить scrollTop. Без scrollIntoView(start): иначе прыжок к началу формы. */
  const nudgeIntoView = useCallback((el: HTMLElement | null) => {
    const body = bodyRef.current;
    if (!el || !body) return;
    const bodyRect = body.getBoundingClientRect();
    const elRect = el.getBoundingClientRect();
    const pad = 10;
    if (elRect.bottom > bodyRect.bottom - pad) {
      body.scrollTop += elRect.bottom - (bodyRect.bottom - pad);
    }
  }, []);
  const nudgeActiveIntoView = useCallback(() => nudgeIntoView(activeFieldRef.current), [nudgeIntoView]);

  useEffect(() => {
    if (!activeFieldRef.current) return;
    const t = window.setTimeout(nudgeActiveIntoView, 120);
    return () => window.clearTimeout(t);
  }, [visualH, nudgeActiveIntoView]);

  // При выборе «Другое» сразу ставим фокус в поле своего типа и подматываем его
  // над клавиатурой — иначе оно появляется под клавиатурой и не видно, куда вводить.
  useEffect(() => {
    if (!otherSelected) return;
    const el = otherInputRef.current;
    if (!el) return;
    el.focus({ preventScroll: true });
    window.setTimeout(nudgeActiveIntoView, 180);
  }, [otherSelected, nudgeActiveIntoView]);

  const handleSubmit = useCallback(async () => {
    if (busy) return;
    if (types.length === 0) {
      (showAlert ?? window.alert)("Выбери тип тренировки.");
      return;
    }
    if (otherSelected && !otherLabel.trim()) {
      (showAlert ?? window.alert)("Укажи свой тип активности или выбери категорию из списка.");
      return;
    }
    setBusy(true);
    hapticImpact("medium");
    try {
      const r = await onSave({
        types,
        min,
        intensity,
        otherLabel: otherSelected ? otherLabel.trim() : undefined,
        note: note.trim(),
        photo,
      });
      if (r !== false) {
        hapticNotification("success");
        onClose();
      } else {
        hapticNotification("error");
      }
    } finally {
      setBusy(false);
    }
  }, [busy, types, otherSelected, otherLabel, min, intensity, note, photo, onSave, onClose, showAlert]);

  useEffect(() => {
    let raf = 0;
    const onVV = () => {
      if (!activeFieldRef.current) return;
      if (raf) cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() => {
        raf = 0;
        nudgeActiveIntoView();
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
  }, [nudgeActiveIntoView]);

  const [nonSportSent, setNonSportSent] = useState(false);
  const handleNonSportInterest = useCallback(() => {
    if (!nonSportSent) {
      setNonSportSent(true);
      onNonSportInterest?.();
    }
    (showAlert ?? window.alert)("Спасибо за ваш интерес! Мы работаем над новым функционалом.");
  }, [nonSportSent, onNonSportInterest, showAlert]);

  const applyMinutes = useCallback((n: number) => {
    const m = clampWorkoutMinutes(n);
    setMin(m);
    setMinDraft(String(m));
  }, []);

  return (
    <div
      className={`nwo${showKeyboardBar ? " nwo--keyboard" : ""}`}
      style={
        // Высоту клампим строго пока показана клавиатурная панель (showKeyboardBar),
        // тем же флагом, что футер/класс .nwo--keyboard. Иначе при тапе по чипу
        // inputFocused слетает раньше, чем дебаунс-метрики keyboardBottom, и кнопка
        // «Отправить» на миг зависает посередине, пока .nwo ещё зажат высотой клавиатуры.
        showKeyboardBar
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
          <h2 className="nwo__sec">Описание</h2>
          <textarea
            ref={noteTaRef}
            className="nwo__note"
            value={note}
            rows={3}
            onChange={(e) => setNote(e.target.value.slice(0, NOTE_MAX))}
            maxLength={NOTE_MAX}
            placeholder="Жим, тяга, пресс…"
            enterKeyHint="done"
            autoCorrect="on"
            spellCheck
            onFocus={() => {
              noteFocusedRef.current = true;
              activeFieldRef.current = noteTaRef.current;
              setInputFocused(true);
              window.setTimeout(nudgeActiveIntoView, 180);
            }}
            onBlur={() => {
              noteFocusedRef.current = false;
              if (activeFieldRef.current === noteTaRef.current) activeFieldRef.current = null;
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
          {PHOTO_ENABLED && (
            <div className="nwo__photo-row">
              <label className="nwo__photo-add">
                <input
                  className="nwo__file-hidden"
                  type="file"
                  accept="image/*"
                  title="Необязательно — стая увидит снимок в ленте"
                  onChange={(e) => {
                    const f = e.target.files?.[0] ?? null;
                    if (f) setPendingCrop(f);
                    e.target.value = "";
                  }}
                />
                <span className="nwo__photo-add-ico" aria-hidden>
                  📷
                </span>
                {photo ? "Заменить фото" : "Добавить фото"}
              </label>
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

        <div className="nwo__upper">
          <h2 className="nwo__sec">Тип</h2>
          <div className="nwo__types-scroll" role="group" aria-label="Тип тренировки (можно выбрать несколько)">
            {TYPES.map((t) => (
              <button
                key={t.id}
                type="button"
                className="chip nwo__type-chip"
                title={t.label}
                aria-pressed={types.includes(t.id)}
                onClick={() => toggleType(t.id)}
              >
                <span className="nwo__type-emoji" aria-hidden>
                  {t.emoji}
                </span>
                <span className="nwo__type-lbl">{t.label}</span>
              </button>
            ))}
          </div>
          <button type="button" className="nwo__non-sport" onClick={handleNonSportInterest}>
            Хочу вносить не только спорт
          </button>
          {otherSelected && (
            <div className="nwo__other-field">
              <label className="nwo__other-label" htmlFor="nwo-other-type">
                Свой тип
              </label>
              <input
                id="nwo-other-type"
                ref={otherInputRef}
                className="nwo__other-input"
                type="text"
                value={otherLabel}
                onChange={(e) => setOtherLabel(e.target.value.slice(0, OTHER_LABEL_MAX))}
                maxLength={OTHER_LABEL_MAX}
                placeholder="Пилатес, скалолазанье…"
                autoComplete="off"
                enterKeyHint="done"
                onFocus={() => {
                  activeFieldRef.current = otherInputRef.current;
                  setInputFocused(true);
                  window.setTimeout(nudgeActiveIntoView, 180);
                }}
                onBlur={() => {
                  if (activeFieldRef.current === otherInputRef.current) activeFieldRef.current = null;
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
