import { useCallback, useEffect, useState } from "react";
import { useTelegramWebApp } from "./hooks/useTelegramWebApp";
import { BottomNav } from "./components/BottomNav";
import { ChatScreen } from "./components/ChatScreen";
import { FeedScreen } from "./components/FeedScreen";
import { ProfileScreen } from "./components/ProfileScreen";
import { NewWorkoutScreen } from "./components/NewWorkoutScreen";
import { RulesScreen } from "./components/RulesScreen";
import { sendMiniappPrivateText, sendMiniappTrainingWithPhoto } from "./lib/miniappPrivateSend";
import { fetchLeoPendingCount } from "./lib/leoPersonalInbox";
import { clearFeedThreadUnread, fetchFeedThreadUnreadCount } from "./lib/feedThreadUnread";
import { miniappLevelFromXp } from "./lib/miniappLevel";
import { ensureMiniappOnboarding } from "./lib/miniappOnboarding";
import "./App.css";

type Tab = "chat" | "feed" | "rules" | "profile";

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
  const [achievementsMax, setAchievementsMax] = useState(4);
  const [workouts, setWorkouts] = useState(0);
  const [workoutsWeek, setWorkoutsWeek] = useState(0);
  const [leoPending, setLeoPending] = useState(0);
  const [feedUnread, setFeedUnread] = useState(0);
  const [feedRefreshToken, setFeedRefreshToken] = useState(0);

  const refreshTabBadges = useCallback(async () => {
    if (!inTelegram || !initData?.trim()) {
      setLeoPending(0);
      setFeedUnread(0);
      return;
    }
    const [leo, feed] = await Promise.all([fetchLeoPendingCount(initData), fetchFeedThreadUnreadCount(initData)]);
    setLeoPending(leo);
    setFeedUnread(feed);
  }, [inTelegram, initData]);

  const refreshProfileStats = useCallback(async () => {
    const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";
    if (!inTelegram || !initData?.trim() || !apiBase) return;
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
      };
      if (!res.ok || !j.ok) return;
      setStreak(typeof j.streak_days === "number" ? j.streak_days : 0);
      setRecordStreak(typeof j.max_streak_days === "number" ? j.max_streak_days : 0);
      setXP(typeof j.xp === "number" ? j.xp : 0);
      setAchievementCount(typeof j.achievement_count === "number" ? j.achievement_count : 0);
      setAchievementsMax(typeof j.achievements_max === "number" ? j.achievements_max : 4);
      setWorkouts(typeof j.workouts_total === "number" ? j.workouts_total : 0);
      setWorkoutsWeek(typeof j.workouts_week === "number" ? j.workouts_week : 0);
    } catch {
      return;
    }
  }, [inTelegram, initData]);

  useEffect(() => {
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
  }, [refreshTabBadges]);

  useEffect(() => {
    void refreshProfileStats();
  }, [refreshProfileStats]);

  useEffect(() => {
    if (tab !== "feed" || !inTelegram || !initData?.trim()) return;
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
  }, [tab, inTelegram, initData]);

  useEffect(() => {
    if (!inTelegram || !initData?.trim()) return;
    void ensureMiniappOnboarding(initData);
  }, [inTelegram, initData]);

  const onLeoInboxDrained = useCallback(() => {
    void refreshTabBadges();
  }, [refreshTabBadges]);

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
          level={miniappLevelFromXp(xp)}
          streak={streak}
          userId={userId}
          initData={initData}
          inTelegram={inTelegram}
          showAlert={showAlert}
          refreshToken={feedRefreshToken}
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
          workoutsWeek={workoutsWeek}
          showAlert={showAlert}
        />
      )}

      <BottomNav
        active={tab}
        leoBadgeCount={leoPending}
        feedBadgeCount={feedUnread}
        onChat={() => setTab("chat")}
        onFeed={() => setTab("feed")}
        onAddWorkout={() => setWorkoutOpen(true)}
        onRules={() => setTab("rules")}
        onProfile={() => setTab("profile")}
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
              other: "другое",
            };
            let kind = labels[type] ?? type;
            if (type === "other" && otherLabel?.trim()) {
              kind = otherLabel.trim();
            }
            const base = `#training_done — ${kind}, ${min} мин, инт. ${intensity}/5`;
            const line = note ? `${base}\n\n${note}` : base;
            tg?.HapticFeedback?.impactOccurred?.("medium");
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
            const msg = result.replyParts.filter(Boolean).join("\n\n").trim() || "Отчёт отправлен.";
            showAlert(msg.length > 350 ? `${msg.slice(0, 347)}…` : msg);
            return true;
          }}
        />
      )}
    </div>
  );
}
