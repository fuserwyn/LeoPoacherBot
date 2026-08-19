// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { RulesScreen } from "./RulesScreen";

afterEach(cleanup);

describe("RulesScreen", () => {
  it("starts with the accordion list and has no top header", () => {
    const { container } = render(<RulesScreen />);
    expect(container.querySelector(".rules__accordion-list")).toBeTruthy();
    expect(screen.queryByText("Правила Стаи")).toBeNull();
    expect(container.querySelector("h1")).toBeNull();
    expect(container.firstElementChild?.firstElementChild?.className).toBe("rules__accordion-list");
  });

  it("uses a decorative CSS chevron instead of plus/minus", () => {
    const { container } = render(<RulesScreen />);
    const triggers = container.querySelectorAll(".rules__accordion-trigger");
    expect(triggers).toHaveLength(11);
    for (const trigger of triggers) {
      expect(trigger.getAttribute("aria-expanded")).toBe("false");
      expect(trigger.textContent).not.toMatch(/[+\u2212−]/);
      const chevron = trigger.querySelector(".rules__accordion-chevron");
      expect(chevron).toBeTruthy();
      expect(chevron?.getAttribute("aria-hidden")).toBe("true");
    }
  });

  it("opens one section at a time and toggles closed on a second tap", () => {
    render(<RulesScreen />);
    const first = screen.getByRole("button", { name: /1 — Зачем это приложение/i });
    const second = screen.getByRole("button", { name: /2 — Принципы/i });

    fireEvent.click(first);
    expect(first.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByText(/Стая — это место, где спорт наконец встраивается/)).toBeTruthy();

    fireEvent.click(second);
    expect(first.getAttribute("aria-expanded")).toBe("false");
    expect(second.getAttribute("aria-expanded")).toBe("true");
    expect(screen.queryByText(/Стая — это место, где спорт наконец встраивается/)).toBeNull();
    expect(screen.getByText(/Любое движение считается/)).toBeTruthy();

    fireEvent.click(second);
    expect(second.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByText(/Любое движение считается/)).toBeNull();
  });

  it("keeps all 11 sections", () => {
    render(<RulesScreen />);
    expect(screen.getByRole("button", { name: /1 — Зачем это приложение/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /2 — Принципы/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /3 — Кто такой Лео/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /4 — Что такое Стая/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /5 — Как ты движешься/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /6 — Как сохранить стрик/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /7 — Что видит Стая/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /8 — Если ты пропустил/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /9 — Если ты удалён и как вернуться/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /10 — Больничный/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: /11 — FAQ/i })).toBeTruthy();
  });
});
