import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type CSSProperties, type ReactElement } from "react";

/** Telegram mini-app phone width — above this the control stays a compact icon. */
export const NARROW_CAMERA_MAX_WIDTH_PX = 390;
export const NARROW_CAMERA_GUTTER_PX = 12;
export const NARROW_CAMERA_MIN_TAP_PX = 44;
export const NARROW_CAMERA_LABEL = "Фото";
export const NARROW_CAMERA_HINT = "Камера или галерея";

export type CameraPickInput = {
  name: string;
  size: number;
  type: string;
  lastModified?: number;
};

export type CameraButtonHideReason = "ok" | "empty" | "repeat";

export type CameraButtonNarrowPath = {
  accepted: boolean;
  reason: CameraButtonHideReason;
  narrow: boolean;
  stacked: boolean;
  gutterPx: number;
  maxWidthPx: number;
  tapPx: number;
  actionFullWidth: boolean;
  labelVisible: boolean;
  hintVisible: boolean;
  capture: "environment" | undefined;
  overflowsHorizontally: boolean;
  label: string;
  hint: string;
};

type Props = {
  onPick?: (file: File) => void;
  onReject?: (reason: CameraButtonHideReason) => void;
  busy?: boolean;
  disabled?: boolean;
  /** Same pick after dismiss — stay idle (повтор). */
  dismissedKey?: string | null;
  /** Injected for tests; live UI reads visualViewport / innerWidth. */
  viewportWidth?: number;
  accept?: string;
  fileName?: string;
};

export function isNarrowCameraViewport(widthPx: number): boolean {
  return Number.isFinite(widthPx) && widthPx > 0 && widthPx <= NARROW_CAMERA_MAX_WIDTH_PX;
}

export function isCameraImagePick(pick: CameraPickInput): boolean {
  if (!(pick.name ?? "").trim()) return false;
  const t = (pick.type ?? "").trim().toLowerCase();
  if (t.startsWith("image/")) return true;
  return /\.(jpe?g|png|webp|gif|heic|heif)$/i.test(pick.name);
}

export function cameraPickKey(pick: CameraPickInput): string {
  return `${(pick.name ?? "").trim()}\0${pick.size}\0${pick.lastModified ?? 0}`;
}

export function cameraPickInputFromFile(file: File | null | undefined): CameraPickInput | null {
  if (!file) return null;
  return {
    name: file.name ?? "",
    size: file.size ?? 0,
    type: file.type ?? "",
    lastModified: file.lastModified,
  };
}

/** Empty / whitespace / non-image → empty. Same key after dismiss → repeat. */
export function canAcceptCameraPick(
  pick: CameraPickInput | null | undefined,
  dismissedKey?: string | null,
): { accept: boolean; reason: CameraButtonHideReason } {
  if (!pick || !(pick.name ?? "").trim() || !Number.isFinite(pick.size) || pick.size <= 0 || !isCameraImagePick(pick)) {
    return { accept: false, reason: "empty" };
  }
  if (dismissedKey && dismissedKey === cameraPickKey(pick)) {
    return { accept: false, reason: "repeat" };
  }
  return { accept: true, reason: "ok" };
}

/**
 * Пользовательский путь на узком экране: кнопка влезает в ширину,
 * подпись читается, тап не меньше 44px, на телефоне открывается камера.
 * На широком превью — компактная иконка без capture (галерея / десктоп).
 */
export function planCameraButtonNarrowPath(
  viewportWidthPx: number,
  pick: CameraPickInput | null | undefined,
  dismissedKey?: string | null,
): CameraButtonNarrowPath {
  const gate = canAcceptCameraPick(pick, dismissedKey);
  const width =
    Number.isFinite(viewportWidthPx) && viewportWidthPx > 0 ? viewportWidthPx : NARROW_CAMERA_MAX_WIDTH_PX;
  const narrow = isNarrowCameraViewport(width);
  const gutterPx = NARROW_CAMERA_GUTTER_PX;
  const maxWidthPx = Math.max(0, Math.floor(width - gutterPx * 2));

  return {
    accepted: gate.accept,
    reason: gate.reason,
    narrow,
    stacked: narrow,
    gutterPx,
    maxWidthPx,
    tapPx: NARROW_CAMERA_MIN_TAP_PX,
    actionFullWidth: narrow,
    labelVisible: true,
    hintVisible: narrow,
    capture: narrow ? "environment" : undefined,
    overflowsHorizontally: false,
    label: NARROW_CAMERA_LABEL,
    hint: narrow ? NARROW_CAMERA_HINT : "",
  };
}

