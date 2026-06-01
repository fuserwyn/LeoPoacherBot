import { useCallback, useEffect, useRef, useState } from "react";
import { inactivityHighlight } from "../lib/inactivityHighlight";
import { cupsLevelProgressBarPct, formatCupsLevelProgressLabel, miniappCupsLevelProgress, miniappLevelFromCups, miniappLevelName } from "../lib/miniappLevel";
import { effectiveStreakDays, streakBurnLabel } from "../lib/streakLabel";
import "./ProfileScreen.css";

const api = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type ProfileData = {
  gender: "m" | "f" | "";
  displayName: string;
  timezoneOffset: number;
};

const EMPTY_PROFILE: ProfileData = { gender: "", displayName: "", timezoneOffset: 0 };

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
  initData: string;
  inTelegram: boolean;
  /** Ссылка на аватар из Telegram WebApp (initDataUnsafe.user.photo_url), если бот открыл мини-апп. */
  userPhotoUrl?: string;
  showAlert: (m: string) => void;
  onProfileSaved?: (displayName: string) => void;
  onStreakSaved?: () => void;
  onSupport?: () => void;
  /** Перезагрузить кубки/уровень/стрик с сервера (из App). */
  onRefreshStats?: () => void;
  /** Вкладка «Профиль» видима (keep-alive). */
  active?: boolean;
};

const STREAK_ACHIEVEMENTS = [
  { days: 7, colorClass: "profile__achievement--7", variant: "paw" },
  { days: 14, colorClass: "profile__achievement--14", variant: "paw" },
  { days: 30, colorClass: "profile__achievement--30", variant: "paw" },
  { days: 42, colorClass: "profile__achievement--42", variant: "heart" },
  { days: 60, colorClass: "profile__achievement--60", variant: "paw" },
  { days: 90, colorClass: "profile__achievement--90", variant: "paw" },
  { days: 180, colorClass: "profile__achievement--180", variant: "paw" },
  { days: 365, colorClass: "profile__achievement--365", variant: "paw" },
  { days: 420, colorClass: "profile__achievement--420", variant: "paw-crown" },
] as const;

// Ачивки за общее число тренировок. «Заработано» считаем на фронте из total workouts —
// бэкенд про эти пороги не знает (это чисто визуальная витрина в профиле).
// variant: внутри звезды лапка ("paw") или сердце ("heart", для 42 — как у стрика).
const WORKOUT_ACHIEVEMENTS = [
  { count: 10, variant: "paw" },
  { count: 20, variant: "paw" },
  { count: 42, variant: "heart" },
  { count: 50, variant: "paw" },
  { count: 100, variant: "paw" },
  { count: 200, variant: "paw" },
  { count: 420, variant: "paw" },
  { count: 500, variant: "paw" },
  { count: 1000, variant: "paw" },
] as const;

