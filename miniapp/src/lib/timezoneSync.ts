/**
 * Часовой пояс пользователя определяется автоматически из устройства (системная зона телефона)
 * и синхронизируется на бэкенд при каждом запуске мини-аппа. Это единственный источник правды —
 * ручной команды (#timezone) больше нет.
 *
 * Бэкенд хранит смещение в часах относительно МСК (UTC+3); конвертим из браузерного offset.
 * IANA-зону не храним — вся серверная логика (стрик, таймеры, дедлайны) работает по int-смещению.
 */

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

const MOSCOW_UTC_OFFSET_HOURS = 3;

/**
 * Смещение пользователя относительно Москвы в часах, округлённое к ближайшему целому
 * (бэкенд хранит int -12..+12). На зонах с 30/45-мин offset стрик считается с погрешностью
 * до часа — это приемлемо для текущей логики.
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
 * Приводит хранимое на бэке смещение к фактическому часовому поясу устройства.
 * Шлём profile/save только если оно отличается от сохранённого, чтобы не дёргать сеть зря.
 * @param currentOffset — текущее смещение пользователя из /profile/load.
 */
export async function syncDeviceTimezone(initData: string, currentOffset: number): Promise<void> {
  if (!apiBase || !initData.trim()) return;
  const { offsetFromMoscow } = detectTimezoneInfo();
  if (offsetFromMoscow === currentOffset) return;
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
