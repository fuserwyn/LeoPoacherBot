// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { applyTheme, getStoredTheme, setTheme } from "./theme";

describe("theme", () => {
  afterEach(() => {
    localStorage.removeItem("leo-theme");
    document.documentElement.removeAttribute("data-theme");
  });

  it("defaults to dark when nothing stored", () => {
    expect(getStoredTheme()).toBe("dark");
  });

  it("setTheme persists and applyTheme sets data-theme", () => {
    setTheme("light");
    expect(localStorage.getItem("leo-theme")).toBe("light");
    expect(getStoredTheme()).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    applyTheme("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("stores leopard theme", () => {
    setTheme("leopard");
    expect(localStorage.getItem("leo-theme")).toBe("leopard");
    expect(getStoredTheme()).toBe("leopard");
    expect(document.documentElement.getAttribute("data-theme")).toBe("leopard");
  });
});
