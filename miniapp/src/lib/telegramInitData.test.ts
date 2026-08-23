// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { resolveInitData, INIT_DATA_STORAGE_KEY } from "./telegramInitData";

// Подписанная строка, какую Telegram кладёт в WebApp.initData (user уже %-кодирован внутри).
const RAW =
  "query_id=AAH123&user=%7B%22id%22%3A777%2C%22first_name%22%3A%22Leo%22%7D&auth_date=1700000000&hash=abc123";

function setHash(rawInitData: string) {
  // Так Telegram передаёт запуск: во фрагменте tgWebAppData ещё раз URL-кодирован.
  const frag = `tgWebAppData=${encodeURIComponent(rawInitData)}&tgWebAppVersion=7.0&tgWebAppPlatform=android`;
  window.location.hash = `#${frag}`;
}

beforeEach(() => {
  window.location.hash = "";
  window.sessionStorage.clear();
  delete (window as { Telegram?: unknown }).Telegram;
});

afterEach(() => {
  window.location.hash = "";
  window.sessionStorage.clear();
  delete (window as { Telegram?: unknown }).Telegram;
});

describe("resolveInitData", () => {
  it("берёт initData напрямую из WebApp, когда он заполнен", () => {
    (window as unknown as { Telegram: { WebApp: { initData: string } } }).Telegram = {
      WebApp: { initData: RAW },
    };
    expect(resolveInitData()).toBe(RAW);
  });

  it("БАГ: WebApp.initData пуст, но запуск пришёл в хэше — достаём из tgWebAppData", () => {
    // Реальный кейс: на части клиентов WebApp.initData == "" хотя приложение
    // открыто из Telegram и параметры запуска лежат в location.hash.
    (window as unknown as { Telegram: { WebApp: { initData: string } } }).Telegram = {
      WebApp: { initData: "" },
    };
    setHash(RAW);
    expect(resolveInitData()).toBe(RAW);
  });

  it("после перезагрузки (хэш потерян) восстанавливает из sessionStorage", () => {
    // Первый запуск: достали из хэша и сохранили.
    setHash(RAW);
    expect(resolveInitData()).toBe(RAW);
    // Перезагрузка SPA: хэш ушёл, WebApp.initData пуст.
    window.location.hash = "";
    (window as unknown as { Telegram: { WebApp: { initData: string } } }).Telegram = {
      WebApp: { initData: "" },
    };
    expect(resolveInitData()).toBe(RAW);
  });

  it("кэширует найденный initData в sessionStorage", () => {
    setHash(RAW);
    resolveInitData();
    expect(window.sessionStorage.getItem(INIT_DATA_STORAGE_KEY)).toBe(RAW);
  });

  it("возвращает пустую строку, когда взять неоткуда", () => {
    expect(resolveInitData()).toBe("");
  });
});
