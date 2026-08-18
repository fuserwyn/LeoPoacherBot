import { describe, expect, it } from "vitest";
import {
  NARROW_CAMERA_GUTTER_PX,
  NARROW_CAMERA_HINT,
  NARROW_CAMERA_LABEL,
  NARROW_CAMERA_MIN_TAP_PX,
  canAcceptCameraPick,
  cameraPickKey,
  isCameraImagePick,
  isNarrowCameraViewport,
  planCameraButtonNarrowPath,
  type CameraPickInput,
} from "./camerabutton";

const shot: CameraPickInput = {
  name: "workout.jpg",
  size: 48_000,
  type: "image/jpeg",
  lastModified: 1_700_000_000_000,
};

describe("isCameraImagePick", () => {
  it("happy path: jpeg from camera is an image", () => {
    expect(isCameraImagePick(shot)).toBe(true);
    expect(isCameraImagePick({ name: "clip.heic", size: 10, type: "" })).toBe(true);
  });

  it("empty / whitespace / non-image is a silent reject", () => {
    expect(isCameraImagePick({ name: "", size: 1, type: "image/jpeg" })).toBe(false);
    expect(isCameraImagePick({ name: "note.txt", size: 12, type: "text/plain" })).toBe(false);
    expect(isCameraImagePick({ name: "   ", size: 12, type: "" })).toBe(false);
  });

  it("old gallery path still works without a mime type", () => {
    expect(isCameraImagePick({ name: "зал.png", size: 99, type: "" })).toBe(true);
  });
});

describe("canAcceptCameraPick", () => {
  it("accepts a complete image payload", () => {
    expect(canAcceptCameraPick(shot)).toEqual({ accept: true, reason: "ok" });
  });

  it("rejects empty / missing payload", () => {
    expect(canAcceptCameraPick(null).reason).toBe("empty");
    expect(canAcceptCameraPick(undefined).reason).toBe("empty");
    expect(canAcceptCameraPick({ name: "", size: 10, type: "image/jpeg" }).reason).toBe("empty");
    expect(canAcceptCameraPick({ name: "x.jpg", size: 0, type: "image/jpeg" }).reason).toBe("empty");
    expect(canAcceptCameraPick({ name: "  ", size: 10, type: "image/jpeg" }).reason).toBe("empty");
  });

  it("repeat of a dismissed pick stays idle", () => {
    expect(canAcceptCameraPick(shot, cameraPickKey(shot))).toEqual({ accept: false, reason: "repeat" });
    expect(canAcceptCameraPick(shot, "other").accept).toBe(true);
  });
});

describe("пользовательский путь на узком экране", () => {
  it("320px: stacked, in-bounds, 44px tap, camera capture, no horizontal overflow", () => {
    const path = planCameraButtonNarrowPath(320, shot);
    expect(path.accepted).toBe(true);
    expect(path.reason).toBe("ok");
    expect(path.narrow).toBe(true);
    expect(path.stacked).toBe(true);
    expect(path.actionFullWidth).toBe(true);
    expect(path.labelVisible).toBe(true);
    expect(path.hintVisible).toBe(true);
    expect(path.hint).toBe(NARROW_CAMERA_HINT);
    expect(path.label).toBe(NARROW_CAMERA_LABEL);
    expect(path.tapPx).toBeGreaterThanOrEqual(NARROW_CAMERA_MIN_TAP_PX);
    expect(path.maxWidthPx).toBe(320 - NARROW_CAMERA_GUTTER_PX * 2);
    expect(path.maxWidthPx).toBeLessThanOrEqual(320);
    expect(path.overflowsHorizontally).toBe(false);
    expect(path.capture).toBe("environment");
    expect(isNarrowCameraViewport(320)).toBe(true);
  });

  it("wide preview keeps the compact control (old scenario)", () => {
    const path = planCameraButtonNarrowPath(768, shot);
    expect(path.accepted).toBe(true);
    expect(path.narrow).toBe(false);
    expect(path.stacked).toBe(false);
    expect(path.actionFullWidth).toBe(false);
    expect(path.hintVisible).toBe(false);
    expect(path.capture).toBeUndefined();
    expect(path.overflowsHorizontally).toBe(false);
    expect(path.tapPx).toBeGreaterThanOrEqual(NARROW_CAMERA_MIN_TAP_PX);
  });

  it("empty input on a phone does not accept a shot", () => {
    const path = planCameraButtonNarrowPath(320, { name: "", size: 0, type: "" });
    expect(path.accepted).toBe(false);
    expect(path.reason).toBe("empty");
    expect(path.narrow).toBe(true);
    expect(path.stacked).toBe(true);
  });

  it("повтор after dismiss stays idle on a narrow screen", () => {
    const path = planCameraButtonNarrowPath(360, shot, cameraPickKey(shot));
    expect(path.accepted).toBe(false);
    expect(path.reason).toBe("repeat");
    expect(path.narrow).toBe(true);
  });
});
