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
    // Bot API 8.0+: у части iOS-клиентов первый вызов может не сработать.
    // Повторяем мягко, пока viewport не станет fullscreen.
    const isV8 = typeof w.isVersionAtLeast === "function" ? w.isVersionAtLeast("8.0") : false;
    const requestFullscreenSafe = () => {
      if (!isV8 || w.isFullscreen) return;
      try {
        w.requestFullscreen?.();
      } catch {
        /* ignore — не все клиенты (TG Desktop/старые iOS) поддерживают fullscreen */
      }
    };
    requestFullscreenSafe();
    const t1 = window.setTimeout(requestFullscreenSafe, 120);
    const t2 = window.setTimeout(requestFullscreenSafe, 420);
    const onViewportChanged = () => {
      requestFullscreenSafe();
      w.expand();
    };
    (w as { onEvent?: (eventType: string, eventHandler: () => void) => void }).onEvent?.(
      "viewportChanged",
      onViewportChanged,
    );
    // Чтобы случайный свайп вниз не сворачивал аппу. Доступно с 7.7.
    try {
      w.disableVerticalSwipes?.();
    } catch {
      /* no-op */
    }
    w.setHeaderColor?.("#0d0d12");
    w.setBackgroundColor?.("#0d0d12");
    setInitData(w.initData ?? "");
    const u = w.initDataUnsafe?.user;
    if (u) {
      setName(u.first_name || u.username || "друг");
      setUserId(u.id);
      const p = (u as { photo_url?: string }).photo_url;
      if (p && typeof p === "string") setPhotoUrl(p);
    }
    return () => {
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
