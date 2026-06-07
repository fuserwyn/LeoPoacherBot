// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { renderHook, waitFor, cleanup } from "@testing-library/react";
import { useTelegramWebApp } from "./useTelegramWebApp";

// Подписанная строка, какую Telegram кладёт в WebApp.initData.
const RAW =
  "query_id=AAH123&user=%7B%22id%22%3A777%2C%22first_name%22%3A%22Leo%22%7D&auth_date=1700000000&hash=abc123";

/** Минимальный стаб window.Telegram.WebApp с управляемым initData. */
function stubWebApp(initData: string) {
  const wa = {
    initData,
    initDataUnsafe: { user: { id: 777, first_name: "Leo" } },
    platform: "android",
    colorScheme: "dark",
    themeParams: {},
    isExpanded: true,
    ready() {},
    expand() {},
    isVersionAtLeast: () => false,
    setHeaderColor() {},
    setBackgroundColor() {},
    onEvent() {},
    offEvent() {},
    showAlert() {},
    close() {},
  };
  (window as unknown as { Telegram: { WebApp: typeof wa } }).Telegram = { WebApp: wa };
  return wa;
}

beforeEach(() => {
  window.location.hash = "";
  window.sessionStorage.clear();
  delete (window as { Telegram?: unknown }).Telegram;
});
afterEach(() => {
  cleanup();
  window.location.hash = "";
  window.sessionStorage.clear();
  delete (window as { Telegram?: unknown }).Telegram;
});

describe("useTelegramWebApp — воспроизведение бага «нужен initData»", () => {
  it("БАГ-СЦЕНАРИЙ: WebApp есть, initData пуст, хэша нет → публикация заблокирована", async () => {
    // Ровно то, что видел юзер: открыт из Telegram (inTelegram=true), но initData
    // так и не появился. Условие гейта публикации (!inTelegram || !initData) = true.
    stubWebApp("");
    const { result } = renderHook(() => useTelegramWebApp());
    // Дать хуку отработать ретраи (interval 150мс × 20). initData всё равно "".
    await new Promise((r) => setTimeout(r, 400));
    expect(result.current.inTelegram).toBe(true);
    expect(result.current.initData).toBe("");
    const publishBlocked = !result.current.inTelegram || !result.current.initData;
    expect(publishBlocked).toBe(true); // ← баг: юзер не может опубликовать
  });

  it("ФИКС: WebApp.initData пуст, но запуск пришёл в хэше → initData восстановлен, публикация разрешена", async () => {
    stubWebApp("");
    window.location.hash = `#tgWebAppData=${encodeURIComponent(RAW)}&tgWebAppVersion=7.0`;
    const { result } = renderHook(() => useTelegramWebApp());
    await waitFor(() => expect(result.current.initData).toBe(RAW));
    const publishBlocked = !result.current.inTelegram || !result.current.initData;
    expect(publishBlocked).toBe(false); // ← починено
  });

  it("ФИКС: после перезагрузки (хэш потерян) initData берётся из sessionStorage", async () => {
    // Первый запуск наполнил sessionStorage.
    stubWebApp("");
    window.location.hash = `#tgWebAppData=${encodeURIComponent(RAW)}`;
    const first = renderHook(() => useTelegramWebApp());
    await waitFor(() => expect(first.result.current.initData).toBe(RAW));
    first.unmount();

    // Перезагрузка: хэш ушёл, WebApp.initData по-прежнему пуст.
    window.location.hash = "";
    stubWebApp("");
    const { result } = renderHook(() => useTelegramWebApp());
    await waitFor(() => expect(result.current.initData).toBe(RAW));
  });

  it("ФИКС: скрипт telegram-web-app.js не загрузился (WebApp нет), но хэш есть → не лочим юзера", async () => {
    // window.Telegram отсутствует целиком, но фрагмент запуска на месте.
    window.location.hash = `#tgWebAppData=${encodeURIComponent(RAW)}`;
    const { result } = renderHook(() => useTelegramWebApp());
    await waitFor(() => expect(result.current.initData).toBe(RAW));
    expect(result.current.inTelegram).toBe(true); // initData валиден ⇒ считаем «в Telegram»
  });
});
