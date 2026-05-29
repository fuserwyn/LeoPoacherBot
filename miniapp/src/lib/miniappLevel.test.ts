import { describe, expect, it } from "vitest";
import { miniappCupsLevelProgress, miniappLevelFromCups } from "./miniappLevel";

describe("miniappLevelFromCups", () => {
  it("maps total cups to level intervals", () => {
    expect(miniappLevelFromCups(0)).toBe(1);
    expect(miniappLevelFromCups(419)).toBe(1);
    expect(miniappLevelFromCups(420)).toBe(2);
    expect(miniappLevelFromCups(425)).toBe(2);
    expect(miniappLevelFromCups(13019)).toBe(5);
    expect(miniappLevelFromCups(13020)).toBe(6);
    expect(miniappLevelFromCups(50000)).toBe(6);
  });
});

describe("miniappCupsLevelProgress", () => {
  it("L1: progress from zero", () => {
    expect(miniappCupsLevelProgress(200)).toEqual({
      level: 1,
      totalCups: 200,
      cupsInSegment: 200,
      cupsToNext: 420,
      nextLevelThreshold: 420,
    });
  });

  it("L2: shows total cups toward next threshold", () => {
    expect(miniappLevelFromCups(800)).toBe(2);
    expect(miniappCupsLevelProgress(800)).toEqual({
      level: 2,
      totalCups: 800,
      cupsInSegment: 380,
      cupsToNext: 840,
      nextLevelThreshold: 1260,
    });
  });

  it("L2 at 425 cups (regression from profile screenshot)", () => {
    expect(miniappCupsLevelProgress(425)).toEqual({
      level: 2,
      totalCups: 425,
      cupsInSegment: 5,
      cupsToNext: 840,
      nextLevelThreshold: 1260,
    });
  });

  it("L3: segment size 1680", () => {
    expect(miniappCupsLevelProgress(2000)).toEqual({
      level: 3,
      totalCups: 2000,
      cupsInSegment: 740,
      cupsToNext: 1680,
      nextLevelThreshold: 2940,
    });
  });

  it("L5: segment size 6720", () => {
    expect(miniappCupsLevelProgress(10000)).toEqual({
      level: 5,
      totalCups: 10000,
      cupsInSegment: 3700,
      cupsToNext: 6720,
      nextLevelThreshold: 13020,
    });
  });

  it("L6: endgame segment after max level threshold", () => {
    expect(miniappLevelFromCups(15000)).toBe(6);
    expect(miniappCupsLevelProgress(15000)).toEqual({
      level: 6,
      totalCups: 15000,
      cupsInSegment: 1980,
      cupsToNext: 13440,
      nextLevelThreshold: null,
    });
  });

  it("L6+: wraps after full endgame cycle", () => {
    expect(miniappCupsLevelProgress(13020 + 13440)).toEqual({
      level: 6,
      totalCups: 26460,
      cupsInSegment: 0,
      cupsToNext: 13440,
      nextLevelThreshold: null,
    });
  });
});
