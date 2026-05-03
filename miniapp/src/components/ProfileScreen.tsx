import { useCallback, useEffect, useState } from "react";
import { formatStreakDaysRu } from "../lib/streakLabel";
import "./ProfileScreen.css";

const api = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type ProfileData = {
  gender: "m" | "f" | "";
  displayName: string;
  age: string;
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
  workoutsWeek: number;
  initData: string;
  inTelegram: boolean;
  /** Ссылка на аватар из Telegram WebApp (initDataUnsafe.user.photo_url), если бот открыл мини-апп. */
  userPhotoUrl?: string;
  showAlert: (m: string) => void;
};

const LEVELS = [200];
const ACHIEVEMENT_MILESTONES = [7, 14, 21, 28];

export function ProfileScreen({
  name,
  streak,
  recordStreak,
  xp,
  achievementCount,
  achievementsMax,
  workouts,
  workoutsWeek,
  initData,
  inTelegram,
  userPhotoUrl,
  showAlert,
}: Props) {
  const [profile, setProfile] = useState<ProfileData>({ gender: "", displayName: "", age: "", timezoneOffset: 0 });
  const [initialAgeSet, setInitialAgeSet] = useState(false);
  const [profileLoading, setProfileLoading] = useState(true);
  const [profileSaving, setProfileSaving] = useState(false);

  const [onSick, setOnSick] = useState<boolean | null>(null);
  const [sickFormOpen, setSickFormOpen] = useState(false);
  const [sickReason, setSickReason] = useState("");
  const [healthBusy, setHealthBusy] = useState(false);

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
        age?: number | null;
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
        age: j.age != null && j.age > 0 ? String(j.age) : "",
        timezoneOffset: tz,
      });
      setInitialAgeSet(j.age != null && j.age > 0);
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
      setTimeout(() => void loadHealth(), 1200);
    }
  }, [sendHealthMessage, loadHealth, showAlert]);

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
      const a = profile.age.trim();
      if (a === "") {
        if (initialAgeSet) {
          body.age = 0;
        }
      } else {
        const n = parseInt(a, 10);
        if (Number.isFinite(n) && n > 0) {
          body.age = n;
        }
      }
      const res = await fetch(`${api}/api/miniapp/profile/save`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const j = (await res.json().catch(() => ({}))) as { ok?: boolean; error?: string; age?: number | null };
      if (!res.ok) {
        showAlert(j.error ?? `Сохранение: ${res.status}`);
        return;
      }
      if (j.ok) {
        setInitialAgeSet(j.age != null && j.age > 0);
        void window.Telegram?.WebApp?.HapticFeedback?.notificationOccurred?.("success");
        showAlert("Сохранено. Лео подстроит обращения.");
      }
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
    } finally {
      setProfileSaving(false);
    }
  }, [inTelegram, initData, profile, initialAgeSet, showAlert]);

  return (
    <div className="profile">
      <header className="profile__hero">
        <div className="profile__avatar" aria-hidden>
          {userPhotoUrl ? (
            <img src={userPhotoUrl} alt="" className="profile__avatar-img" width={88} height={88} />
          ) : (
            "🐆"
          )}
          {onSick ? <span className="profile__avatar-med">🌡️</span> : null}
        </div>
        <div>
          <h1 className="profile__name">{(profile.displayName || name).trim() || "Стая"}</h1>
          <p className="profile__level muted">Уровень 1 · Новичок</p>
          {onSick ? (
            <button
              type="button"
              className="profile__quick-healthy"
              onClick={() => void submitHealthy()}
              disabled={healthBusy}
            >
              {healthBusy ? "Отправляю…" : "Выйти с больничного"}
            </button>
          ) : null}
        </div>
        <div className="profile__xp">
          <div className="profile__xp-bar">
            <div
              className="profile__xp-fill"
              style={{ width: `${Math.min(100, (xp / LEVELS[0]) * 100)}%` }}
            />
          </div>
          <span className="profile__xp-txt">
            {xp} XP / {LEVELS[0]}
          </span>
        </div>
      </header>

      <div className="profile__grid3">
        <div className="stat-card">
          <div className="stat-card__label">Серия дней</div>
          <div className="stat-card__val">
            <span className="stat-card__streak-ico">🔥</span> {formatStreakDaysRu(streak)}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card__label">РЕКОРД</div>
          <div className="stat-card__val">{recordStreak} д</div>
        </div>
        <div className="stat-card">
          <div className="stat-card__label">ТРЕНИРОВОК</div>
          <div className="stat-card__val">{workouts}</div>
        </div>
      </div>

      <h2 className="section-title">За неделю</h2>
      <div className="profile__grid2">
        <div className="wide-card">
          <div className="wide-card__label">Тренировок</div>
          <div className="wide-card__val">{workoutsWeek}</div>
        </div>
        <div className="wide-card">
          <div className="wide-card__label">Средняя интенсивность</div>
          <div className="wide-card__val">{workouts > 0 ? "3.0" : "—"}</div>
        </div>
      </div>

      <h2 className="section-title">Достижения</h2>
      <div className="profile__empty">
        🏆 {achievementCount}/{achievementsMax}
      </div>
      <div className="profile__grid2">
        {ACHIEVEMENT_MILESTONES.map((d, i) => (
          <div key={d} className="wide-card">
            <div className="wide-card__label">Серия {d} дней</div>
            <div className="wide-card__val">{i < achievementCount ? "✅" : "⏳"}</div>
          </div>
        ))}
      </div>

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
          <span>Возраст</span>
          <input
            type="number"
            className="profile__input"
            inputMode="numeric"
            min={1}
            max={120}
            value={profile.age}
            onChange={(e) => setProfile((p) => ({ ...p, age: e.target.value.replace(/\D/g, "").slice(0, 3) }))}
            disabled={profileLoading}
          />
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
          По нему считаются «сегодня/вчера» для тренировок и стрика. Если живёшь в Москве — оставь «МСК (+0)».
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

      <h2 className="section-title">Заморозка</h2>
      <p className="profile__hint muted">Осталось: 1 из 1 в месяц (Free)</p>
    </div>
  );
}
