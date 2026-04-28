import { useCallback, useEffect, useState } from "react";
import { useTelegramWebApp } from "./hooks/useTelegramWebApp";
import { BottomNav } from "./components/BottomNav";
import { ChatScreen } from "./components/ChatScreen";
import { FeedScreen } from "./components/FeedScreen";
import { ProfileScreen } from "./components/ProfileScreen";
import { NewWorkoutScreen } from "./components/NewWorkoutScreen";
import { RulesScreen } from "./components/RulesScreen";
import { sendMiniappPrivateText } from "./lib/miniappPrivateSend";
import { fetchLeoPendingCount } from "./lib/leoPersonalInbox";
import { ensureMiniappOnboarding } from "./lib/miniappOnboarding";
import "./App.css";

type Tab = "chat" | "feed" | "rules" | "profile";

export function App() {
  const { name, streak, setStreak, initData, userId, photoUrl, inTelegram, tg } = useTelegramWebApp();
  const showAlert = useCallback((m: string) => {
    if (tg?.showAlert) void tg.showAlert(m);
    else window.alert(m);
  }, [tg]);
  const [tab, setTab] = useState<Tab>("feed");
  const [workoutOpen, setWorkoutOpen] = useState(false);
  const [workouts, setWorkouts] = useState(1);
  const [leoPending, setLeoPending] = useState(0);

  const refreshLeoPending = useCallback(async () => {
    if (!inTelegram || !initData?.trim()) {
      setLeoPending(0);
      return;
    }
    setLeoPending(await fetchLeoPendingCount(initData));
  }, [inTelegram, initData]);

  useEffect(() => {
    void refreshLeoPending();
    const t = window.setInterval(() => {
      void refreshLeoPending();
    }, 30_000);
    const onVis = () => {
      if (document.visibilityState === "visible") void refreshLeoPending();
    };
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.clearInterval(t);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [refreshLeoPending]);

  useEffect(() => {
    if (!inTelegram || !initData?.trim()) return;
    void ensureMiniappOnboarding(initData);
  }, [inTelegram, initData]);

  const onLeoInboxDrained = useCallback(() => {
    void refreshLeoPending();
  }, [refreshLeoPending]);

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
          showAlert={showAlert}
        />
      )}

      <BottomNav
        active={tab}
        leoBadgeCount={leoPending}
        onChat={() => setTab("chat")}
        onFeed={() => setTab("feed")}
        onRules={() => setTab("rules")}
        onWorkout={() => setWorkoutOpen(true)}
        onProfile={() => setTab("profile")}
      />

      {workoutOpen && (
        <NewWorkoutScreen
          onClose={() => setWorkoutOpen(false)}
          onSave={async ({ type, min, intensity, note }) => {
            if (!inTelegram || !initData) {
              showAlert("Открой мини-апп из Telegram (нужен initData).");
              return false;
            }
            const labels: Record<string, string> = {
              run: "бег",
              walk: "ходьба",
              bike: "велосипед",
              swim: "плавание",
              strength: "силовая",
              other: "другое",
            };
            const kind = labels[type] ?? type;
            const base = `#training_done — ${kind}, ${min} мин, инт. ${intensity}/5`;
            const line = note ? `${base}\n\n${note}` : base;
            tg?.HapticFeedback?.impactOccurred?.("medium");
            const result = await sendMiniappPrivateText(initData, line);
            if (!result.ok) {
              showAlert(result.error);
              return false;
            }
            void refreshLeoPending();
            setWorkouts((c) => c + 1);
            setStreak((s) => s + 1);
            const msg = result.replyParts.filter(Boolean).join("\n\n").trim() || "Отчёт отправлен.";
            showAlert(msg.length > 350 ? `${msg.slice(0, 347)}…` : msg);
            return true;
          }}
        />
      )}
    </div>
  );
}
