import { useState } from "react";
import { WORKOUT_TYPES as TYPES } from "../lib/workoutCategories";
import "./NewWorkoutScreen.css";

const INTENSITIES: { v: 1 | 2 | 3 | 4 | 5; label: string }[] = [
  { v: 1, label: "1 · Разминка" },
  { v: 2, label: "2 · Комфортно" },
  { v: 3, label: "3 · Рабочий темп" },
  { v: 4, label: "4 · Тяжело" },
  { v: 5, label: "5 · Макс" },
];

const PRESET_MIN = [5, 15, 30, 45, 60] as const;

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
  const [type, setType] = useState<string>("strength");
  const [min, setMin] = useState(15);
  const [intensity, setIntensity] = useState<1 | 2 | 3 | 4 | 5>(3);
  const [note, setNote] = useState("");
  const [otherLabel, setOtherLabel] = useState("");
  const [busy, setBusy] = useState(false);
  const [photo, setPhoto] = useState<File | null>(null);

  const dec = (d: number) => setMin((m) => Math.max(1, m + d));
  return (
    <div className="nwo">
      <header className="nwo__head">
        <button type="button" className="nwo__close" onClick={onClose} aria-label="Закрыть">
          ✕
        </button>
        <h1 className="nwo__title">#training_done</h1>
        <span className="nwo__spacer" aria-hidden />
      </header>

      <div className="nwo__body">
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
            <div className="nwo__presets">
              {PRESET_MIN.map((p) => (
                <button
                  key={p}
                  type="button"
                  className={`nwo__circle ${min === p ? "is-on" : ""}`}
                  aria-pressed={min === p}
                  onClick={() => setMin(p)}
                >
                  {p}
                </button>
              ))}
            </div>
            <div className="nwo__stepper">
              <button type="button" className="nwo__step" onClick={() => dec(-5)}>
                −5
              </button>
              <button type="button" className="nwo__step" onClick={() => dec(-1)}>
                −1
              </button>
              <button type="button" className="nwo__step" onClick={() => dec(1)}>
                +1
              </button>
              <button type="button" className="nwo__step" onClick={() => dec(5)}>
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

        <div className="nwo__note-block">
          <h2 className="nwo__sec">Что сделал</h2>
          <textarea
            className="nwo__note"
            value={note}
            onChange={(e) => setNote(e.target.value.slice(0, NOTE_MAX))}
            maxLength={NOTE_MAX}
            placeholder="Жим, тяга, пресс…"
            enterKeyHint="done"
          />
          <p className="nwo__note-cnt muted" aria-live="polite">
            {note.length}/{NOTE_MAX}
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
