/**
 * Автоопределение часового пояса пользователя при первом открытии мини-аппа.
 * Бэкенд хранит смещение в часах относительно МСК (UTC+3) — конвертим из браузерного offset.
 *
 * Стратегия (минимум риска): если у пользователя на бэке timezone_offset == 0 (дефолт МСК)
 * И детектированное смещение != 0 — апдейтим. Это разовый upgrade нового пользователя
 * без перезаписи ручной настройки `#timezone +X` (если он осознанно её ставил).
 *
 * IANA-зону пока не сохраняем — её хранение требует миграции БД (отложено).
 */

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

const MOSCOW_UTC_OFFSET_HOURS = 3;

/**
 * Возвращает смещение пользователя относительно Москвы в часах,
 * округлённое к ближайшему целому (бэкенд хранит int -12..+12).
 * Если страна на 30/45-минутном offset — стрик считается с погрешностью до часа,
 * но это приемлемо для текущей логики.
 */
function detectOffsetFromMoscow(): number {
  // getTimezoneOffset() возвращает разницу UTC - local в минутах, поэтому со знаком минус.
  const browserOffsetMinutes = -new Date().getTimezoneOffset();
  const browserOffsetHours = browserOffsetMinutes / 60;
  const offset = Math.round(browserOffsetHours - MOSCOW_UTC_OFFSET_HOURS);
  if (offset < -12) return -12;
  if (offset > 12) return 12;
  return offset;
}

export function detectTimezoneInfo(): { offsetFromMoscow: number; ianaName: string } {
  let ianaName = "";
  try {
    ianaName = Intl.DateTimeFormat().resolvedOptions().timeZone || "";
  } catch {
    ianaName = "";
  }
  return { offsetFromMoscow: detectOffsetFromMoscow(), ianaName };
}

/**
 * Идемпотентно дотягивает TZ пользователя на бэке.
 * currentOffset — то, что уже хранится у пользователя (из /profile/load); если 0 и detected != 0 — обновляем.
 * Если currentOffset != 0 — не трогаем (значит пользователь уже сконфигурирован).
 */
export async function syncTimezoneIfNeeded(initData: string, currentOffset: number): Promise<void> {
  if (!apiBase || !initData.trim()) return;
  const { offsetFromMoscow } = detectTimezoneInfo();
  if (currentOffset !== 0) return;
  if (offsetFromMoscow === 0) return;
  try {
    await fetch(`${apiBase}/api/miniapp/profile/save`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData, timezone_offset: offsetFromMoscow }),
    });
  } catch {
    /* TZ-синк необязателен; ошибка не должна ломать инициализацию мини-аппа */
  }
}
