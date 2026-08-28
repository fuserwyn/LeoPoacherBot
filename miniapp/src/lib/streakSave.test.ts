import { describe, expect, it } from "vitest";
import {
  canUseStreakSave,
  streakSaveHint,
  streakSaveWindowError,
} from "./streakLabel";
import { effectiveStreakDays } from "./streakLabel";

/**
 * Контракт окна «Спасти стрик» — зеркало StreakSaveWindowError / UseStreakSaveAttemptForAPI.
 * daysSince === 2 → стрик в UI уже 0, можно закрыть один пропуск.
 */
describe("streak save window contract", () => {
  const cases: Array<{
    days: number;
    avail: number;
    error: ReturnType<typeof streakSaveWindowError>;
    can: boolean;
  }> = [
    { days: 2, avail: 1, error: null, can: true },
    { days: 2, avail: 3, error: null, can: true },
    { days: 0, avail: 1, error: "not_needed", can: false },
    { days: 1, avail: 1, error: "not_needed", can: false },
    { days: 3, avail: 1, error: "too_late", can: false },
    { days: 10, avail: 2, error: "too_late", can: false },
    { days: 2, avail: 0, error: "no_attempts", can: false },
    { days: 1, avail: 0, error: "no_attempts", can: false },
    { days: -1, avail: 1, error: "no_training_history", can: false },
    { days: -1, avail: 0, error: "no_attempts", can: false },
  ];

  it.each(cases)("days=$days avail=$avail → $error", ({ days, avail, error, can }) => {
    expect(streakSaveWindowError(days, avail)).toBe(error);
    expect(canUseStreakSave(days, avail)).toBe(can);
  });
});

describe("streakSaveHint", () => {
  it("all reasons have user-facing copy", () => {
    expect(streakSaveHint(2, 1)).toMatch(/восстановить сгоревший стрик/i);
    expect(streakSaveHint(1, 1)).toMatch(/ещё жив/i);
    expect(streakSaveHint(0, 1)).toMatch(/ещё жив/i);
    expect(streakSaveHint(3, 1)).toMatch(/больше одного дня/i);
    expect(streakSaveHint(2, 0)).toMatch(/попыток нет/i);
    expect(streakSaveHint(-1, 1)).toMatch(/не было тренировок/i);
  });
});

describe("Анечка-сценарий: стрик жив → кнопка disabled", () => {
  it("18.07, last training 17.07, attempts left", () => {
    const stored = 107;
    const daysSince = 1;
    const avail = 1;
    expect(effectiveStreakDays(stored, daysSince)).toBe(107);
    expect(canUseStreakSave(daysSince, avail)).toBe(false);
    expect(streakSaveWindowError(daysSince, avail)).toBe("not_needed");
    expect(streakSaveHint(daysSince, avail)).toMatch(/потренируйся/i);
  });

  it("утром после пропуска (days=2) — окно открыто", () => {
    const stored = 107;
    const daysSince = 2;
    expect(effectiveStreakDays(stored, daysSince)).toBe(0);
    expect(canUseStreakSave(daysSince, 1)).toBe(true);
    expect(streakSaveHint(daysSince, 1)).toMatch(/восстановить сгоревший стрик/i);
  });
});
