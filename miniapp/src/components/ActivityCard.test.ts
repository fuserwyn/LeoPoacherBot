import { describe, expect, it } from "vitest";
import {
  activityCardKey,
  canShowActivityCard,
  isNarrowFeedViewport,
  NARROW_FEED_GUTTER_PX,
  NARROW_FEED_MIN_TAP_PX,
  planActivityCardNarrowPath,
  type ActivityCardSnapshot,
} from "../lib/packFeed";

const workout: ActivityCardSnapshot = {
  name: "Анна",
  activity: "Велосипед",
  comment: "Доехала до фонтана и обратно, ноги горят",
};

describe("canShowActivityCard", () => {
  it("shows a complete workout card", () => {
    expect(canShowActivityCard(workout)).toEqual({ show: true, reason: "ok" });
  });

  it("rejects empty / missing payload", () => {
    expect(canShowActivityCard(null).reason).toBe("empty");
    expect(canShowActivityCard(undefined).reason).toBe("empty");
    expect(canShowActivityCard({ name: "", activity: "", comment: "" }).reason).toBe("empty");
    expect(canShowActivityCard({ name: "   ", activity: "  ", comment: "\n" }).reason).toBe("empty");
  });

  it("repeat of a dismissed card stays hidden", () => {
    expect(canShowActivityCard(workout, activityCardKey(workout))).toEqual({ show: false, reason: "repeat" });
    expect(canShowActivityCard(workout, "other").show).toBe(true);
  });
});

describe("пользовательский путь на узком экране", () => {
  it("320px: stacked, in-bounds, 44px tap, no horizontal overflow", () => {
    const path = planActivityCardNarrowPath(320, workout);
    expect(path.visible).toBe(true);
    expect(path.reason).toBe("ok");
    expect(path.narrow).toBe(true);
    expect(path.stacked).toBe(true);
    expect(path.actionFullWidth).toBe(true);
    expect(path.filtersScrollX).toBe(true);
    expect(path.reactionsScrollX).toBe(true);
    expect(path.tapPx).toBeGreaterThanOrEqual(NARROW_FEED_MIN_TAP_PX);
    expect(path.maxWidthPx).toBe(320 - NARROW_FEED_GUTTER_PX * 2);
    expect(path.maxWidthPx).toBeLessThanOrEqual(320);
    expect(path.overflowsHorizontally).toBe(false);
    expect(path.swipeAxis).toBe("y");
    expect(path.commentExpandable).toBe(true);
    expect(isNarrowFeedViewport(320)).toBe(true);
  });

  it("wide preview keeps the compact row (old scenario)", () => {
    const path = planActivityCardNarrowPath(768, workout);
    expect(path.visible).toBe(true);
    expect(path.narrow).toBe(false);
    expect(path.stacked).toBe(false);
    expect(path.actionFullWidth).toBe(false);
    expect(path.filtersScrollX).toBe(false);
    expect(path.overflowsHorizontally).toBe(false);
    expect(path.swipeAxis).toBe("both");
    expect(path.tapPx).toBeGreaterThanOrEqual(NARROW_FEED_MIN_TAP_PX);
  });

  it("empty input on a phone does not open a card", () => {
    const path = planActivityCardNarrowPath(320, { name: "", activity: "", comment: "" });
    expect(path.visible).toBe(false);
    expect(path.reason).toBe("empty");
    expect(path.narrow).toBe(true);
    expect(path.stacked).toBe(true);
  });

  it("повтор after dismiss stays closed on a narrow screen", () => {
    const path = planActivityCardNarrowPath(360, workout, activityCardKey(workout));
    expect(path.visible).toBe(false);
    expect(path.reason).toBe("repeat");
    expect(path.narrow).toBe(true);
  });
});
