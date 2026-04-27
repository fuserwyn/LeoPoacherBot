import { useCallback, useEffect, useState } from "react";
import "./ProfileScreen.css";

const api = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type ProfileData = {
  gender: "m" | "f" | "";
  displayName: string;
  age: string;
};

type Props = {
  name: string;
  streak: number;
  workouts: number;
  initData: string;
  inTelegram: boolean;
  showAlert: (m: string) => void;
};

const LEVELS = [200];

export function ProfileScreen({ name, streak, workouts, initData, inTelegram, showAlert }: Props) {
  const xp = 25;
  const [burn, setBurn] = useState<3 | 5 | 7>(5);
  const [profile, setProfile] = useState<ProfileData>({ gender: "", displayName: "", age: "" });
  const [initialAgeSet, setInitialAgeSet] = useState(false);
  const [profileLoading, setProfileLoading] = useState(true);
  const [profileSaving, setProfileSaving] = useState(false);

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
      };
      if (!res.ok) {
        showAlert(j.error ?? `Профиль: ошибка ${res.status}`);
        return;
      }
      if (!j.ok) return;
      const g = j.gender === "m" || j.gender === "f" ? j.gender : "";
      const dn = (j.display_name ?? "").trim() || (name && name !== "друг" ? name : "");
      setProfile({
        gender: g,
        displayName: dn,
        age: j.age != null && j.age > 0 ? String(j.age) : "",
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
          🐆
        </div>
        <h1 className="profile__name">{(profile.displayName || name).trim() || "Стая"}</h1>
        <p className="profile__level muted">Уровень 1 · Новичок</p>
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

      <h2 className="section-title">Профиль (для Лео)</h2>
      <p className="profile__form-hint muted">Всё по желанию. Пол помогает говорить «качал/качала» и т.п. без путаницы.</p>
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
            placeholder="—"
            disabled={profileLoading}
          />
        </label>
        <button
          type="button"
          className="profile__save"
          onClick={() => void saveProfile()}
          disabled={profileLoading || profileSaving}
        >
          {profileSaving ? "Сохраняю…" : "Сохранить"}
        </button>
      </div>

      <div className="profile__grid3">
        <div className="stat-card">
          <div className="stat-card__label">СТРИК</div>
          <div className="stat-card__val">
            <span className="stat-card__streak-ico">🔥</span> {streak}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card__label">РЕКОРД</div>
          <div className="stat-card__val">{streak} д</div>
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
          <div className="wide-card__val">{workouts}</div>
        </div>
        <div className="wide-card">
          <div className="wide-card__label">Средняя интенсивность</div>
          <div className="wide-card__val">{workouts > 0 ? "3.0" : "—"}</div>
        </div>
      </div>

      <h2 className="section-title">Достижения</h2>
      <div className="profile__empty">Пока нет — тренируйся, и они появятся</div>

      <h2 className="section-title">Порог сгорания</h2>
      <p className="profile__hint muted">Текущий: {burn} дн. без тренировки — и стрик сгорает</p>
      <div className="burn-row" role="group" aria-label="Дней до сгорания стрика">
        {([3, 5, 7] as const).map((d) => (
          <button
            key={d}
            type="button"
            className={`burn-btn ${burn === d ? "is-on" : ""}`}
            aria-pressed={burn === d}
            onClick={() => setBurn(d)}
          >
            {d}
          </button>
        ))}
      </div>

      <h2 className="section-title">Заморозка</h2>
      <p className="profile__hint muted">Осталось: 1 из 1 в месяц (Free)</p>
    </div>
  );
}
