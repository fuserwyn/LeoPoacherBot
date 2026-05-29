// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { ActivityCard } from "./ActivityCard";

afterEach(cleanup);

const baseProps = {
  avatar: "🦁",
  name: "Иван",
  timeAgo: "5 мин назад",
  emoji: "🏃",
  activity: "Бег",
  details: "15 мин, инт. 3/5",
};

describe("ActivityCard streak pill", () => {
  it("renders streak number with correctly declined aria-label", () => {
    render(<ActivityCard {...baseProps} streak={21} />);
    const pill = screen.getByLabelText("Стрик: 21 день подряд");
    expect(pill).toBeTruthy();
    expect(pill.textContent).toContain("21");
    expect(pill.textContent).toContain("Стрик");
  });

  it("uses 'дня' form for 3", () => {
    render(<ActivityCard {...baseProps} streak={3} />);
    expect(screen.getByLabelText("Стрик: 3 дня подряд")).toBeTruthy();
  });

  it("uses 'дней' form for 7", () => {
    render(<ActivityCard {...baseProps} streak={7} />);
    expect(screen.getByLabelText("Стрик: 7 дней подряд")).toBeTruthy();
  });

  it("hides the streak pill when hideStreak is set", () => {
    render(<ActivityCard {...baseProps} streak={10} hideStreak />);
    expect(screen.queryByLabelText(/^Стрик:/)).toBeNull();
  });
});
