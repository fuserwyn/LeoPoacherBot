import { useEffect, useState } from "react";

/**
 * Подключает Telegram WebApp: разворот на весь экран, тёмный header под макет.
 * Вне Telegram (браузер) — no-op, имя из query ?name=.
 */
export function useTelegramWebApp() {
  const [name, setName] = useState("Стас");
  const [streak, setStreak] = useState(0);

  const [initData, setInitData] = useState("");
  const [userId, setUserId] = useState(0);

  const [photoUrl, setPhotoUrl] = useState<string | undefined>(undefined);

  useEffect(() => {
    const w = window.Telegram?.WebApp;
    if (!w) {
      const q = new URLSearchParams(window.location.search);
      const n = q.get("name");
      if (n) setName(n);
      const s = q.get("streak");
      if (s) setStreak(Number(s) || 0);
      return;
    }
    w.ready();
    w.expand();
    // Fullscreen и блокировка вертикальных свайпов нужны только на мобильных клиентах.
    // На Telegram Desktop / macOS / web requestFullscreen разворачивает окно во весь экран
    // и мешает пользователю менять его размер и сворачивать стандартными контролами окна.
    const platform = (w.platform ?? "").toLowerCase();
    const isMobile = platform === "ios" || platform === "android" || platform === "android_x";
    const isV8 = typeof w.isVersionAtLeast === "function" ? w.isVersionAtLeast("8.0") : false;
    const requestFullscreenSafe = () => {
      if (!isMobile || !isV8 || w.isFullscreen) return;
      try {
        w.requestFullscreen?.();
      } catch {
        /* ignore — не все клиенты (TG Desktop/старые iOS) поддерживают fullscreen */
      }
    };
    requestFullscreenSafe();
    const t1 = isMobile ? window.setTimeout(requestFullscreenSafe, 120) : 0;
    const t2 = isMobile ? window.setTimeout(requestFullscreenSafe, 420) : 0;
    // initData на части клиентов (особенно iOS) появляется в объекте WebApp не
    // мгновенно после ready(), а спустя несколько тиков. Если считать его один
    // раз и он окажется пустым — пустая строка «прилипает» в стейте и навсегда
    // блокирует все запросы к API (добавление тренировки, бейджи, access-gate)
    // с ошибкой «нужен initData». Поэтому дочитываем его, пока не появится
    // непустая строка (или пока не истечёт пара секунд попыток).
    const syncInitData = () => {
      const fresh = w.initData ?? "";
      if (fresh) setInitData(fresh);
      return Boolean(fresh);
    };
    const onViewportChanged = () => {
      requestFullscreenSafe();
      if (isMobile) w.expand();
      syncInitData();
    };
    (w as { onEvent?: (eventType: string, eventHandler: () => void) => void }).onEvent?.(
      "viewportChanged",
      onViewportChanged,
    );
    // Чтобы случайный свайп вниз не сворачивал аппу. Доступно с 7.7. Только для мобильных.
    if (isMobile) {
      try {
        w.disableVerticalSwipes?.();
      } catch {
        /* no-op */
      }
    }
    w.setHeaderColor?.("#0d0d12");
    w.setBackgroundColor?.("#0d0d12");
    let initDataTries = 0;
    const initDataTimer = window.setInterval(() => {
      initDataTries += 1;
      if (syncInitData() || initDataTries >= 20) window.clearInterval(initDataTimer);
    }, 150);
    syncInitData();
    const u = w.initDataUnsafe?.user;
    if (u) {
      setName(u.first_name || u.username || "друг");
      setUserId(u.id);
      const p = (u as { photo_url?: string }).photo_url;
      if (p && typeof p === "string") setPhotoUrl(p);
    }
    return () => {
      window.clearInterval(initDataTimer);
      window.clearTimeout(t1);
      window.clearTimeout(t2);
      (w as { offEvent?: (eventType: string, eventHandler: () => void) => void }).offEvent?.(
        "viewportChanged",
        onViewportChanged,
      );
    };
  }, []);

  return {
    name,
    streak,
    setStreak,
    initData,
    userId,
    photoUrl,
    inTelegram: Boolean(window.Telegram?.WebApp),
    tg: window.Telegram?.WebApp,
  };
}
