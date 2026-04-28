import { LEO_AVATAR_URL } from "./leoAvatar";
import { timeAgoFromISO } from "./timeAgo";
import type { ActivityCardProps } from "../components/ActivityCard";

const viteMiniappApi = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "").trim() ?? "";

/** Лента всегда ходит на VITE_MINIAPP_API_URL; в БД мог остаться старый origin (127.0.0.1 или другой домен). */
export function resolveTrainingPhotoUrl(stored: string | undefined): string | undefined {
  const raw = (stored ?? "").trim();
  if (!raw) return undefined;
  const marker = "/api/miniapp/media/";
  const idx = raw.indexOf(marker);
  let path = "";
  if (idx >= 0) {
    path = raw.slice(idx);
  } else if (raw.startsWith(marker.slice(1))) {
    path = "/" + raw;
  } else if (raw.startsWith(marker)) {
    path = raw;
  } else {
    return raw;
  }
  if (!viteMiniappApi) return raw;
  return viteMiniappApi + path;
}

/** Совпадает с ms_leo trainingFeedAllowedEmojis (порядок отображения). */
export const TRAINING_FEED_EMOJIS = [
  "🔥",
  "💪",
  "👏",
  "❤️",
  "🎉",
  "🦁",
  "⭐",
  "👍",
  "🙌",
  "✨",
  "🤝",
  "⚡",
  "🎯",
  "😤",
  "👀",
  "🙏",
  "😱",
] as const;

export type PackFeedReactionDTO = { emoji: string; count: number; me: boolean };

export type PackFeedThreadReplyDTO = {
  id: number;
  user_id: number;
  username: string;
  text: string;
  created_at: string;
  is_you: boolean;
  is_leo?: boolean;
  /** URL из miniapp_user_profile (Telegram WebApp user.photo_url при онбординге). */
  author_photo_url?: string;
  /** Ответ на сообщение внутри треда (как Reply в Telegram). */
  reply_to_id?: number;
  reply_to_username?: string;
  reply_to_text?: string;
  reply_to_is_leo?: boolean;
};

export type PackFeedItemDTO = {
  id: number;
  user_id: number;
  username: string;
  type: string;
  text: string;
  created_at: string;
  streak_days: number;
  is_you: boolean;
  pack_chat_id?: number;
  pack_title?: string;
  author_photo_url?: string;
  reactions?: PackFeedReactionDTO[];
  thread?: PackFeedThreadReplyDTO[];
  /** Публичный URL фото из мини‑аппа после POST /api/miniapp/workout */
  training_photo_url?: string;
};

/** Полная строка эмодзи для кнопок реакций (с нулевыми счётчиками). */
export function mergeTrainingFeedReactions(fromServer?: PackFeedReactionDTO[]): { emoji: string; count: number; me: boolean }[] {
  const byEmoji = new Map<string, PackFeedReactionDTO>();
  for (const r of fromServer ?? []) {
    byEmoji.set(r.emoji, r);
  }
  return TRAINING_FEED_EMOJIS.map((emoji) => {
    const r = byEmoji.get(emoji);
    return { emoji, count: r?.count ?? 0, me: r?.me ?? false };
  });
}

function typeMeta(t: string): { emoji: string; activity: string; details: string } {
  switch (t) {
    case "training_done":
      return { emoji: "💪", activity: "Тренировка", details: "Отчёт #training_done" };
    case "sick_leave":
      return { emoji: "🏥", activity: "Больничный", details: "Заявка #sick_leave" };
    case "healthy":
      return { emoji: "💚", activity: "Выздоровление", details: "#healthy" };
    case "pack_join":
      return { emoji: "🐆", activity: "Лео · приветствие", details: "Новый участник" };
    case "pack_rejoin":
      return { emoji: "🐆", activity: "Лео · приветствие", details: "Вернулся в стаю" };
    case "daily_wisdom":
      return { emoji: "🌅", activity: "Мудрость дня", details: "Мудрость дня" };
    case "pack_removed":
      return { emoji: "🐆", activity: "Лео · стая", details: "Выбыл за неактивность" };
    case "inactive_notice":
      return { emoji: "⏳", activity: "Лео · напоминание стае", details: "Таймер неактивности (дубль контекста)" };
    default:
      return { emoji: "📝", activity: t, details: "Сообщение" };
  }
}

function avatarFor(name: string) {
  const t = (name || "?").trim();
  const first = t[0] ?? "🐆";
  if (first && !/[a-zA-Zа-яА-Я0-9@#]/.test(first)) {
    return first; // already emoji
  }
  return "🐾";
}

export function dtoToCard(d: PackFeedItemDTO): ActivityCardProps {
  const m = typeMeta(d.type);
  const isLeoNotice =
    d.type === "pack_join" ||
    d.type === "pack_rejoin" ||
    d.type === "daily_wisdom" ||
    d.type === "pack_removed";
  const newcomer = (d.username || "").trim() || `Участник ${d.user_id}`;
  const pic = (d.author_photo_url || "").trim();
  const commentRaw = d.text.trim();
  const maxComment =
    d.type === "training_done" || d.type === "daily_wisdom" ? 2000 : 280;
  const comment = commentRaw.length > maxComment ? commentRaw.slice(0, maxComment - 1) + "…" : commentRaw;
  if (isLeoNotice) {
    let leoDetails = newcomer;
    if (d.type === "daily_wisdom") leoDetails = "Мудрость дня";
    else if (d.type === "pack_removed") leoDetails = newcomer;
    return {
      avatar: LEO_AVATAR_URL,
      name: "Лео",
      streak: 0,
      hideStreak: true,
      timeAgo: timeAgoFromISO(d.created_at),
      emoji: m.emoji,
      activity: m.activity,
      details: leoDetails,
      comment,
    };
  }
  return {
    avatar: pic || avatarFor(d.username),
    name: d.is_you ? "Ты" : d.username || `Участник ${d.user_id}`,
    streak: d.streak_days,
    timeAgo: timeAgoFromISO(d.created_at),
    emoji: m.emoji,
    activity: m.activity,
    details: m.details,
    comment,
    trainingPhotoUrl: resolveTrainingPhotoUrl(d.training_photo_url),
  };
}
