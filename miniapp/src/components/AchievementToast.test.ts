import { describe, expect, it } from "vitest";
import {
  NARROW_TOAST_GUTTER_PX,
  NARROW_TOAST_MIN_TAP_PX,
  canShowAchievementToast,
  isNarrowAchievementToastViewport,
  leoCheer,
  planAchievementToastNarrowPath,
  type AchievementToastItem,
} from "./AchievementToast";

const week: AchievementToastItem = {
  id: "streak-7",
  title: "Неделя в строю",
  streakDays: 7,
  cups: 42,
};

describe("leoCheer", () => {
  it("happy path: 7-day streak keeps Leo voice", () => {
    const line = leoCheer(week.title, 7);
    expect(line).toContain("Лео");
    expect(line.length).toBeGreaterThan(10);
  });

  it("empty / whitespace title is a silent reject", () => {
    expect(leoCheer("")).toBe("");
    expect(leoCheer("   ")).toBe("");
    expect(leoCheer("\n\t")).toBe("");
  });

  it("old default path still works without streakDays", () => {
    expect(leoCheer("Первый кубок")).toBe("Лео рядом: Первый кубок");
  });
});

describe("canShowAchievementToast", () => {
  it("shows a complete payload", () => {
    expect(canShowAchievementToast(week)).toEqual({ show: true, reason: "ok" });
  });

  it("rejects empty / missing payload", () => {
    expect(canShowAchievementToast(null).reason).toBe("empty");
    expect(canShowAchievementToast(undefined).reason).toBe("empty");
    expect(canShowAchievementToast({ id: "", title: "x" }).reason).toBe("empty");
    expect(canShowAchievementToast({ id: "a", title: "  " }).reason).toBe("empty");
  });

  it("repeat of a dismissed id stays hidden", () => {
    expect(canShowAchievementToast(week, "streak-7")).toEqual({ show: false, reason: "repeat" });
    expect(canShowAchievementToast(week, "other").show).toBe(true);
  });
});

describe("пользовательский путь на узком экране", () => {
  it("320px: stacked, in-bounds, 44px tap, no horizontal overflow", () => {
    const path = planAchievementToastNarrowPath(320, week);
    expect(path.visible).toBe(true);
    expect(path.reason).toBe("ok");
    expect(path.narrow).toBe(true);
    expect(path.stacked).toBe(true);
    expect(path.actionFullWidth).toBe(true);
    expect(path.dismissTapPx).toBeGreaterThanOrEqual(NARROW_TOAST_MIN_TAP_PX);
    expect(path.maxWidthPx).toBe(320 - NARROW_TOAST_GUTTER_PX * 2);
    expect(path.maxWidthPx).toBeLessThanOrEqual(320);
    expect(path.overflowsHorizontally).toBe(false);
    expect(path.cheer).toBe(leoCheer(week.title, 7));
    expect(isNarrowAchievementToastViewport(320)).toBe(true);
  });

  it("wide preview keeps the compact row (old scenario)", () => {
    const path = planAchievementToastNarrowPath(768, week);
    expect(path.visible).toBe(true);
    expect(path.narrow).toBe(false);
    expect(path.stacked).toBe(false);
    expect(path.actionFullWidth).toBe(false);
    expect(path.overflowsHorizontally).toBe(false);
  });

  it("empty input on a phone does not open a toast", () => {
    const path = planAchievementToastNarrowPath(320, { id: "x", title: "" });
    expect(path.visible).toBe(false);
    expect(path.reason).toBe("empty");
    expect(path.cheer).toBe("");
  });

  it("повтор after dismiss stays closed on a narrow screen", () => {
    const path = planAchievementToastNarrowPath(360, week, week.id);
    expect(path.visible).toBe(false);
    expect(path.reason).toBe("repeat");
  });
});