// Правильное склонение: 1 тренировка, 2-4 тренировки, 5-20 тренировок.
function workoutsWordRu(n: number): string {
  const mod100 = n % 100;
  if (mod100 >= 11 && mod100 <= 14) return "тренировок";
  const mod10 = n % 10;
  if (mod10 === 1) return "тренировка";
  if (mod10 >= 2 && mod10 <= 4) return "тренировки";
  return "тренировок";
}

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
  initData,
  inTelegram,
  userPhotoUrl,
  showAlert,
  onProfileSaved,
  onStreakSaved,
  onSupport,
  onRefreshStats,
  active = true,
}: Props) {
  const [cups, setCups] = useState(xp);
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

  useEffect(() => {
    setCups(xp);
  }, [xp]);

  // Время, чтобы пересчитывать остаток до сгорания стрика. Тикает раз в минуту, пока вкладка видима.
  const [now, setNow] = useState(() => new Date());
  const displayStreak = effectiveStreakDays(streak, daysSinceLastTraining, now, lastTrainingDate);
  const burnLabel = streakBurnLabel(streak, daysSinceLastTraining, now, lastTrainingDate);

  // Сколько ачивок за тренировки уже открыто = число порогов, не превышающих total.
  const workoutAchEarned = WORKOUT_ACHIEVEMENTS.filter(({ count }) => workouts >= count).length;

  const scrollHealthAboveKeyboard = useCallback(() => {
    const ta = healthTextareaRef.current;
    if (!ta) return;
    const gap = 10;
    // Граница, ниже которой поле нельзя опускать, — это РЕАЛЬНЫЙ верх зафиксированной
    // панели с кнопкой «Отправить заявку». Привязка к её позиции даёт одинаковый
    // результат на iOS и Android. Раньше граница считалась по visualViewport, который
    // в Android TG WebView расходится с позицией панели → поле уезжало слишком вверх.
    const bar = document.querySelector<HTMLElement>(".profile__health-actions--keyboard");
    let limitBottom: number;
    if (bar) {
      limitBottom = bar.getBoundingClientRect().top - gap;
    } else {
      const vv = window.visualViewport;
      const visualH = vv?.height ?? window.innerHeight;
      const offsetTop = vv?.offsetTop ?? 0;
      limitBottom = offsetTop + visualH - 44 - gap;
    }
    const taRect = ta.getBoundingClientRect();
    const delta = taRect.bottom - limitBottom;
    // Порог гасит дрожание/петлю обратной связи: прокручиваем только если поле реально
    // перекрыто панелью/клавиатурой. behavior:"auto" — мгновенно, чтобы плавный скролл
    // не плодил новые scroll-события и не запускал прокрутку повторно.
    if (delta > 4) {
      window.scrollBy({ top: delta, behavior: "auto" });
    }
  }, []);

  // Пока вкладка видима и есть что показывать — пересчитываем остаток до сгорания стрика раз в минуту.
  useEffect(() => {
    if (!active || burnLabel == null) return;
    setNow(new Date()); // освежаем при возврате на вкладку, не дожидаясь первого тика
    const id = window.setInterval(() => setNow(new Date()), 60_000);
    return () => window.clearInterval(id);
  }, [active, burnLabel]);

  const cupProgress = miniappCupsLevelProgress(cups);
  const cupProgressLabel = formatCupsLevelProgressLabel(cupProgress);
  const level = miniappLevelFromCups(cups);
  const levelTitle = miniappLevelName(level) || "—";
  const barPct = cupsLevelProgressBarPct(cupProgress);

  const load = useCallback(async () => {
    if (!api || !inTelegram || !initData?.trim()) {
      setProfileLoading(false);
      return;
    }
    setProfileLoading(true);
    try {
      const res = await fetch(`${api}/api/miniapp/profile/load`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData }),
      });
      const j = (await res.json().catch(() => ({}))) as {
        ok?: boolean;
        error?: string;
        gender?: string;
        display_name?: string;
        timezone_offset?: number;
        xp?: number;
        streak_save_attempts_used?: number;
        streak_save_attempts_max?: number;
      };
      if (!res.ok) {
        showAlert(j.error ?? `Профиль: ошибка ${res.status}`);
        return;
      }
      if (!j.ok) return;
      const g = j.gender === "m" || j.gender === "f" ? j.gender : "";
      const dn = (j.display_name ?? "").trim() || (name && name !== "друг" ? name : "");
      const tz =
        typeof j.timezone_offset === "number" && Number.isFinite(j.timezone_offset)
          ? Math.max(-12, Math.min(12, Math.trunc(j.timezone_offset)))
          : 0;
      const nextProfile = {
        gender: g,
        displayName: dn,
        timezoneOffset: tz,
      };
      setProfile(nextProfile);
      setSavedProfile(normalizeProfileData(nextProfile));
      if (typeof j.xp === "number") {
        setCups(Math.max(0, j.xp));
      }
      if (typeof j.streak_save_attempts_used === "number") {
        setSaveStreakUsed(Math.max(0, j.streak_save_attempts_used));
      }
      if (typeof j.streak_save_attempts_max === "number") {
        setSaveStreakMax(Math.max(1, j.streak_save_attempts_max));
      }
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
    } finally {
      setProfileLoading(false);
    }
  }, [inTelegram, initData, name, showAlert]);

  useEffect(() => {
    if (!active) return;
    void load();
    onRefreshStats?.();
  }, [load, active, onRefreshStats]);

  const loadHealth = useCallback(async () => {
    if (!api || !inTelegram || !initData?.trim()) {
      setOnSick(false);
      return;
    }
    try {
      const res = await fetch(`${api}/api/miniapp/health/status`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData }),
      });
      const j = (await res.json().catch(() => ({}))) as { ok?: boolean; on_sick?: boolean };
      if (res.ok && j.ok) {
        setOnSick(Boolean(j.on_sick));
      }
    } catch {
      // тихо: не критично, ниже UI просто не блокируется
    }
  }, [inTelegram, initData]);

  useEffect(() => {
    if (!active) return;
    void loadHealth();
  }, [loadHealth, active]);

  const sendHealthMessage = useCallback(
    async (text: string) => {
      if (!api || !inTelegram || !initData?.trim()) {
        showAlert("Открой мини-апп из Telegram (нужен initData).");
        return false;
      }
      try {
        const res = await fetch(`${api}/api/miniapp/messages`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, text }),
        });
        const j = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string };
        if (!res.ok || !j.ok) {
          showAlert(j.error ?? `Ошибка ${res.status}`);
          return false;
        }
        return true;
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
        return false;
      }
    },
    [inTelegram, initData, showAlert],
  );

  const submitSickLeave = useCallback(async () => {
    const reason = sickReason.trim();
    if (reason.length < 3) {
      showAlert("Опиши, что случилось — пара слов минимум.");
      return;
    }
    setHealthBusy(true);
    const ok = await sendHealthMessage(`#sick_leave ${reason}`);
    setHealthBusy(false);
    if (ok) {
      void window.Telegram?.WebApp?.HapticFeedback?.notificationOccurred?.("success");
      setSickFormOpen(false);
      setSickReason("");
      setOnSick(true);
      showAlert("Заявка отправлена. Ответ Лео — во вкладке Чат.");
      setTimeout(() => void loadHealth(), 1200);
    }
  }, [sickReason, sendHealthMessage, loadHealth, showAlert]);

  const submitHealthy = useCallback(async () => {
    setHealthBusy(true);
    const ok = await sendHealthMessage("#healthy");
    setHealthBusy(false);
    if (ok) {
      void window.Telegram?.WebApp?.HapticFeedback?.notificationOccurred?.("success");
      setOnSick(false);
      showAlert("С выздоровлением");
      setTimeout(() => {
        void loadHealth();
        void load();
      }, 1200);
    }
  }, [sendHealthMessage, loadHealth, load, showAlert]);

  const useSaveStreak = useCallback(async () => {
    if (!api || !inTelegram || !initData?.trim()) {
      showAlert("Открой мини-апп из Telegram (нужен initData).");
      return;
    }
    if (saveStreakAvail <= 0 || saveStreakBusy) return;
    setSaveStreakBusy(true);
    try {
      const res = await fetch(`${api}/api/miniapp/streak/save-use`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData }),
      });
      const j = (await res.json().catch(() => ({}))) as {
        ok?: boolean;
        error?: string;
        streak_days?: number;
        streak_save_attempts_used?: number;
        streak_save_attempts_max?: number;
      };
      if (typeof j.streak_save_attempts_used === "number") {
        setSaveStreakUsed(Math.max(0, j.streak_save_attempts_used));
      }
      if (typeof j.streak_save_attempts_max === "number") {
        setSaveStreakMax(Math.max(1, j.streak_save_attempts_max));
      }
      if (!res.ok || !j.ok) {
        if (j.error === "no_attempts") {
          showAlert("Попыток не осталось — расти по уровням, чтобы получить ещё.");
        } else if (j.error === "not_needed") {
          showAlert("Стрик ещё жив — спасение не нужно.");
        } else if (j.error === "too_late") {
          showAlert("Пропущено больше одного дня — одной попыткой не восстановить.");
        } else if (j.error === "nothing_to_save") {
          showAlert("Нечего восстанавливать — стрика не было.");
        } else {
          showAlert(j.error ?? `Ошибка ${res.status}`);
        }
        return;
      }
      void window.Telegram?.WebApp?.HapticFeedback?.notificationOccurred?.("success");
      onStreakSaved?.();
      showAlert(
        typeof j.streak_days === "number" && j.streak_days > 0
          ? `Стрик восстановлен: ${j.streak_days} ${j.streak_days === 1 ? "день" : j.streak_days < 5 ? "дня" : "дней"}.`
          : "Попытка использована — стрик защищён.",
      );
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
    } finally {
      setSaveStreakBusy(false);
    }
  }, [inTelegram, initData, saveStreakAvail, saveStreakBusy, showAlert, onStreakSaved]);

  const saveProfile = useCallback(async () => {
    if (!api || !inTelegram || !initData?.trim()) {
      showAlert("Открой мини-апп из Telegram (нужен initData).");
      return;
    }
    setProfileSaving(true);
    try {
      const normalizedProfile = normalizeProfileData(profile);
      const body: Record<string, unknown> = {
        init_data: initData,
        gender: normalizedProfile.gender,
        display_name: normalizedProfile.displayName,
        timezone_offset: normalizedProfile.timezoneOffset,
      };
      const res = await fetch(`${api}/api/miniapp/profile/save`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const j = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string };
      if (!res.ok) {
        showAlert(j.error ?? `Сохранение: ${res.status}`);
        return;
      }
      if (j.ok) {
        setProfile(normalizedProfile);
        setSavedProfile(normalizedProfile);
        void window.Telegram?.WebApp?.HapticFeedback?.notificationOccurred?.("success");
        onProfileSaved?.(normalizedProfile.displayName);
        showAlert("Сохранено");
      }
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
    } finally {
      setProfileSaving(false);
    }
  }, [inTelegram, initData, onProfileSaved, profile, showAlert]);

  const profileDirty = (() => {
    const current = normalizeProfileData(profile);
    return (
      current.gender !== savedProfile.gender ||
      current.displayName !== savedProfile.displayName ||
      current.timezoneOffset !== savedProfile.timezoneOffset
    );
  })();

  const inactiveHighlight = inactivityHighlight(daysSinceLastTraining);
  const showDaysWithoutTraining = daysSinceLastTraining >= 5;
  const showKickBanner = daysSinceLastTraining >= 7;

  const healthActionsKeyboard = sickFormOpen && healthInputFocused;

  useEffect(() => {
    if (!healthInputFocused || !sickFormOpen) return;
    const run = () => scrollHealthAboveKeyboard();
    run();
    const t1 = window.setTimeout(run, 100);
    const t2 = window.setTimeout(run, 280);
    const vv = window.visualViewport;
    // Только resize (показ/скрытие клавиатуры). На scroll НЕ подписываемся: он срабатывает
    // во время самой прокрутки и при ручном скролле пользователя → петля «уезжания» вверх.
    vv?.addEventListener("resize", run);
    return () => {
      window.clearTimeout(t1);
      window.clearTimeout(t2);
      vv?.removeEventListener("resize", run);
    };
  }, [healthInputFocused, sickFormOpen, scrollHealthAboveKeyboard]);

  return (
    <div
      className={`profile${healthActionsKeyboard ? " profile--health-keyboard" : ""}${
        inactiveHighlight !== "none" ? ` profile--inactive-${inactiveHighlight}` : ""
      }`}
    >
      <header className="profile__hero">
        <div
          className={`profile__avatar${
            inactiveHighlight !== "none" ? ` profile__avatar--inactive-${inactiveHighlight}` : ""
          }`}
          aria-hidden
        >
          {userPhotoUrl ? (
            <img src={userPhotoUrl} alt="" className="profile__avatar-img" width={88} height={88} />
          ) : (
            "🐆"
          )}
          {onSick ? <span className="profile__avatar-med">🌡️</span> : null}
        </div>
        <div>
          <h1 className="profile__name">{(profile.displayName || name).trim() || "Стая"}</h1>
          <p className="profile__level muted">
            Уровень {level} · {levelTitle}
          </p>
          {showDaysWithoutTraining ? (
            <p
              className={`profile__kick profile__kick--inactive-${inactiveHighlight}`}
              title={`Дней без тренировок: ${daysSinceLastTraining}`}
            >
              Дней без тренировок: {daysSinceLastTraining}
            </p>
          ) : null}
          {showKickBanner ? (
            <p className="profile__danger">ПОТРЕНИРУЙСЯ ДО 00:00 ИЛИ ЛЕО СЪЕСТ ТЕБЯ</p>
          ) : null}
        </div>
        <div
          className="profile__xp"
          aria-label={`Кубки: ${cupProgressLabel}, уровень ${level}`}
        >
          <div className="profile__xp-meter">
            <span className="profile__xp-caption" aria-hidden>
              🏆
            </span>
            <div className="profile__xp-bar">
              <div className="profile__xp-fill" style={{ width: `${barPct}%` }} />
            </div>
          </div>
          <span className="profile__xp-txt">{cupProgressLabel}</span>
        </div>
      </header>

      <div className="profile__grid3">
        <div className="profile__stat-slot">
          <div
            className={`stat-card${
              inactiveHighlight !== "none" ? ` stat-card--inactive-${inactiveHighlight}` : ""
            }`}
          >
            <div className="stat-card__label">Дней подряд</div>
            <div className="stat-card__val">{displayStreak}</div>
          </div>
          {burnLabel ? (
            <div className="stat-card__burn" title={`Стрик ${burnLabel}`}>
              🔥 {burnLabel}
            </div>
          ) : null}
        </div>
        <div
          className={`stat-card${
            inactiveHighlight !== "none" ? ` stat-card--inactive-${inactiveHighlight}` : ""
          }`}
        >
          <div className="stat-card__label">Рекорд стрика</div>
          <div className="stat-card__val">{recordStreak}</div>
        </div>
        <div
          className={`stat-card${
            inactiveHighlight !== "none" ? ` stat-card--inactive-${inactiveHighlight}` : ""
          }`}
        >
          <div className="stat-card__label">Всего тренировок</div>
          <div className="stat-card__val">{workouts}</div>
        </div>
      </div>

      <section className="profile__achievements" aria-label="Ачивки">
        <div className="profile__achievements-head">
          <h2 className="section-title profile__achievements-title">Ачивки</h2>
          <span className="profile__achievements-count">
            {achievementCount + workoutAchEarned}/{achievementsMax + WORKOUT_ACHIEVEMENTS.length}
          </span>
        </div>
        <div className="profile__achievements-group">Ачивки за стрики</div>
        <div className="profile__achievements-strip">
        {STREAK_ACHIEVEMENTS.map(({ days, colorClass, variant }, i) => (
          <div key={days} className={`profile__achievement ${colorClass}${i < achievementCount ? " is-earned" : ""}`}>
            <div className="profile__achievement-badge" aria-hidden>
              {variant === "heart" ? (
                <svg className="profile__achievement-paw profile__achievement-heart" viewBox="0 0 64 64">
                  <circle className="profile__achievement-bg" cx="32" cy="32" r="28" />
                  <path
                    className="profile__achievement-heart-shell"
                    d="M32 56 C18 46 8 38 8 26 C8 18 14 12 22 12 C26 12 30 14 32 18 C34 14 38 12 42 12 C50 12 56 18 56 26 C56 38 46 46 32 56 Z"
                  />
                  <path
                    className="profile__achievement-heart-bean"
                    d="M32 49 C22 41 14 35 14 26 C14 21 18 17 23 17 C27 17 30 19 32 23 C34 19 37 17 41 17 C46 17 50 21 50 26 C50 35 42 41 32 49 Z"
                  />
                </svg>
              ) : variant === "paw-crown" ? (
                <svg className="profile__achievement-paw profile__achievement-paw--crown" viewBox="0 0 64 64">
                  <path
                    className="profile__achievement-crown"
                    d="M11 24 L14.5 11 L18.5 19.5 L26.5 8.5 L32 15.5 L37.5 8.5 L45.5 19.5 L49.5 11 L53 24 Z"
                  />
                  <rect className="profile__achievement-crown-band" x="11" y="21.5" width="42" height="5.5" rx="1.4" />
                  <circle className="profile__achievement-crown-gem" cx="14.5" cy="12" r="1.5" />
                  <circle className="profile__achievement-crown-gem" cx="26.5" cy="9.5" r="1.5" />
                  <circle className="profile__achievement-crown-gem profile__achievement-crown-gem--main" cx="32" cy="8" r="2" />
                  <circle className="profile__achievement-crown-gem" cx="37.5" cy="9.5" r="1.5" />
                  <circle className="profile__achievement-crown-gem" cx="49.5" cy="12" r="1.5" />
                  <path
                    className="profile__achievement-pad-shell"
                    d="M32 34 C22 34 16 41 16 50 C16 57 22 61 32 61 C42 61 48 57 48 50 C48 41 42 34 32 34 Z"
                  />
                  <path
                    className="profile__achievement-pad-bean"
                    d="M32 39 C25 39 21 44 21 50 C21 55 25 58 32 58 C39 58 43 55 43 50 C43 44 39 39 32 39 Z"
                  />
                </svg>
              ) : (
                <svg className="profile__achievement-paw" viewBox="0 0 64 64">
                  <circle className="profile__achievement-bg" cx="32" cy="32" r="28" />
                  <path className="profile__achievement-claw" d="M15 14 L18 6 L21 14 Z" />
                  <path className="profile__achievement-claw" d="M27 10 L30 2 L33 10 Z" />
                  <path className="profile__achievement-claw" d="M39 10 L42 2 L45 10 Z" />
                  <path className="profile__achievement-claw" d="M51 14 L54 6 L57 14 Z" />

                  <ellipse className="profile__achievement-toe-shell" cx="15.5" cy="26" rx="8" ry="9.5" transform="rotate(-16 15.5 26)" />
                  <ellipse className="profile__achievement-toe-shell" cx="27.5" cy="19.5" rx="8.6" ry="10.5" transform="rotate(-6 27.5 19.5)" />
                  <ellipse className="profile__achievement-toe-shell" cx="40.5" cy="19.5" rx="8.6" ry="10.5" transform="rotate(6 40.5 19.5)" />
                  <ellipse className="profile__achievement-toe-shell" cx="52.5" cy="26" rx="8" ry="9.5" transform="rotate(16 52.5 26)" />

                  <ellipse className="profile__achievement-toe-bean" cx="15.5" cy="27" rx="4.7" ry="5.9" transform="rotate(-16 15.5 27)" />
                  <ellipse className="profile__achievement-toe-bean" cx="27.5" cy="20.5" rx="4.9" ry="6.3" transform="rotate(-6 27.5 20.5)" />
                  <ellipse className="profile__achievement-toe-bean" cx="40.5" cy="20.5" rx="4.9" ry="6.3" transform="rotate(6 40.5 20.5)" />
                  <ellipse className="profile__achievement-toe-bean" cx="52.5" cy="27" rx="4.7" ry="5.9" transform="rotate(16 52.5 27)" />

                  <path
                    className="profile__achievement-pad-shell"
                    d="M32 34 C22 34 16 41 16 50 C16 57 22 61 32 61 C42 61 48 57 48 50 C48 41 42 34 32 34 Z"
                  />
                  <path
                    className="profile__achievement-pad-bean"
                    d="M32 39 C25 39 21 44 21 50 C21 55 25 58 32 58 C39 58 43 55 43 50 C43 44 39 39 32 39 Z"
                  />
                </svg>
              )}
              <span
                className={`profile__achievement-days${days >= 100 ? " profile__achievement-days--triple" : days >= 10 ? " profile__achievement-days--double" : ""}${variant === "heart" ? " profile__achievement-days--heart" : ""}`}
              >
                {days}
              </span>
            </div>
            <div className="profile__achievement-label">стрик {days}</div>
          </div>
        ))}
        </div>
        <div className="profile__achievements-group">Ачивки за тренировки</div>
        <div className="profile__achievements-strip">
        {WORKOUT_ACHIEVEMENTS.map(({ count, variant }, i) => (
          <div
            key={count}
            className={`profile__achievement profile__wach profile__wach--t${i + 1}${i < workoutAchEarned ? " is-earned" : ""}`}
          >
            <div className="profile__achievement-badge" aria-hidden>
              <svg className="profile__achievement-star-svg" viewBox="0 0 64 64">
                <circle className="profile__achievement-bg" cx="32" cy="32" r="28" />
                {/* пятиконечная звезда — корпус ачивки */}
                <path
                  className="profile__achievement-star"
                  d="M32 2 L40.23 20.67 L60.53 22.73 L45.32 36.33 L49.63 56.27 L32 46 L14.37 56.27 L18.68 36.33 L3.47 22.73 L23.77 20.67 Z"
                />
                {variant === "heart" ? (
                  /* маленькое сердце внутри звезды (как у стрика на 42) */
                  <g className="profile__achievement-miniheart" transform="translate(32 31) scale(0.46) translate(-32 -34)">
                    <path
                      className="profile__achievement-heart-shell"
                      d="M32 56 C18 46 8 38 8 26 C8 18 14 12 22 12 C26 12 30 14 32 18 C34 14 38 12 42 12 C50 12 56 18 56 26 C56 38 46 46 32 56 Z"
                    />
                    <path
                      className="profile__achievement-heart-bean"
                      d="M32 49 C22 41 14 35 14 26 C14 21 18 17 23 17 C27 17 30 19 32 23 C34 19 37 17 41 17 C46 17 50 21 50 26 C50 35 42 41 32 49 Z"
                    />
                  </g>
                ) : (
                  /* маленькая лапка внутри звезды — число «на подушечке» */
                  <g className="profile__achievement-minipaw" transform="translate(32 31.5) scale(0.5) translate(-32 -34)">
                    <ellipse className="profile__achievement-toe-shell" cx="15.5" cy="26" rx="8" ry="9.5" transform="rotate(-16 15.5 26)" />
                    <ellipse className="profile__achievement-toe-shell" cx="27.5" cy="19.5" rx="8.6" ry="10.5" transform="rotate(-6 27.5 19.5)" />
                    <ellipse className="profile__achievement-toe-shell" cx="40.5" cy="19.5" rx="8.6" ry="10.5" transform="rotate(6 40.5 19.5)" />
                    <ellipse className="profile__achievement-toe-shell" cx="52.5" cy="26" rx="8" ry="9.5" transform="rotate(16 52.5 26)" />

                    <ellipse className="profile__achievement-toe-bean" cx="15.5" cy="27" rx="4.7" ry="5.9" transform="rotate(-16 15.5 27)" />
                    <ellipse className="profile__achievement-toe-bean" cx="27.5" cy="20.5" rx="4.9" ry="6.3" transform="rotate(-6 27.5 20.5)" />
                    <ellipse className="profile__achievement-toe-bean" cx="40.5" cy="20.5" rx="4.9" ry="6.3" transform="rotate(6 40.5 20.5)" />
                    <ellipse className="profile__achievement-toe-bean" cx="52.5" cy="27" rx="4.7" ry="5.9" transform="rotate(16 52.5 27)" />

                    <path
                      className="profile__achievement-pad-shell"
                      d="M32 34 C22 34 16 41 16 50 C16 57 22 61 32 61 C42 61 48 57 48 50 C48 41 42 34 32 34 Z"
                    />
                    <path
                      className="profile__achievement-pad-bean"
                      d="M32 39 C25 39 21 44 21 50 C21 55 25 58 32 58 C39 58 43 55 43 50 C43 44 39 39 32 39 Z"
                    />
                  </g>
                )}
              </svg>
              <span
                className={`profile__achievement-num${variant === "heart" ? " profile__achievement-num--heart" : ""}${count >= 1000 ? " profile__achievement-num--quad" : count >= 100 ? " profile__achievement-num--triple" : count >= 10 ? " profile__achievement-num--double" : ""}`}
              >
                {count}
              </span>
            </div>
            <div className="profile__achievement-label">{count} {workoutsWordRu(count)}</div>
          </div>
        ))}
        </div>
      </section>

      <h2 className="section-title">Спасение стрика</h2>
      <div className="profile__streak-save">
        <button
          type="button"
          className="profile__save profile__streak-save-btn"
          onClick={() => void useSaveStreak()}
          disabled={saveStreakBusy || saveStreakAvail <= 0}
        >
          {saveStreakBusy ? "Отправляю…" : "Спасти стрик 🛡️"}
        </button>
        <span
          className="profile__streak-save-counter"
          title={`Доступно попыток: ${saveStreakAvail} из ${saveStreakMax}`}
        >
          осталось {saveStreakAvail}/{saveStreakMax}
        </span>
      </div>

      <h2 className="section-title">Здоровье</h2>
      {onSick === null ? (
        <p className="profile__hint muted">Загрузка статуса…</p>
      ) : onSick ? (
        <div className="profile__health">
          <p className="profile__hint">🏥 Ты на больничном — таймер остановлен. Возвращайся, когда поправишься.</p>
          <button
            type="button"
            className="profile__save profile__health-btn"
            onClick={() => void submitHealthy()}
            disabled={healthBusy}
          >
            {healthBusy ? "Отправляю…" : "Выйти с больничного"}
          </button>
        </div>
      ) : sickFormOpen ? (
        <div className="profile__health">
          <p className="profile__hint muted">Опиши, что случилось — Лео решит, принимать ли больничный.</p>
          <textarea
            ref={healthTextareaRef}
            className="profile__input profile__health-textarea"
            value={sickReason}
            onChange={(e) => setSickReason(e.target.value.slice(0, 500))}
            placeholder="Например: температура 38, кашель"
            rows={3}
            maxLength={500}
            disabled={healthBusy}
            onFocus={() => {
              setHealthInputFocused(true);
              window.setTimeout(() => scrollHealthAboveKeyboard(), 80);
            }}
            onBlur={() => {
              window.setTimeout(() => {
                const el = document.activeElement;
                if (el instanceof HTMLElement && el.closest(".profile__health-actions")) return;
                setHealthInputFocused(false);
              }, 80);
            }}
          />
          {healthActionsKeyboard ? <div className="profile__health-actions-spacer" aria-hidden /> : null}
          <div
            className={`profile__health-actions${healthActionsKeyboard ? " profile__health-actions--keyboard" : ""}`}
          >
            <button
              type="button"
              className="profile__save profile__health-btn"
              onClick={() => void submitSickLeave()}
              disabled={healthBusy || sickReason.trim().length < 3}
            >
              {healthBusy ? "Отправляю…" : "Отправить заявку"}
            </button>
            <button
              type="button"
              className="profile__health-cancel"
              onClick={() => {
                setSickFormOpen(false);
                setSickReason("");
                setHealthInputFocused(false);
              }}
              disabled={healthBusy}
            >
              Отмена
            </button>
          </div>
        </div>
      ) : (
        <div className="profile__health">
          <p className="profile__hint muted">Заболел — таймер остановится до выздоровления.</p>
          <div className="profile__health-actions">
            <button
              type="button"
              className="profile__save profile__health-btn profile__health-btn--secondary"
              onClick={() => setSickFormOpen(true)}
              disabled={healthBusy}
            >
              Взять больничный
            </button>
          </div>
        </div>
      )}

      <h2 className="section-title">Профиль (для Лео)</h2>
      {profileLoading && <p className="muted">Загрузка профиля…</p>}
      <div className="profile__form">
        <label className="profile__field">
          <span>Имя (как обращаться)</span>
          <input
            type="text"
            className="profile__input"
            value={profile.displayName}
            onChange={(e) => setProfile((p) => ({ ...p, displayName: e.target.value.slice(0, 64) }))}
            placeholder="Например, Саша"
            maxLength={64}
            autoComplete="name"
            disabled={profileLoading}
          />
        </label>
        <label className="profile__field">
          <span>Пол</span>
          <select
            className="profile__input"
            value={profile.gender}
            onChange={(e) => setProfile((p) => ({ ...p, gender: e.target.value as "m" | "f" | "" }))}
            disabled={profileLoading}
          >
            <option value="">Не указывать</option>
            <option value="m">Мужской</option>
            <option value="f">Женский</option>
          </select>
        </label>
        {(profileDirty || profileSaving) && (
          <button
            type="button"
            className="profile__save"
            onClick={() => void saveProfile()}
            disabled={profileLoading || profileSaving || !profileDirty}
          >
            {profileSaving ? "Сохраняю…" : "Сохранить"}
          </button>
        )}
      </div>

      <div className="profile__support">
        <h2 className="section-title">Поддержка</h2>
        <p className="profile__hint muted">Если что-то сломалось или нужен ответ от человека — напиши сюда.</p>
        <button type="button" className="profile__save profile__support-btn" onClick={onSupport}>
          Поддержка
        </button>
      </div>

    </div>
  );
}
