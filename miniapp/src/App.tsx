import { useCallback, useEffect, useRef, useState } from "react";
import { useTelegramWebApp } from "./hooks/useTelegramWebApp";
import { BottomNav } from "./components/BottomNav";
import { ChatScreen } from "./components/ChatScreen";
import { FeedScreen } from "./components/FeedScreen";
import { ProfileScreen } from "./components/ProfileScreen";
import { NewWorkoutScreen } from "./components/NewWorkoutScreen";
import { RulesScreen } from "./components/RulesScreen";
import { MiniappRemovedScreen } from "./components/MiniappRemovedScreen";
import { sendMiniappPrivateText, sendMiniappTrainingWithPhoto } from "./lib/miniappPrivateSend";
import { fetchLeoPendingCount } from "./lib/leoPersonalInbox";
import { clearFeedThreadUnread, fetchFeedThreadUnreadCount } from "./lib/feedThreadUnread";
import { ensureMiniappOnboarding } from "./lib/miniappOnboarding";
import { syncTimezoneIfNeeded } from "./lib/timezoneSync";
import "./App.css";

type Tab = "chat" | "feed" | "rules" | "profile";
type AccessGateStatus = "checking" | "ok" | "deleted";

function formatTrainingDoneAlert(replyParts: string[]): string {
  const summary = replyParts.filter(Boolean).join("\n\n").trim();
  if (!summary.startsWith("✅ Отчёт принят")) return summary;

  const lines = summary
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);

  const title = lines.find((line) => line.startsWith("✅ Отчёт принят")) ?? "";
  const streakLineRaw = lines.find((line) => /Серия:/i.test(line)) ?? "";
  const cupsLineRaw = lines.find((line) => line.startsWith("🏆")) ?? "";
  const streakLine = streakLineRaw.replace(/^🦁\s*Серия:\s*/i, "Стрик: ");
  const cupsLine = cupsLineRaw.replace(/^🏆\s*/u, "");

  const compact = [title, [streakLine, cupsLine].filter(Boolean).join("\n")].filter(Boolean).join("\n\n").trim();
  return compact || summary;
}

