import { useCallback, useEffect, useRef, useState } from "react";
import { useAppKeyboardInset } from "./hooks/useAppKeyboardInset";
import { useTelegramWebApp } from "./hooks/useTelegramWebApp";
import { BottomNav } from "./components/BottomNav";
import { ChatScreen } from "./components/ChatScreen";
import { FeedScreen } from "./components/FeedScreen";
import { ProfileScreen } from "./components/ProfileScreen";
import { NewWorkoutScreen } from "./components/NewWorkoutScreen";
import { RulesScreen } from "./components/RulesScreen";
import { TabKeepAlive } from "./components/TabKeepAlive";
import { MiniappRemovedScreen } from "./components/MiniappRemovedScreen";
import { SupportScreen } from "./components/SupportScreen";
import { buildOptimisticTrainingFeedItem, type PackFeedItemDTO } from "./lib/packFeed";
import { sendMiniappPrivateText, sendMiniappTrainingWithPhoto } from "./lib/miniappPrivateSend";
import { fetchLeoPendingCount } from "./lib/leoPersonalInbox";
import { fetchFeedThreadUnreadSummary } from "./lib/feedThreadUnread";
import { fetchPackGroupUnreadCount } from "./lib/packGroupUnread";
import { ensureMiniappOnboarding } from "./lib/miniappOnboarding";
import { syncDeviceTimezone } from "./lib/timezoneSync";
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
  const streakLineRaw = lines.find((line) => /Стрик:|Серия:/i.test(line)) ?? "";
  const cupsLineRaw = lines.find((line) => line.startsWith("🏆")) ?? "";
  const streakLine = streakLineRaw.replace(/^🦁\s*(Стрик|Серия):\s*/i, "Стрик: ");
  const cupsLine = cupsLineRaw.replace(/^🏆\s*/u, "");

  const compact = [title, [streakLine, cupsLine].filter(Boolean).join("\n")].filter(Boolean).join("\n\n").trim();
  return compact || summary;
}

