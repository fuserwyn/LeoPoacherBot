import { useCallback, useEffect, useState } from "react";
import { miniappCupsLevelProgress, miniappLevelFromCups, miniappLevelName } from "../lib/miniappLevel";
import { cupsWordRu, daysWordRu } from "../lib/streakLabel";
import "./ProfileScreen.css";

const api = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type ProfileData = {
  gender: "m" | "f" | "";
  displayName: string;
  timezoneOffset: number;
};

const TZ_OPTIONS: number[] = Array.from({ length: 25 }, (_, i) => i - 12);

const formatTzLabel = (offset: number): string => {
  if (offset === 0) return "МСК (Москва, +0)";
  const sign = offset > 0 ? "+" : "−";
  return `МСК ${sign}${Math.abs(offset)} ч`;
};

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
  initData: string;
  inTelegram: boolean;
  /** Ссылка на аватар из Telegram WebApp (initDataUnsafe.user.photo_url), если бот открыл мини-апп. */
  userPhotoUrl?: string;
  showAlert: (m: string) => void;
};

const ACHIEVEMENTS = [
  { days: 7, colorClass: "profile__achievement--7" },
  { days: 14, colorClass: "profile__achievement--14" },
  { days: 21, colorClass: "profile__achievement--21" },
  { days: 30, colorClass: "profile__achievement--30" },
  { days: 42, colorClass: "profile__achievement--42" },
  { days: 50, colorClass: "profile__achievement--50" },
  { days: 100, colorClass: "profile__achievement--100" },
] as const;

