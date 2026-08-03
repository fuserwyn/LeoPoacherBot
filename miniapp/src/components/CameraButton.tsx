import { useRef, type ChangeEvent, type ReactNode } from "react";

/** Кнопка «Сделать фото»: открывает камеру устройства напрямую (input capture) рядом с
 *  обычным выбором из галереи. Переиспользует тот же обработчик, что и галерейный input —
 *  он читает e.target.files[0], поэтому камере отдельная логика не нужна. Значение input
 *  сбрасывается после выбора, чтобы повторный тот же кадр снова триггерил onChange. */
type CameraButtonProps = {
  onChange: (e: ChangeEvent<HTMLInputElement>) => void;
  disabled?: boolean;
  className?: string;
  ariaLabel?: string;
  title?: string;
  /** Содержимое кнопки; по умолчанию 📷. */
  children?: ReactNode;
};

export function CameraButton({
  onChange,
  disabled,
  className,
  ariaLabel = "Сделать фото",
  title,
  children,
}: CameraButtonProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  return (
    <>
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        capture="environment"
        hidden
        tabIndex={-1}
        aria-hidden
        disabled={disabled}
        onChange={(e) => {
          onChange(e);
          if (inputRef.current) inputRef.current.value = "";
        }}
      />
      <button
        type="button"
        className={className}
        disabled={disabled}
        aria-label={ariaLabel}
        title={title ?? ariaLabel}
        onClick={() => inputRef.current?.click()}
      >
        {children ?? "📷"}
      </button>
    </>
  );
}