export function App() {
  useAppKeyboardInset();
  const { name, streak: hookStreak, initData, userId, photoUrl, inTelegram, tg } = useTelegramWebApp();
  const showAlert = useCallback((m: string) => {
    if (tg?.showAlert) void tg.showAlert(m);
    else window.alert(m);
  }, [tg]);
  const [tab, setTab] = useState<Tab>("feed");
  const [workoutOpen, setWorkoutOpen] = useState(false);
  const [supportOpen, setSupportOpen] = useState(false);
  const [streak, setStreak] = useState(hookStreak);
  const [recordStreak, setRecordStreak] = useState(hookStreak);
  const [profileDisplayName, setProfileDisplayName] = useState("");
  const [xp, setXP] = useState(0);
  const [achievementCount, setAchievementCount] = useState(0);
  const [achievementsMax, setAchievementsMax] = useState(9);
  const [workouts, setWorkouts] = useState(0);
  const [leoPending, setLeoPending] = useState(0);
  const [feedThreadUnread, setFeedThreadUnread] = useState(0);
  const [packGroupUnread, setPackGroupUnread] = useState(0);
  const feedUnread = feedThreadUnread + packGroupUnread;
  const [feedRefreshToken, setFeedRefreshToken] = useState(0);
  const [optimisticFeedItem, setOptimisticFeedItem] = useState<PackFeedItemDTO | null>(null);
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
      setFeedThreadUnread(0);
      setPackGroupUnread(0);
      return;
    }
    const [leo, feedThread, packGroup] = await Promise.all([
      fetchLeoPendingCount(initData),
      fetchFeedThreadUnreadSummary(initData),
      fetchPackGroupUnreadCount(initData),
    ]);
    setLeoPending(leo);
    setFeedThreadUnread(feedThread.count);
    setPackGroupUnread(packGroup);
  }, [accessGateStatus, inTelegram, initData]);

  const clearLeoBadge = useCallback(() => setLeoPending(0), []);
  const clearPackGroupBadge = useCallback(() => setPackGroupUnread(0), []);

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
        display_name?: string;
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
      setProfileDisplayName((j.display_name ?? "").trim());
      setDaysSinceLastTraining(typeof j.days_since_last_training === "number" ? j.days_since_last_training : -1);
      setStreak(typeof j.streak_days === "number" ? j.streak_days : 0);
      setRecordStreak(typeof j.max_streak_days === "number" ? j.max_streak_days : 0);
      setXP(typeof j.xp === "number" ? j.xp : 0);
      setAchievementCount(typeof j.achievement_count === "number" ? j.achievement_count : 0);
      setAchievementsMax(typeof j.achievements_max === "number" ? j.achievements_max : 9);
      setWorkouts(typeof j.workouts_total === "number" ? j.workouts_total : 0);
      // Автоопределение часового пояса из устройства: приводим хранимое смещение к зоне телефона.
      if (!tzSyncedRef.current) {
        tzSyncedRef.current = true;
        const currentOffset = typeof j.timezone_offset === "number" ? j.timezone_offset : 0;
        void syncDeviceTimezone(initData, currentOffset);
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

  const onLeoInboxDrained = useCallback(() => {
    void refreshTabBadges();
  }, [refreshTabBadges]);

  const onLeoChatOpened = useCallback(() => {
    clearLeoBadge();
  }, [clearLeoBadge]);

  const effectiveName = profileDisplayName.trim() || name.trim() || "друг";
  const tabsVisible = !supportOpen && !workoutOpen;

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
      <div className="app__tabs">
        <TabKeepAlive active={tab === "feed"} hidden={!tabsVisible}>
          <FeedScreen
            active={tab === "feed" && tabsVisible}
            name={effectiveName}
            streak={streak}
            userId={userId}
            initData={initData}
            inTelegram={inTelegram}
            showAlert={showAlert}
            refreshToken={feedRefreshToken}
            optimisticFeedItem={optimisticFeedItem}
            onOptimisticConsumed={() => setOptimisticFeedItem(null)}
            feedThreadUnreadCount={feedThreadUnread}
            packGroupUnreadCount={packGroupUnread}
            onRefreshAll={async () => {
              await Promise.all([refreshProfileStats(), refreshTabBadges()]);
            }}
            onRefreshTabBadges={refreshTabBadges}
            onPackGroupChatOpened={clearPackGroupBadge}
          />
        </TabKeepAlive>
        <TabKeepAlive active={tab === "chat"} hidden={!tabsVisible}>
          <ChatScreen
            active={tab === "chat" && tabsVisible}
            name={effectiveName}
            initData={initData}
            inTelegram={inTelegram}
            showAlert={showAlert}
            onInboxOpened={onLeoChatOpened}
            onInboxDrained={onLeoInboxDrained}
          />
        </TabKeepAlive>
        <TabKeepAlive active={tab === "rules"} hidden={!tabsVisible}>
          <RulesScreen />
        </TabKeepAlive>
        <TabKeepAlive active={tab === "profile"} hidden={!tabsVisible}>
          <ProfileScreen
            active={tab === "profile" && tabsVisible}
            name={effectiveName}
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
            onProfileSaved={(displayName) => {
              setProfileDisplayName(displayName.trim());
            }}
            onSupport={() => {
              setWorkoutOpen(false);
              setSupportOpen(true);
            }}
          />
        </TabKeepAlive>
      </div>

      {supportOpen ? (
        <SupportScreen initData={initData} inTelegram={inTelegram} showAlert={showAlert} />
      ) : null}

      <BottomNav
        active={tab}
        leoBadgeCount={leoPending}
        feedBadgeCount={feedUnread}
        onChat={() => {
          setWorkoutOpen(false);
          setSupportOpen(false);
          clearLeoBadge();
          setTab("chat");
        }}
        onFeed={() => {
          setWorkoutOpen(false);
          setSupportOpen(false);
          setTab("feed");
        }}
        onAddWorkout={() => {
          setSupportOpen(false);
          setWorkoutOpen(true);
        }}
        onRules={() => {
          setWorkoutOpen(false);
          setSupportOpen(false);
          setTab("rules");
        }}
        onProfile={() => {
          setWorkoutOpen(false);
          setSupportOpen(false);
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
            const base = `${kind}, ${min} мин, инт. ${intensity}/5`;
            const line = note ? `${base}\n\n${note}` : base;
            tg?.HapticFeedback?.impactOccurred?.("medium");
            // Отчёт: сервер отдаёт reply_text сразу (стрик, кубки, ачивки).
            const result = photo
              ? await sendMiniappTrainingWithPhoto(initData, line, photo)
              : await sendMiniappPrivateText(initData, line);
            if (!result.ok) {
              showAlert(result.error);
              return false;
            }
            setOptimisticFeedItem(
              buildOptimisticTrainingFeedItem({
                userId: userId || 0,
                username: effectiveName,
                text: line,
                streakDays: streak,
                trainingPhotoUrl: result.trainingPhotoUrl,
                authorPhotoUrl: photoUrl,
              }),
            );
            void refreshTabBadges();
            void refreshProfileStats();
            setTab("feed");
            setFeedRefreshToken((v) => v + 1);
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
