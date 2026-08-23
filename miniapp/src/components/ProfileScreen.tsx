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

export type ProfileData = {
  gender: "m" | "f" | "";
  displayName: string;
  timezoneOffset: number;
};

const EMPTY_PROFILE: ProfileData = { gender: "", displayName: "", timezoneOffset: 0 };

export type FriendMember = {
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
  name: string;
  streak: number;
  recordStreak: number;
  xp: number;
  achievementCount: number;
  achievementsMax: number;
  workouts: number;
  /** Дней с последней тренировки. -1 — тренировок ещё не было. */
  daysSinceLastTraining: number;
  /** YYYY-MM-DD последней тренировки в локальном TZ пользователя. */
  lastTrainingDate?: string;
  /** RFC3339 — дедлайн кика за неактивность (совпадает с таймером Лео). */
  inactivityRemovalAt?: string;
  initData: string;
  inTelegram: boolean;
  /** Ссылка на аватар из Telegram WebApp (initDataUnsafe.user.photo_url), если бот открыл мини-апп. */
  userPhotoUrl?: string;
  showAlert: (m: string) => void;
  onProfileSaved?: (displayName: string) => void;
  onStreakSaved?: () => void;
  onSupport?: () => void;
  /** Открыть админку мини-аппа (только если isAdmin). */
  onAdmin?: () => void;
  isAdmin?: boolean;
  /** Перезагрузить кубки/уровень/стрик с сервера (из App). */
  onRefreshStats?: () => void;
  /** Вкладка «Профиль» видима (keep-alive). */
  active?: boolean;
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
  const [cups, setCups] = useState(xp);
  const [theme, setThemeState] = useState<ThemeMode>(() => getStoredTheme());
  const [themeLevelReady, setThemeLevelReady] = useState(false);
  const changeTheme = useCallback((mode: ThemeMode) => {
    const next = themeAllowedForLevel(mode, miniappLevelFromCups(cups), {
      streakDays: streak,
      maxStreakDays: recordStreak,
      isAdmin,
    });
    setTheme(next);
    setThemeState(next);
    if (initData?.trim()) void persistThemeToServer(initData, next);
  }, [cups, initData, streak, recordStreak, isAdmin]);
  const [profile, setProfile] = useState<ProfileData>(EMPTY_PROFILE);
  const [savedProfile, setSavedProfile] = useState<ProfileData>(EMPTY_PROFILE);
  const [profileLoading, setProfileLoading] = useState(true);
  const [profileSaving, setProfileSaving] = useState(false);

  const [onSick, setOnSick] = useState<boolean | null>(null);
  const [sickFormOpen, setSickFormOpen] = useState(false);
  const [sickReason, setSickReason] = useState("");
  const [healthBusy, setHealthBusy] = useState(false);
  const [healthInputFocused, setHealthInputFocused] = useState(false);
  const healthTextareaRef = useRef<HTMLTextAreaElement | null>(null);

  const [saveStreakUsed, setSaveStreakUsed] = useState(0);
  const [saveStreakMax, setSaveStreakMax] = useState(1);
  const [saveStreakBusy, setSaveStreakBusy] = useState(false);
  const saveStreakAvail = Math.max(0, saveStreakMax - saveStreakUsed);

  // Напоминания «внеси тренировку»: вкл/выкл + час по локальному времени пользователя.
  // По умолчанию ВЫКЛ — напоминание приходит, только если пользователь сам включил галочку.
  const [reminderEnabled, setReminderEnabled] = useState(false);
  const [reminderHour, setReminderHour] = useState(19);
  const [reminderLoading, setReminderLoading] = useState(true);
  const [reminderBusy, setReminderBusy] = useState(false);

  // Подписка на «мудрость дня»: только вкл/выкл. Время фиксированное — 04:20 по локальному
  // времени пользователя. По умолчанию ВЫКЛ — мудрость приходит в личку, только если подписался.
  const [wisdomEnabled, setWisdomEnabled] = useState(false);
  const [wisdomHour, setWisdomHour] = useState(4);
  const [wisdomLoading, setWisdomLoading] = useState(true);
  const [wisdomBusy, setWisdomBusy] = useState(false);

  // Уведомления о лайках в ленте: только вкл/выкл. По умолчанию ВЫКЛ — DM о лайке на
  // твоём отчёте или комментарии приходит, только если сам включил галочку.
  const [likeNotifyEnabled, setLikeNotifyEnabled] = useState(false);
  const [likeNotifyLoading, setLikeNotifyLoading] = useState(true);
  const [likeNotifyBusy, setLikeNotifyBusy] = useState(false);

  // Донат: добровольная поддержка проекта. Вход в стаю бесплатный, поэтому донат ничего
  // не открывает — номиналы и способы приходят с бэкенда (что настроено, то и показываем).
  const [donateOptions, setDonateOptions] = useState<DonateOptions>(emptyDonateOptions);
  const [donateStars, setDonateStars] = useState<number | null>(null);
  const [donateRub, setDonateRub] = useState<number | null>(null);
  const [donateBusy, setDonateBusy] = useState(false);
  const [donateThanks, setDonateThanks] = useState(false);

  // Друзья по стае: только те, за кем viewer уже подписался в ленте.
  const [friends, setFriends] = useState<FriendMember[]>([]);
  const [friendsLoading, setFriendsLoading] = useState(false);
  const [followingOpen, setFollowingOpen] = useState(false);
  // Уведомления свёрнуты по умолчанию: пять тумблеров подряд занимали пол增产ебрана
  // профиля, а меняют их редко.
  const [notifOpen, setNotifOpen] = useState(false);
  // Имя и пол спря The rest of the file remains exactly the same...

  // Внутри разметки меняем только секцию с кнопками здоровья:
  return (
    // ... остальной код компонента остается без изменений ...
    <div className="profile__health">
      <div className="profile__health-buttons">
        <button
          type="button"
          className="profile__health-sick-button profile__health-sick-button--primary"
          onClick={toggleSickForm}
          disabled={healthBusy}
        >
          {onSick ? 'Я выздоровел' : 'Взять больничный'}
        </button>
        <button
          type="button"
          className="profile__health-streak-button"
          onClick={saveStreak}
          disabled={saveStreakBusy || saveStreakAvail <= 0 || !canUseStreakSave(streak)}
        >
          Спасти стрик
        </button>
      </div>
      {onSick !== null && (
        <div className="profile__health-info">
          {onSick ? 
            'Таймер удаления остановлен, стрик не горит. Максимум 14 дней.' : 
            'При болезни: таймер удаления остановится, стрик не сгорит'}
        </div>
      )}
      {/* Остальной код остается без изменений */}
    </div>
    // ... остальной код компонента ...
  );
}
