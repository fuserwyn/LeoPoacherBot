// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { AchievementToast, leoCheer } from "./AchievementToast";

afterEach(cleanup);

describe("AchievementToast", () => {
  it("празднует первую тренировку отдельной фразой", () => {
    const { getByText } = render(<AchievementToast achievementKey="workout-1" onDone={() => {}} />);
    expect(getByText("Ачивка получена!")).toBeTruthy();
    expect(getByText("1 тренировка")).toBeTruthy();
    expect(getByText(/Первая тренировка/)).toBeTruthy();
  });

  it("остальным порогам оставляет прежние фразы", () => {
    expect(leoCheer("workout-10")).toBe("Ты просто зверь! Лео даёт лапу 🐾");
    expect(leoCheer("streak-7")).toBe("Так держать — Лео тобой гордится!");
  });
});
