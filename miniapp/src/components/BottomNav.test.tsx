// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { BottomNav } from "./BottomNav";

class ROStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(globalThis as unknown as { ResizeObserver: typeof ROStub }).ResizeObserver = ROStub;

afterEach(cleanup);

const handlers = {
  onChat: () => {},
  onFeed: () => {},
  onAddWorkout: () => {},
  onRules: () => {},
  onProfile: () => {},
};

describe("BottomNav compose collapse", () => {
  it("keeps the compose row mounted and collapsed when hidden", () => {
    const { container, rerender } = render(<BottomNav active="feed" showCompose {...handlers} />);
    expect(container.querySelector(".bottom-nav__compose-row--collapsed")).toBeNull();
    expect(container.querySelector(".bottom-nav__compose-input")).toBeTruthy();

    rerender(<BottomNav active="chat" showCompose={false} {...handlers} />);
    const row = container.querySelector(".bottom-nav__compose-row--collapsed");
    expect(row).toBeTruthy();
    expect(container.querySelector(".bottom-nav__compose-input")).toBeTruthy();
    expect(row?.getAttribute("aria-hidden")).toBe("true");
  });
});