export function App() {
  const { name, streak: hookStreak, initData, userId, photoUrl, inTelegram, tg } = useTelegramWebApp();
  const showAlert = useCallback((m: string) => {
    if (tg?.showAlert) void tg.showAlert(m);
    else window.alert(m);
  }, [tg]);
  const [tab, setTab] = useState<Tab>("feed");
  const [workoutOpen, setWorkoutOpen] = useState(false);
  const [streak, setStreak] = useState(hookStreak);
  const [recordStreak, setRecordStreak] = useState(hookStreak);
  const [xp, setXP] = useState(0);
  const [achievementCount, setAchievementCount] = useState(0);
  const [achievementsMax, setAchievementsMax] = useState(7);
  const [workouts, setWorkouts] = useState(0);
  const [leoPending, setLeoPending] = useState(0);
  const [feedUnread, setFeedUnread] = useState(0);
  const [feedRefreshToken, setFeedRefreshToken] = useState(0);
  const [daysSinceLastTraining, setDaysSinceLastTraining] = useState<number>(-1);
  const [accessGateStatus, setAccessGateStatus] = useState<AccessGateStatus>("checking");
  const tzSyncedRef = useRef(false);

  const refreshAccessStatus = useCallback(async () => {
    if (!inTelegram || !initData?.trim()) {
      setAccessGateStatus("ok");
      return;
    }
    const res = await ensureMiniappOnboarding(initData);
    setAccessGateStatus(res.deleted || res.accessState === "deleted" ? "deleted" : "ok");
  }, [inTelegram, initData]);

  const refreshTabBadges = useCallback(async () => {
    if (accessGateStatus !== "ok" || !inTelegram || !initData?.trim()) {
      setLeoPending(0);
      setFeedUnread(0);
      return;
    }
    const [leo, feed] = await Promise.all([fetchLeoPendingCount(initData), fetchFeedThreadUnreadCount(initData)]);
    setLeoPending(leo);
    setFeedUnread(feed);
  }, [accessGateStatus, inTelegram, initData]);

  const refreshProfileStats = useCallback(async () => {
    const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";
    if (accessGateStatus !== "ok" || !inTelegram || !initData?.trim() || !apiBase) return;
    try {
      const res = await fetch(`${apiBase}/api/miniapp/profile/load`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData }),
      });
      const j = (await res.json().catch(() => ({}))) as {
        ok?: boolean;
        streak_days?: number;
        max_streak_days?: number;
        xp?: number;
        achievement_count?: number;
        achievements_max?: number;
        workouts_total?: number;
        workouts_week?: number;
        inactivity_removal_at?: string;
        timezone_offset?: number;
        days_since_last_training?: number;
      };
      if (!res.ok || !j.ok) return;
      setDaysSinceLastTraining(typeof j.days_since_last_training === "number" ? j.days_since_last_training : -1);
      setStreak(typeof j.streak_days === "number" ? j.streak_days : 0);
      setRecordStreak(typeof j.max_streak_days === "number" ? j.max_streak_days : 0);
      setXP(typeof j.xp === "number" ? j.xp : 0);
      setAchievementCount(typeof j.achievement_count === "number" ? j.achievement_count : 0);
      setAchievementsMax(typeof j.achievements_max === "number" ? j.achievements_max : 7);
      setWorkouts(typeof j.workouts_total === "number" ? j.workouts_total : 0);
      // Одноразовое автоопределение TZ для пользователей без явно выставленного смещения.
      if (!tzSyncedRef.current) {
        tzSyncedRef.current = true;
        const currentOffset = typeof j.timezone_offset === "number" ? j.timezone_offset : 0;
        void syncTimezoneIfNeeded(initData, currentOffset);
      }
    } catch {
      return;
    }
  }, [accessGateStatus, inTelegram, initData]);

  useEffect(() => {
    void refreshAccessStatus();
  }, [refreshAccessStatus]);

  useEffect(() => {
    const t = window.setInterval(() => {
      void refreshAccessStatus();
    }, 30_000);
    const onVis = () => {
      if (document.visibilityState === "visible") void refreshAccessStatus();
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(t);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [refreshAccessStatus]);

  useEffect(() => {
    if (accessGateStatus !== "ok") return;
    void refreshTabBadges();
    const t = window.setInterval(() => {
      void refreshTabBadges();
    }, 30_000);
    const onVis = () => {
      if (document.visibilityState === "visible") void refreshTabBadges();
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(t);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [accessGateStatus, refreshTabBadges]);

  useEffect(() => {
    if (accessGateStatus !== "ok") return;
    void refreshProfileStats();
  }, [accessGateStatus, refreshProfileStats]);

  useEffect(() => {
    if (accessGateStatus !== "ok" || tab !== "feed" || !inTelegram || !initData?.trim()) return;
    let cancelled = false;
    void (async () => {
      await clearFeedThreadUnread(initData);
      if (!cancelled) {
        const n = await fetchFeedThreadUnreadCount(initData);
        if (!cancelled) setFeedUnread(n);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [accessGateStatus, tab, inTelegram, initData]);

  const onLeoInboxDrained = useCallback(() => {
    void refreshTabBadges();
  }, [refreshTabBadges]);

  if (accessGateStatus === "checking") {
    return <div className="app" />;
  }

  if (accessGateStatus === "deleted") {
    return (
      <div className="app">
        <MiniappRemovedScreen />
      </div>
    );
  }

  return (
    <div className="app">
      {tab === "chat" && (
        <ChatScreen
          name={name}
          initData={initData}
          inTelegram={inTelegram}
          showAlert={showAlert}
          onInboxDrained={onLeoInboxDrained}
        />
      )}
      {tab === "feed" && (
        <FeedScreen
          name={name}
          streak={streak}
          userId={userId}
          initData={initData}
          inTelegram={inTelegram}
          showAlert={showAlert}
          refreshToken={feedRefreshToken}
          onRefreshAll={async () => {
            await Promise.all([refreshProfileStats(), refreshTabBadges()]);
          }}
        />
      )}
      {tab === "rules" && <RulesScreen />}
      {tab === "profile" && (
        <ProfileScreen
          name={name}
          streak={streak}
          workouts={workouts}
          initData={initData}
          inTelegram={inTelegram}
          userPhotoUrl={photoUrl}
          xp={xp}
          recordStreak={recordStreak}
          achievementCount={achievementCount}
          achievementsMax={achievementsMax}
          daysSinceLastTraining={daysSinceLastTraining}
          showAlert={showAlert}
          onSupport={() => {
            setWorkoutOpen(false);
            setTab("chat");
          }}
        />
      )}

      <BottomNav
        active={tab}
        leoBadgeCount={leoPending}
        feedBadgeCount={feedUnread}
        onChat={() => {
          setWorkoutOpen(false);
          setTab("chat");
        }}
        onFeed={() => {
          setWorkoutOpen(false);
          setTab("feed");
        }}
        onAddWorkout={() => setWorkoutOpen(true)}
        onRules={() => {
          setWorkoutOpen(false);
          setTab("rules");
        }}
        onProfile={() => {
          setWorkoutOpen(false);
          setTab("profile");
        }}
      />

      {workoutOpen && (
        <NewWorkoutScreen
          showAlert={showAlert}
          onClose={() => setWorkoutOpen(false)}
          onSave={async ({ type, min, intensity, note, photo, otherLabel }) => {
            if (!inTelegram || !initData) {
              showAlert("Открой мини-апп из Telegram (нужен initData).");
              return false;
            }
            const labels: Record<string, string> = {
              run: "бег",
              walk: "ходьба",
              bike: "велосипед",
              swim: "плавание",
              yoga: "йога",
              rowing: "гребля",
              workout: "воркаут",
              crossfit: "кроссфит",
              stretch: "растяжка",
              dance: "танцы",
              hiit: "hiit",
              cardio: "кардио",
              kettlebell: "гиря",
              strength: "силовая",
              jump_rope: "скакалка",
              other: "другое",
            };
            let kind = labels[type] ?? type;
            if (type === "other" && otherLabel?.trim()) {
              kind = otherLabel.trim();
            }
            const base = `#training_done — ${kind}, ${min} мин, инт. ${intensity}/5`;
            const line = note ? `${base}\n\n${note}` : base;
            tg?.HapticFeedback?.impactOccurred?.("medium");
            // #training_done: сервер отдаёт reply_text сразу (серия, кубки, ачивки; без блока про неактивность).
            const result = photo
              ? await sendMiniappTrainingWithPhoto(initData, line, photo)
              : await sendMiniappPrivateText(initData, line);
            if (!result.ok) {
              showAlert(result.error);
              return false;
            }
            void refreshTabBadges();
            void refreshProfileStats();
            setTab("feed");
            setFeedRefreshToken((v) => v + 1);
            window.setTimeout(() => setFeedRefreshToken((v) => v + 1), 4000);
            window.setTimeout(() => setFeedRefreshToken((v) => v + 1), 10000);
            window.setTimeout(() => setFeedRefreshToken((v) => v + 1), 20000);
            window.setTimeout(() => void refreshTabBadges(), 6000);
            const summary = formatTrainingDoneAlert(result.replyParts);
            const fallback =
              "Отчёт принят. Комментарий Лео скоро появится в ленте.";
            const msg = summary.length > 0 ? summary : fallback;
            showAlert(msg.length > 400 ? `${msg.slice(0, 397)}…` : msg);
            return true;
          }}
        />
      )}
    </div>
  );
}
