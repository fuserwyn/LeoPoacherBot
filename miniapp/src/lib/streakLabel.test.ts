import { describe, expect, it } from "vitest";
import {
  daysWordRu,
  streakStreakAriaLabel,
  cupsWordRu,
  streakBurnRemainingMs,
  formatStreakBurnRemaining,
  streakBurnLabel,
} from "./streakLabel";

describe("daysWordRu", () => {
  it("singular for 1, 21, 101", () => {
    expect(daysWordRu(1)).toBe("день");
    expect(daysWordRu(21)).toBe("день");
    expect(daysWordRu(101)).toBe("день");
  });

  it("few (2-4) form", () => {
    expect(daysWordRu(2)).toBe("дня");
    expect(daysWordRu(3)).toBe("дня");
    expect(daysWordRu(4)).toBe("дня");
    expect(daysWordRu(22)).toBe("дня");
  });

  it("many form for 5-20, 0", () => {
    expect(daysWordRu(0)).toBe("дней");
    expect(daysWordRu(5)).toBe("дней");
    expect(daysWordRu(10)).toBe("дней");
    expect(daysWordRu(25)).toBe("дней");
    expect(daysWordRu(100)).toBe("дней");
  });

  it("special case 11-14 always many", () => {
    expect(daysWordRu(11)).toBe("дней");
    expect(daysWordRu(12)).toBe("дней");
    expect(daysWordRu(13)).toBe("дней");
    expect(daysWordRu(14)).toBe("дней");
    expect(daysWordRu(111)).toBe("дней");
    expect(daysWordRu(112)).toBe("дней");
  });
});

describe("cupsWordRu", () => {
  it("declensions", () => {
    expect(cupsWordRu(1)).toBe("кубок");
    expect(cupsWordRu(2)).toBe("кубка");
    expect(cupsWordRu(5)).toBe("кубков");
    expect(cupsWordRu(11)).toBe("кубков");
    expect(cupsWordRu(21)).toBe("кубок");
    expect(cupsWordRu(42)).toBe("кубка");
    expect(cupsWordRu(420)).toBe("кубков");
  });
});

describe("streakStreakAriaLabel", () => {
  it("empty state for 0 and negatives", () => {
    expect(streakStreakAriaLabel(0)).toBe("Стрик: пока нет дней подряд с тренировками");
    expect(streakStreakAriaLabel(-3)).toBe("Стрик: пока нет дней подряд с тренировками");
  });

  it("builds label with correct declension", () => {
    expect(streakStreakAriaLabel(1)).toBe("Стрик: 1 день подряд");
    expect(streakStreakAriaLabel(3)).toBe("Стрик: 3 дня подряд");
    expect(streakStreakAriaLabel(7)).toBe("Стрик: 7 дней подряд");
    expect(streakStreakAriaLabel(11)).toBe("Стрик: 11 дней подряд");
    expect(streakStreakAriaLabel(21)).toBe("Стрик: 21 день подряд");
  });
});

describe("streakBurnRemainingMs", () => {
  // 29 мая 2026, 18:00 локального времени.
  const now = new Date(2026, 4, 29, 18, 0, 0);
  const H = 3_600_000;

  it("нет стрика → null", () => {
    expect(streakBurnRemainingMs(0, 1, now)).toBeNull();
    expect(streakBurnRemainingMs(-1, 0, now)).toBeNull();
  });

  it("дата последней тренировки неизвестна (−1) → null", () => {
    expect(streakBurnRemainingMs(5, -1, now)).toBeNull();
  });

  it("стрик уже должен сгореть (≥2 дня без тренировки) → null", () => {
    expect(streakBurnRemainingMs(5, 2, now)).toBeNull();
    expect(streakBurnRemainingMs(5, 7, now)).toBeNull();
  });

  it("тренировался вчера → горит в конце сегодня (через 6 ч от 18:00)", () => {
    expect(streakBurnRemainingMs(5, 1, now)).toBe(6 * H);
  });

  it("тренировался сегодня → горит в конце завтра (через 30 ч от 18:00)", () => {
    expect(streakBurnRemainingMs(5, 0, now)).toBe(30 * H);
  });
});

describe("formatStreakBurnRemaining", () => {
  const H = 3_600_000;
  const M = 60_000;

  it("только минуты", () => {
    expect(formatStreakBurnRemaining(18 * M)).toBe("18 мин");
  });

  it("часы и минуты", () => {
    expect(formatStreakBurnRemaining(5 * H + 20 * M)).toBe("5 ч 20 мин");
  });

  it("дни и часы с правильным склонением", () => {
    expect(formatStreakBurnRemaining(24 * H + 4 * H)).toBe("1 день 4 ч");
    expect(formatStreakBurnRemaining(2 * 24 * H + 3 * H)).toBe("2 дня 3 ч");
  });

  it("отрицательное/ноль → 0 мин", () => {
    expect(formatStreakBurnRemaining(-5000)).toBe("0 мин");
  });
});

describe("streakBurnLabel", () => {
  const now = new Date(2026, 4, 29, 18, 0, 0);

  it("собирает подпись для живого стрика", () => {
    expect(streakBurnLabel(5, 1, now)).toBe("сгорит через 6 ч 0 мин");
  });

  it("null, когда стрик не горит", () => {
    expect(streakBurnLabel(0, 1, now)).toBeNull();
    expect(streakBurnLabel(5, 2, now)).toBeNull();
  });
});
