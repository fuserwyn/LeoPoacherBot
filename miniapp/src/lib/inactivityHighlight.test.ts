import { describe, expect, it } from "vitest";
import { inactivityHighlight } from "./inactivityHighlight";

describe("inactivityHighlight", () => {
  it("none below 5 days", () => {
    expect(inactivityHighlight(0)).toBe("none");
    expect(inactivityHighlight(4)).toBe("none");
  });

  it("yellow on day 5", () => {
    expect(inactivityHighlight(5)).toBe("yellow");
  });

  it("red on day 6", () => {
    expect(inactivityHighlight(6)).toBe("red");
  });

  it("bordeaux on day 7+", () => {
    expect(inactivityHighlight(7)).toBe("bordeaux");
    expect(inactivityHighlight(10)).toBe("bordeaux");
  });
});