function clampLine(): CSSProperties {
  return {
    overflow: "hidden",
    textOverflow: "ellipsis",
    whiteSpace: "nowrap",
    overflowWrap: "anywhere",
    minWidth: 0,
  };
}

export function CameraButton({
  onPick,
  onReject,
  busy = false,
  disabled = false,
  dismissedKey = null,
  viewportWidth,
  accept = "image/*",
  fileName,
}: Props): ReactElement {
  const inputRef = useRef<HTMLInputElement>(null);
  const [liveWidth, setLiveWidth] = useState(() =>
    typeof window === "undefined"
      ? NARROW_CAMERA_MAX_WIDTH_PX
      : Math.floor(window.visualViewport?.width || window.innerWidth || NARROW_CAMERA_MAX_WIDTH_PX),
  );

  useEffect(() => {
    if (typeof viewportWidth === "number") return;
    const read = () => {
      setLiveWidth(
        Math.floor(window.visualViewport?.width || window.innerWidth || NARROW_CAMERA_MAX_WIDTH_PX),
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
    () =>
      planCameraButtonNarrowPath(
        width,
        fileName?.trim() ? { name: fileName.trim(), size: 1, type: "image/jpeg" } : null,
        dismissedKey,
      ),
    [width, fileName, dismissedKey],
  );

  const locked = busy || disabled;

  const openPicker = useCallback(() => {
    if (locked) return;
    inputRef.current?.click();
  }, [locked]);

  const onFileChange = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      const input = cameraPickInputFromFile(file);
      const gate = canAcceptCameraPick(input, dismissedKey);
      e.target.value = "";
      if (!gate.accept) {
        onReject?.(gate.reason);
        return;
      }
      if (file) onPick?.(file);
    },
    [dismissedKey, onPick, onReject],
  );

  const shell: CSSProperties = {
    boxSizing: "border-box",
    maxWidth: path.maxWidthPx,
    width: path.actionFullWidth ? "100%" : "auto",
    margin: 0,
    padding: 0,
    display: "flex",
    flexDirection: path.stacked ? "column" : "row",
    alignItems: path.stacked ? "stretch" : "center",
    gap: path.stacked ? 6 : 8,
    minWidth: 0,
  };

  const actionBtn: CSSProperties = {
    boxSizing: "border-box",
    display: "inline-flex",
    flexDirection: "row",
    alignItems: "center",
    justifyContent: path.stacked ? "center" : "flex-start",
    gap: 8,
    minWidth: path.tapPx,
    minHeight: path.tapPx,
    width: path.actionFullWidth ? "100%" : "auto",
    padding: path.actionFullWidth ? "10px 12px" : "8px 12px",
    border: 0,
    borderRadius: 10,
    background: locked ? "rgba(232, 163, 23, 0.45)" : "#e8a317",
    color: "#1c160f",
    fontWeight: 600,
    fontSize: path.stacked ? 15 : 14,
    lineHeight: 1.2,
    cursor: locked ? "default" : "pointer",
    opacity: locked ? 0.72 : 1,
  };

  const hintStyle: CSSProperties = {
    margin: 0,
    fontSize: 13,
    lineHeight: 1.3,
    color: "#6b5a48",
    ...clampLine(),
  };

  const shownName = (fileName ?? "").trim();

  return (
    <div
      className="camera-button"
      data-narrow={path.narrow ? "true" : "false"}
      data-stacked={path.stacked ? "true" : "false"}
      data-reason={path.reason}
      data-overflows={path.overflowsHorizontally ? "true" : "false"}
      data-capture={path.capture ?? "none"}
      style={shell}
    >
      <button
        type="button"
        className="camera-button__action"
        aria-label="Прикрепить фото"
        aria-busy={busy || undefined}
        disabled={locked}
        onClick={openPicker}
        style={actionBtn}
      >
        <span aria-hidden style={{ fontSize: 18, lineHeight: 1 }}>
          📷
        </span>
        {path.labelVisible ? (
          <span className="camera-button__label" style={clampLine()}>
            {busy ? "Загрузка…" : shownName || path.label}
          </span>
        ) : null}
      </button>
      {path.hintVisible && path.hint ? (
        <p className="camera-button__hint" style={hintStyle}>
          {path.hint}
        </p>
      ) : null}
      <input
        ref={inputRef}
        type="file"
        className="camera-button__input"
        accept={accept}
        {...(path.capture ? { capture: path.capture } : {})}
        disabled={locked}
        onChange={onFileChange}
        style={{ display: "none" }}
        aria-hidden
        tabIndex={-1}
      />
    </div>
  );
}
