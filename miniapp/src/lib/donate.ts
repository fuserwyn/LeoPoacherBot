...
/**
 * Донаты из профиля: добровольная поддержка проекта.
 * Вход в стаю бесплатный — донат ничего не открывает и не отменяет выбывание.
 *
 * Звёзды: бэкенд отдаёт invoice link, оплата идёт через WebApp.openInvoice, не покидая мини-апп.
 * Карта РФ: бэкенд отдаёт ссылку ЮKassa, её открываем через WebApp.openLink и затем
 * опрашиваем статус — вебхук платежей донаты не закрывает (см. ms_leo/internal/bot/donate.go).
 */

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type DonateOptions = {
  starsTiers: number[];
  cardTiersRub: number[];
  starsAvailable: boolean;
  cardAvailable: boolean;
  completedCount: number;
};

export const emptyDonateOptions: DonateOptions = {
  starsTiers: [],
  cardTiersRub: [],
  starsAvailable: false,
  cardAvailable: false,
  completedCount: 0,
};

async function post<T>(path: string, payload: Record<string, unknown>): Promise<T | null> {
  if (!apiBase) return null;
  try {
    const res = await fetch(`${apiBase}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const j = (await res.json().catch(() => ({}))) as { ok?: boolean } & T;
    if (!res.ok || !j.ok) return null;
    return j;
  } catch {
    return null;
  }
}

export async function fetchDonateOptions(initData: string): Promise<DonateOptions> {
  if (!initData.trim()) return emptyDonateOptions;
  const j = await post<{
    stars_tiers?: number[];
    card_tiers_rub?: number[];
    stars_available?: boolean;
    card_available?: boolean;
    completed_count?: number;
  }>("/api/miniapp/donate/options", { init_data: initData });
  if (!j) return emptyDonateOptions;
  return {
    starsTiers: Array.isArray(j.stars_tiers) ? j.stars_tiers : [],
    cardTiersRub: Array.isArray(j.card_tiers_rub) ? j.card_tiers_rub : [],
    starsAvailable: !!j.stars_available,
    cardAvailable: !!j.card_available,
    completedCount: typeof j.completed_count === "number" ? j.completed_count : 0,
  };
}

export type DonateInvoice = { link: string; donationId: number };

export async function createStarsDonateInvoice(initData: string, stars: number): Promise<DonateInvoice | null> {
  const j = await post<{ invoice_link?: string; donation_id?: number }>("/api/miniapp/donate/stars", {
    init_data: initData,
    stars,
  });
  if (!j?.invoice_link || !j.donation_id) return null;
  return { link: j.invoice_link, donationId: j.donation_id };
}

export async function createCardDonatePayment(initData: string, rub: number): Promise<DonateInvoice | null> {
  const j = await post<{ confirmation_url?: string; donation_id?: number }>("/api/miniapp/donate/card", {
    init_data: initData,
    rub,
  });
  if (!j?.confirmation_url || !j.donation_id) return null;
  return { link: j.confirmation_url, donationId: j.donation_id };
}

export async function fetchDonationStatus(initData: string, donationId: number): Promise<"pending" | "completed" | null> {
  const j = await post<{ status?: string }>("/api/miniapp/donate/status", {
    init_data: initData,
    donation_id: donationId,
  });
  if (!j) return null;
  return j.status === "completed" ? "completed" : "pending";
}

/**
 * Опрос статуса доната картой после возврата из браузера: пользователь может вернуться
 * раньше, чем ЮKassa проведёт платёж, поэтому ждём несколько попыток и молча сдаёмся —
 * бот всё равно догонит «спасибо» при следующем /start (DonateSyncPendingForUser).
 */
export async function waitForDonationCompleted(
  initData: string,
  donationId: number,
  attempts = 6,
  delayMs = 2500,
): Promise<boolean> {
  for (let i = 0; i < attempts; i += 1) {
    if (i > 0) await new Promise((r) => setTimeout(r, delayMs));
    const status = await fetchDonationStatus(initData, donationId);
    if (status === "completed") return true;
  }
  return false;
}
