import { describe, expect, it } from "vitest";
import { formatDurationToDays, inactiveDaysFromRemovalRemaining, removalRemainingUntil } from "./inactivityRemoval";

describe("formatDurationToDays", () => {
  it("formats days and hours", () => {
    expect(formatDurationToDays(6 * 24 * 60 * 60_000 + 22 * 60 * 60_000)).toBe("6 дней 22 ч.");
  });

  it("formats hours and minutes", () => {
    expect(formatDurationToDays(22 * 60 * 60_000)).toBe("22 ч.");
    expect(formatDurationToDays(90 * 60_000)).toBe("1 ч. 30 мин.");
  });
});

describe("removalRemainingUntil", () => {
  it("returns null for past deadline", () => {
    expect(removalRemainingUntil("2020-01-01T00:00:00+03:00", new Date("2025-01-01"))).toBeNull();
  });

  it("computes remaining from RFC3339", () => {
    const now = new Date("2026-07-01T01:12:00+03:00");
    const at = "2026-07-08T00:00:00+03:00";
    const r = removalRemainingUntil(at, now);
    expect(r).not.toBeNull();
    expect(r!.text).toBe("6 дней 22 ч.");
  });
});

describe("inactiveDaysFromRemovalRemaining", () => {
  it("maps 72/48/24h milestones to days 5/6/7", () => {
    expect(inactiveDaysFromRemovalRemaining(80 * 3_600_000)).toBeNull();
    expect(inactiveDaysFromRemovalRemaining(60 * 3_600_000)).toBe(5);
    expect(inactiveDaysFromRemovalRemaining(36 * 3_600_000)).toBe(6);
    expect(inactiveDaysFromRemovalRemaining(12 * 3_600_000)).toBe(7);
  });
});
