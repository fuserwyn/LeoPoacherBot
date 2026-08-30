import { useCallback, useEffect, useRef, useState } from "react";
import { STREAK_ACHIEVEMENTS, WORKOUT_ACHIEVEMENTS, workoutsWordRu } from "../lib/achievements";
import { inactivityHighlight } from "../lib/inactivityHighlight";
import { inactiveDaysFromRemovalRemaining, removalRemainingUntil } from "../lib/inactivityRemoval";
import { cupsLevelProgressBarPct, formatCupsLevelProgressLabel, miniappCupsLevelProgress, miniappLevelFromCups, miniappLevelName } from "../lib/miniappLevel";
import {
  canUseStreakSave,
  effectiveStreakDays,
  streakBurnLabel,
} from "../lib/streakLabel";
import {
  canUseLeopardTheme,
  canUseWildTheme,
  getStoredTheme,
  isThemeMode,
  persistThemeToServer,
  setTheme,
  themeAllowedForLevel,
  type ThemeMode,
} from "../lib/theme";
import {
  createCardDonatePayment,
  createStarsDonateInvoice,
  emptyDonateOptions,
  fetchDonateOptions,
  waitForDonationCompleted,
  type DonateOptions,
} from "../lib/donate";
import { DonateThanksToast } from "./DonateThanksToast";
import "./ProfileScreen.css";

const api = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

type ProfileData = {
  gender: "m" | "f" | "";
  displayName: string;
  timezoneOffset: number;
};

const EMPTY_PROFILE: ProfileData = { gender: "", displayName: "", timezoneOffset: 0 };

type FriendMember = {
  user_id: number;
  name: string;
  streak_days: number;
  following: boolean;
  notify_workouts: boolean;
};

function normalizeProfileData(profile: ProfileData): ProfileData {
  return {
    gender: profile.gender,
    displayName: profile.displayName.trim(),
    timezoneOffset: Math.max(-12, Math.min(12, Math.trunc(profile.timezoneOffset || 0))),
  };
}

type Props = {
  // пропсы компонента...
};

export function ProfileScreen({
  name,
  streak,
  recordStreak,
  xp,
  achievementCount,
  achievementsMax,
  workouts,
  daysSinceLastTraining,
  lastTrainingDate,
  inactivityRemovalAt,
  initData,
  inTelegram,
  userPhotoUrl,
  showAlert,
  onProfileSaved,
  onStreakSaved,
  onSupport,
  onAdmin,
  isAdmin = false,
  onRefreshStats,
  active = true,
}: Props) {
  // состояние компонента...

  const donate = useCallback(async (rub: number, stars: number) => {
    if (!inTelegram || !initData?.trim()) {
      showAlert("Открой мини-апп из Telegram (нужен initData).");
      return;
    }
    
    setDonateBusy(true);
    try {
      if (stars > 0) {
        const url = await createStarsDonateInvoice(api, initData, stars);
        if (!url) {
          showAlert("Ошибка выставления счёта в звёздах");
          return;
        }
        const ok = await waitForDonationCompleted(url);
        if (ok) {
          setDonateThanks(true);
          setTimeout(() => setDonateThanks(false), 5000);
        }
      } else if (rub > 0) {
        const url = await createCardDonatePayment(api, initData, rub);
        if (!url) {
          showAlert("Ошибка создания платежа");
          return;
        }
        window.open(url, "_blank");
        setDonateThanks(true);
        setTimeout(() => setDonateThanks(false), 5000);
      }
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Ошибка платежа");
    } finally {
      setDonateBusy(false);
    }
  }, [inTelegram, initData, showAlert]);

  // остальной код компонента...

  return (
    <div className="profile">
      {/* Остальная разметка профиля... */}
      
      <div className="profile__section">
        <h3 className="profile__section-title">Поддержать проект</h3>
        <div className="profile__donate-options">
          {donateOptions.cardTiersRub.length > 0 && (
            <>
              <button 
                className="profile__donate-btn"
                onClick={() => donate(100, 0)}
                disabled={donateBusy}
              >
                100 ₽
              </button>
              <button 
                className="profile__donate-btn"
                onClick={() => donate(300, 0)}
                disabled={donateBusy}
              >
                300 ₽
              </button>
              <button 
                className="profile__donate-btn"
                onClick={() => donate(1000, 0)}
                disabled={donateBusy}
              >
                1000 ₽
              </button>
            </>
          )}
          {donateOptions.starsTiers.length > 0 && (
            <>
              <button 
                className="profile__donate-btn"
                onClick={() => donate(0, 50)}
                disabled={donateBusy}
              >
                50 ☆
              </button>
              <button 
                className="profile__donate-btn"
                onClick={() => donate(0, 150)}
                disabled={donateBusy}
              >
                150 ☆
              </button>
              <button 
                className="profile__donate-btn"
                onClick={() => donate(0, 500)}
                disabled={donateBusy}
              >
                500 ☆
              </button>
            </>
          )}
        </div>
        <p className="profile__donate-note">Добровольная поддержка проекта. Ничего не открывает.</p>
      </div>

      {donateThanks && <DonateThanksToast />}
    </div>
  );
}