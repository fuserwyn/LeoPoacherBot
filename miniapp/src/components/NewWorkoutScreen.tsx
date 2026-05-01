import { useState } from "react";
import "./NewWorkoutScreen.css";

const TYPES: { id: string; label: string; emoji: string }[] = [
  { id: "run", label: "Бег", emoji: "🏃" },
  { id: "walk", label: "Ходьба", emoji: "🚶" },
  { id: "bike", label: "Велосипед", emoji: "🚴" },
  { id: "swim", label: "Плавание", emoji: "🏊" },
  { id: "yoga", label: "Йога", emoji: "🧘" },
  { id: "rowing", label: "Гребля", emoji: "🚣" },
  { id: "workout", label: "Воркаут", emoji: "🔥" },
  { id: "crossfit", label: "Кроссфит", emoji: "🎯" },
  { id: "stretch", label: "Растяжка", emoji: "🧎" },
  { id: "dance", label: "Танцы", emoji: "💃" },
  { id: "hiit", label: "HIIT", emoji: "⚡" },
  { id: "cardio", label: "Кардио", emoji: "💓" },
  { id: "kettlebell", label: "Гиря", emoji: "🏋️" },
  { id: "strength", label: "Силовая", emoji: "🏋️" },
  { id: "other", label: "Другое", emoji: "✨" },
];

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
  const [type, setType] = useState("strength");
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
        <h2 className="section-title">Что сделал</h2>
        <textarea
          className="nwo__note"
          value={note}
          onChange={(e) => setNote(e.target.value.slice(0, NOTE_MAX))}
          rows={4}
          maxLength={NOTE_MAX}
          placeholder="Например: жим лёжа, тяга верхнего, пресс. Или ощущения — устал плечами…"
          enterKeyHint="done"
        />
        <p className="nwo__note-cnt muted" aria-live="polite">
          {note.length}/{NOTE_MAX}
        </p>

        <h2 className="section-title" style={{ marginTop: 22 }}>
          Тип
        </h2>
        <div className="chip-group">
          {TYPES.map((t) => (
            <button
              key={t.id}
              type="button"
              className="chip"
              aria-pressed={type === t.id}
              onClick={() => setType(t.id)}
            >
              {t.emoji} {t.label}
            </button>
          ))}
        </div>
        {type === "other" && (
          <div className="nwo__other-field">
            <label className="nwo__other-label muted" htmlFor="nwo-other-type">
              Свой тип активности
            </label>
            <input
              id="nwo-other-type"
              className="nwo__other-input"
              type="text"
              value={otherLabel}
              onChange={(e) => setOtherLabel(e.target.value.slice(0, OTHER_LABEL_MAX))}
              maxLength={OTHER_LABEL_MAX}
              placeholder="Например: пилатес на кольцах…"
              autoComplete="off"
              enterKeyHint="done"
            />
          </div>
        )}

        <h2 className="section-title" style={{ marginTop: 22 }}>
          Длительность
        </h2>
        <p className="nwo__big-min">{min} мин</p>
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
            -5
          </button>
          <button type="button" className="nwo__step" onClick={() => dec(-1)}>
            -1
          </button>
          <button type="button" className="nwo__step" onClick={() => dec(1)}>
            +1
          </button>
          <button type="button" className="nwo__step" onClick={() => dec(5)}>
            +5
          </button>
        </div>

        <h2 className="section-title" style={{ marginTop: 22 }}>
          Интенсивность
        </h2>
        <div className="nwo__intensity">
          {INTENSITIES.map((i) => (
            <button
              key={i.v}
              type="button"
              className="chip nwo__intensity-chip"
              aria-pressed={intensity === i.v}
              onClick={() => setIntensity(i.v)}
            >
              {i.label}
            </button>
          ))}
        </div>

        <h2 className="section-title" style={{ marginTop: 22 }}>
          Фото с тренировки
        </h2>
        <p className="nwo__note-hint muted">Необязательно. Так стая увидит снимок в ленте.</p>
        <input
          className="nwo__file"
          type="file"
          accept="image/*"
          onChange={(e) => setPhoto(e.target.files?.[0] ?? null)}
        />
        {photo && (
          <p className="nwo__note-hint muted" aria-live="polite">
            Выбрано: {photo.name}
          </p>
        )}
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
