import { describe, expect, it } from "vitest";
import { isModerationError, moderationUserMessage } from "./moderationMessages";

describe("moderationUserMessage", () => {
  it("maps known codes", () => {
    expect(moderationUserMessage("moderation_profanity")).toMatch(/недопустимые слова/i);
    expect(moderationUserMessage("moderation_link")).toMatch(/ссылки/i);
    expect(moderationUserMessage("user_muted")).toMatch(/ограничена/i);
    expect(moderationUserMessage("leo_daily_limited")).toMatch(/20 сообщений/i);
  });

  it("fallback / default", () => {
    expect(moderationUserMessage("unknown", " Свой текст ")).toBe("Свой текст");
    expect(moderationUserMessage(undefined)).toMatch(/не прошёл проверку/i);
  });
});

describe("isModerationError", () => {
  it("detects moderation family", () => {
    expect(isModerationError("moderation_blocked")).toBe(true);
    expect(isModerationError("user_muted")).toBe(true);
    expect(isModerationError("leo_daily_limited")).toBe(true);
    expect(isModerationError("save_failed")).toBe(false);
    expect(isModerationError(undefined)).toBe(false);
  });
});
