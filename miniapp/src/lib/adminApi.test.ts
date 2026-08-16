import { describe, expect, it } from "vitest";
import { adminErrorLabel } from "./adminApi";

describe("adminErrorLabel", () => {
  it("maps known codes", () => {
    expect(adminErrorLabel("forbidden")).toBe("Нет прав администратора");
    expect(adminErrorLabel("not_found")).toBe("Не найдено или уже обработано");
    expect(adminErrorLabel("empty_text")).toBe("Напиши текст");
    expect(adminErrorLabel("invalid_price")).toBe("Цена должна быть от 1 до 100000 ₽");
  });

  it("falls back to the raw code", () => {
    expect(adminErrorLabel("weird")).toBe("weird");
    expect(adminErrorLabel()).toBe("");
  });
});
