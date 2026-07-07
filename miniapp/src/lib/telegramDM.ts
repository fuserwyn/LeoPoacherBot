/** Открытие лички участника из мини-аппа. В поле `username` ленты для юзеров с TG-ником
 *  лежит «@nick», иначе — отображаемое имя (имя/фамилия), по которому личку не открыть. */

/** TG-ник без «@» из username ленты; undefined, если это не похоже на настоящий @username. */
export function feedTgUsername(raw: string | undefined): string | undefined {
  const t = (raw ?? "").trim();
  if (!t.startsWith("@")) return undefined;
  const nick = t.slice(1);
  return /^[a-zA-Z0-9_]{4,32}$/.test(nick) ? nick : undefined;
}

type TgWebApp = { openTelegramLink?: (url: string) => void; platform?: string };

/** Открыть личку с участником: t.me/<ник> внутри Telegram (openTelegramLink),
 *  вне Telegram — новой вкладкой; без ника — диплинк по user id (Android понимает
 *  tg://openmessage, остальные клиенты — tg://user; сработает, если приватность
 *  собеседника позволяет открыть профиль по id). */
export function openTelegramDM(p: { tgUsername?: string; userId?: number }): void {
  const wa = (window as unknown as { Telegram?: { WebApp?: TgWebApp } }).Telegram?.WebApp;
  if (p.tgUsername) {
    const url = `https://t.me/${p.tgUsername}`;
    if (wa?.openTelegramLink) {
      try {
        wa.openTelegramLink(url);
        return;
      } catch {
        // старый клиент без метода — падаем на window.open ниже
      }
    }
    window.open(url, "_blank", "noopener");
    return;
  }
  if (p.userId && p.userId > 0) {
    const deepLink =
      wa?.platform === "android"
        ? `tg://openmessage?user_id=${p.userId}`
        : `tg://user?id=${p.userId}`;
    window.location.href = deepLink;
  }
}
