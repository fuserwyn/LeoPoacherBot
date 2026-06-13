import { describe, expect, it } from "vitest";
import { achievementLabel, earnedAchievementKeys } from "./achievements";

describe("earnedAchievementKeys", () => {
  it("без прогресса — пусто", () => {
    expect(earnedAchievementKeys(0, 0)).toEqual([]);
  });

  it("стрик-ачивки берёт первые N порогов по achievementCount", () => {
    expect(earnedAchievementKeys(2, 0)).toEqual(["streak-7", "streak-14"]);
  });

  it("ачивки за тренировки открываются по достижению порога", () => {
    expect(earnedAchievementKeys(0, 10)).toEqual(["workout-10"]);
    expect(earnedAchievementKeys(0, 42)).toEqual(["workout-10", "workout-20", "workout-42"]);
  });

  it("комбинирует стрик- и тренировочные ачивки", () => {
    expect(earnedAchievementKeys(1, 20)).toEqual(["streak-7", "workout-10", "workout-20"]);
  });

  it("achievementCount сверх числа порогов не ломает результат", () => {
    expect(earnedAchievementKeys(99, 0)).toHaveLength(9);
  });
});

describe("achievementLabel", () => {
  it("подписи со склонением", () => {
    expect(achievementLabel("streak-7")).toBe("Стрик 7 дней");
    expect(achievementLabel("streak-42")).toBe("Стрик 42 дня");
    expect(achievementLabel("workout-10")).toBe("10 тренировок");
    expect(achievementLabel("workout-42")).toBe("42 тренировки");
  });
});
