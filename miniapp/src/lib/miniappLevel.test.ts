import { describe, expect, it } from "vitest";
import { miniappCupsLevelProgress, miniappLevelFromCups } from "./miniappLevel";

describe("miniappCupsLevelProgress", () => {
  it("L1: progress from zero", () => {
    expect(miniappCupsLevelProgress(200)).toEqual({
      level: 1,
      cupsInSegment: 200,
      cupsToNext: 420,
    });
  });

  it("L2: segment excludes cups from previous levels", () => {
    expect(miniappLevelFromCups(800)).toBe(2);
    expect(miniappCupsLevelProgress(800)).toEqual({
      level: 2,
      cupsInSegment: 380,
      cupsToNext: 840,
    });
  });

  it("L3: segment size 1680", () => {
    expect(miniappCupsLevelProgress(2000)).toEqual({
      level: 3,
      cupsInSegment: 740,
      cupsToNext: 1680,
    });
  });

  it("L6: segment size 13440", () => {
    expect(miniappCupsLevelProgress(20000)).toEqual({
      level: 6,
      cupsInSegment: 6980,
      cupsToNext: 13440,
    });
  });

  it("L7+: cyclic endgame segment", () => {
    expect(miniappLevelFromCups(28873)).toBe(7);
    expect(miniappCupsLevelProgress(28873)).toEqual({
      level: 7,
      cupsInSegment: 2413,
      cupsToNext: 13440,
    });
  });

  it("L7+: wraps after full endgame cycle", () => {
    // 26460 + 13440 = 39900 → second endgame cycle, 0 in segment
    expect(miniappCupsLevelProgress(39900)).toEqual({
      level: 7,
      cupsInSegment: 0,
      cupsToNext: 13440,
    });
  });
});
