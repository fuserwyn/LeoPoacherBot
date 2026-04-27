/**
 * Идемпотентный хендшейк мини-аппа после оплаты:
 * на первом открытии стартует таймер неактивности и пишет pack_join/pack_rejoin в ленту.
 * Следующие вызовы — no-op, можно дёргать без бойлерплейта.
 */

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type OnboardingEnsureResult = {
  ok: boolean;
  inPack: boolean;
  justOnboarded: boolean;
  isRejoin: boolean;
};

export async function ensureMiniappOnboarding(initData: string): Promise<OnboardingEnsureResult> {
  const empty: OnboardingEnsureResult = { ok: false, inPack: false, justOnboarded: false, isRejoin: false };
  if (!apiBase || !initData.trim()) return empty;
  try {
    const res = await fetch(`${apiBase}/api/miniapp/onboarding/ensure`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ init_data: initData }),
    });
    const j = (await res.json().catch(() => ({}))) as {
      ok?: boolean;
      in_pack?: boolean;
      just_onboarded?: boolean;
      is_rejoin?: boolean;
    };
    if (!res.ok || !j.ok) return empty;
    return {
      ok: true,
      inPack: !!j.in_pack,
      justOnboarded: !!j.just_onboarded,
      isRejoin: !!j.is_rejoin,
    };
  } catch {
    return empty;
  }
}