export function ProfileScreen({
  name,
  streak,
  recordStreak,
  xp,
  achievementCount,
  achievementsMax,
  workouts,
  daysSinceLastTraining,
  initData,
  inTelegram,
  userPhotoUrl,
  showAlert,
}: Props) {
  const [profile, setProfile] = useState<ProfileData>({ gender: "", displayName: "", timezoneOffset: 0 });
  const [profileLoading, setProfileLoading] = useState(true);
  const [profileSaving, setProfileSaving] = useState(false);

  const [onSick, setOnSick] = useState<boolean | null>(null);
  const [sickFormOpen, setSickFormOpen] = useState(false);
  const [sickReason, setSickReason] = useState("");
  const [healthBusy, setHealthBusy] = useState(false);

  const cupProgress = miniappCupsLevelProgress(xp);
  const levelTitle = miniappLevelName(miniappLevelFromCups(xp)) || "—";
  const barPct = Math.min(100, (cupProgress.cupsInSegment / cupProgress.cupsToNext) * 100);

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
      setProfile({
        gender: g,
        displayName: dn,
        timezoneOffset: tz,
      });
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
    } finally {
      setProfileLoading(false);
    }
  }, [inTelegram, initData, name, showAlert]);

  useEffect(() => {
    void load();
  }, [load]);

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
    void loadHealth();
  }, [loadHealth]);

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
      showAlert("Отправлено. Лео подтвердит во вкладке Чат.");
      setTimeout(() => {
        void loadHealth();
        void load();
      }, 1200);
    }
  }, [sendHealthMessage, loadHealth, load, showAlert]);

  const saveProfile = useCallback(async () => {
    if (!api || !inTelegram || !initData?.trim()) {
      showAlert("Открой мини-апп из Telegram (нужен initData).");
      return;
    }
    setProfileSaving(true);
    try {
      const body: Record<string, unknown> = {
        init_data: initData,
        gender: profile.gender,
        display_name: profile.displayName.trim(),
        timezone_offset: profile.timezoneOffset,
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
        void window.Telegram?.WebApp?.HapticFeedback?.notificationOccurred?.("success");
        showAlert("Сохранено. Лео подстроит обращения.");
      }
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
    } finally {
      setProfileSaving(false);
    }
  }, [inTelegram, initData, profile, showAlert]);

  const noTrainingAlert = daysSinceLastTraining >= 5;
  const noTrainingDanger = daysSinceLastTraining >= 7;

  return (
    <div className="profile">
      <header className="profile__hero">
        <div className={`profile__avatar${noTrainingAlert ? " profile__avatar--alert" : ""}`} aria-hidden>
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
            Уровень {miniappLevelFromCups(xp)} · {levelTitle}
          </p>
          {daysSinceLastTraining > 0 ? (
            <p
              className={`profile__kick${noTrainingAlert ? " profile__kick--alert" : ""}${noTrainingDanger ? " profile__kick--danger" : ""}`}
              title={`Дней без тренировок: ${daysSinceLastTraining}`}
            >
              Дней без тренировок: {daysSinceLastTraining}
            </p>
          ) : null}
          {noTrainingDanger ? (
            <p className="profile__danger">ПОТРЕНИРУЙСЯ ДО 00:00 ИЛИ ЛЕО СЪЕСТ ТЕБЯ</p>
          ) : null}
        </div>
        <div
          className="profile__xp"
          aria-label={`Кубки: ${xp} ${cupsWordRu(xp)}, до следующего уровня ${cupProgress.cupsToNext - cupProgress.cupsInSegment}`}
        >
          <div className="profile__xp-meter">
            <span className="profile__xp-caption" aria-hidden>
              🏆
            </span>
            <div className="profile__xp-bar">
              <div className="profile__xp-fill" style={{ width: `${barPct}%` }} />
            </div>
          </div>
          <span className="profile__xp-txt">
            {xp} {cupsWordRu(xp)} / {cupProgress.cupsToNext}
          </span>
        </div>
      </header>

      <div className="profile__grid3">
        <div className="stat-card">
          <div className="stat-card__label">Дней подряд</div>
          <div className="stat-card__val">{streak}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card__label">Рекорд стрика</div>
          <div className="stat-card__val">{recordStreak}</div>
        </div>
        <div className="stat-card">
          <div className="stat-card__label">Всего тренировок</div>
          <div className="stat-card__val">{workouts}</div>
        </div>
      </div>

      <section className="profile__achievements" aria-label="Достижения">
        <div className="profile__achievements-head">
          <h2 className="section-title profile__achievements-title">Ачивки</h2>
          <span className="profile__achievements-count">
            {achievementCount}/{achievementsMax}
          </span>
        </div>
        <div className="profile__achievements-strip">
        {ACHIEVEMENTS.map(({ days, colorClass }, i) => (
          <div key={days} className={`profile__achievement ${colorClass}${i < achievementCount ? " is-earned" : ""}`}>
            <div className="profile__achievement-badge" aria-hidden>
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
              <span
                className={`profile__achievement-days${days >= 100 ? " profile__achievement-days--triple" : days >= 10 ? " profile__achievement-days--double" : ""}`}
              >
                {days}
              </span>
            </div>
            <div className="profile__achievement-label">
              {days} {daysWordRu(days)}
            </div>
          </div>
        ))}
        </div>
      </section>

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
            className="profile__input profile__health-textarea"
            value={sickReason}
            onChange={(e) => setSickReason(e.target.value.slice(0, 500))}
            placeholder="Например: температура 38, кашель"
            rows={3}
            maxLength={500}
            disabled={healthBusy}
          />
          <div className="profile__health-actions">
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
        <label className="profile__field">
          <span>Часовой пояс</span>
          <select
            className="profile__input"
            value={String(profile.timezoneOffset)}
            onChange={(e) =>
              setProfile((p) => ({
                ...p,
                timezoneOffset: Math.max(-12, Math.min(12, parseInt(e.target.value, 10) || 0)),
              }))
            }
            disabled={profileLoading}
          >
            {TZ_OPTIONS.map((o) => (
              <option key={o} value={String(o)}>
                {formatTzLabel(o)}
              </option>
            ))}
          </select>
        </label>
        <p className="profile__form-hint muted">
          По нему считаются «сегодня/вчера» для тренировок и дней подряд. Если живёшь в Москве — оставь «МСК (+0)».
        </p>
        <button
          type="button"
          className="profile__save"
          onClick={() => void saveProfile()}
          disabled={profileLoading || profileSaving}
        >
          {profileSaving ? "Сохраняю…" : "Сохранить"}
        </button>
      </div>

    </div>
  );
}
