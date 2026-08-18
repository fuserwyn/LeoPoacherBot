// @vitest-environment jsdom
import { describe, expect, it, afterEach } from "vitest";
import { applyScrollY, captureScrollY, feedFilterEpoch } from "./tabScrollRestore";

function stubScroll(initial = 0) {
  let y = initial;
  const scrollYDesc = { configurable: true, get: () => y };
  Object.defineProperty(window, "scrollY", scrollYDesc);
  Object.defineProperty(document.documentElement, "scrollTop", {
    configurable: true,
    get: () => y,
    set: (v: number) => {
      y = Number(v) || 0;
    },
  });
  Object.defineProperty(document.body, "scrollTop", {
    configurable: true,
    get: () => y,
    set: (v: number) => {
      y = Number(v) || 0;
    },
  });
  window.scrollTo = ((...args: unknown[]) => {
    if (typeof args[0] === "number") {
      y = Number(args[1]) || 0;
      return;
    }
    const opts = args[0] as { top?: number } | undefined;
    if (opts && typeof opts.top === "number") y = opts.top;
  }) as typeof window.scrollTo;
  return {
    get: () => y,
    set: (n: number) => {
      y = n;
    },
  };
}

afterEach(() => {
  stubScroll(0);
});

describe("tabScrollRestore", () => {
  it("captureScrollY reads window.scrollY", () => {
    const scroll = stubScroll(420);
    expect(captureScrollY()).toBe(420);
    scroll.set(12);
    expect(captureScrollY()).toBe(12);
  });

  it("applyScrollY writes a non-negative position", () => {
    const scroll = stubScroll(800);
    applyScrollY(140);
    expect(scroll.get()).toBe(140);
    applyScrollY(-20);
    expect(scroll.get()).toBe(0);
  });
});

describe("feedFilterEpoch", () => {
  it("changes when scope, type or categories change", () => {
    const a = feedFilterEpoch("all", "all", []);
    const b = feedFilterEpoch("mine", "all", []);
    const c = feedFilterEpoch("all", "message", []);
    const d = feedFilterEpoch("all", "all", ["run"]);
    const e = feedFilterEpoch("all", "all", ["run", "bike"]);
    expect(new Set([a, b, c, d, e]).size).toBe(5);
  });

  it("is stable for the same filter tuple", () => {
    expect(feedFilterEpoch("friends", "training", ["yoga"])).toBe(
      feedFilterEpoch("friends", "training", ["yoga"]),
    );
  });
});
