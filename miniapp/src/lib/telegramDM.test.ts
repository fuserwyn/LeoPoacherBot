import { describe, expect, it } from "vitest";
import { feedTgUsername } from "./telegramDM";

describe("feedTgUsername", () => {
  it("извлекает ник из «@nick»", () => {
    expect(feedTgUsername("@ivan_fit")).toBe("ivan_fit");
    expect(feedTgUsername("  @Leo123  ")).toBe("Leo123");
  });

  it("отклоняет отображаемые имена и мусор", () => {
    expect(feedTgUsername("Иван Петров")).toBeUndefined();
    expect(feedTgUsername("@Иван")).toBeUndefined(); // кириллица — не TG-ник
    expect(feedTgUsername("@a b")).toBeUndefined();
    expect(feedTgUsername("@ab")).toBeUndefined(); // слишком короткий
    expect(feedTgUsername("")).toBeUndefined();
    expect(feedTgUsername(undefined)).toBeUndefined();
  });
});
