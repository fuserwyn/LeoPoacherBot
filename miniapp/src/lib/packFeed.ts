import { LEO_AVATAR_URL } from "./leoAvatar";
import { timeAgoFromISO } from "./timeAgo";
import { trainingDoneCategoryDisplayLabel, trainingDoneCategoryEmoji } from "./workoutCategories";
import type { ActivityCardProps } from "../components/ActivityCard";

const viteMiniappApi = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "").trim() ?? "";
const browserOrigin = typeof window !== "undefined" ? window.location.origin.replace(/\/$/, "") : "";

/** Лента: в БД мог остаться старый origin; API additionally canonicalizes via MINIAPP_PUBLIC_BASE_URL. */
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
  // Уже канонический https с API — не перетираем ошибочным VITE_MINIAPP_API_URL (частая причина битых картинок).
  if (raw.startsWith("https://") && !raw.includes("127.0.0.1") && !raw.includes("localhost")) {
    return raw;
  }
  if (viteMiniappApi) return viteMiniappApi + path;
  if (browserOrigin && browserOrigin.startsWith("http")) return browserOrigin + path;
  return raw;
}

/** Аватары в ленте: абсолютные URL как есть; относительные `/api/miniapp/...` — к базе API (прокси user-avatar). */
export function resolveFeedAvatarUrl(stored: string | undefined): string {
  const raw = (stored ?? "").trim();
  if (!raw) return "";
  if (raw.startsWith("http://") || raw.startsWith("https://")) return raw;
  if (raw.startsWith("/") && viteMiniappApi) return viteMiniappApi + raw;
  return raw;
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
  "🏆",
  "💯",
  "🥳",
  "🤩",
  "😮",
  "💦",
  "🤗",
  "👌",
  "🫶",
  "🧡",
  "💜",
  "🤙",
  "🫡",
  "🤘",
  "🏃",
  "🧘",
  "🤯",
  "💤",
] as const;

/** Совпадает с ms_leo sickLeaveAllowedEmojis. */
export const SICK_LEAVE_FEED_EMOJIS = ["😢", "😔", "🥺", "🤒", "🫂", "🙏", "❤️", "💙", "🌧️", "💤"] as const;

/** Совпадает с ms_leo healthyAllowedEmojis. */
export const HEALTHY_FEED_EMOJIS = ["🎉", "🥳", "😄", "💚", "❤️", "👏", "🙌", "✨", "🌟", "💪"] as const;

export type PackFeedReactionDTO = { emoji: string; count: number; me: boolean; voters?: string[] };

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
  like_count?: number;
  like_me?: boolean;
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

/** Полная строка эмодзи для кнопок реакций (с нулевыми счётчиками). Собственная реакция (`me`) показывается первой — в т.ч. если её выбрали в меню «⋯». */
export function mergeTrainingFeedReactions(fromServer?: PackFeedReactionDTO[]): { emoji: string; count: number; me: boolean; voters?: string[] }[] {
  return mergePackFeedReactions(TRAINING_FEED_EMOJIS, fromServer);
}

export function mergePackFeedReactions(
  allowedEmojis: readonly string[],
  fromServer?: PackFeedReactionDTO[],
): { emoji: string; count: number; me: boolean; voters?: string[] }[] {
  const byEmoji = new Map<string, PackFeedReactionDTO>();
  for (const r of fromServer ?? []) {
    byEmoji.set(r.emoji, r);
  }
  const ordered = allowedEmojis.map((emoji) => {
    const r = byEmoji.get(emoji);
    return { emoji, count: r?.count ?? 0, me: r?.me ?? false, voters: r?.voters };
  });
  const myIdx = ordered.findIndex((x) => x.me);
  if (myIdx <= 0) return ordered;
  const mine = ordered[myIdx]!;
  const rest = [...ordered.slice(0, myIdx), ...ordered.slice(myIdx + 1)];
  return [mine, ...rest];
}

function typeMeta(t: string): { emoji: string; activity: string; details: string } {
  switch (t) {
    case "training_done":
      return { emoji: "💪", activity: "", details: "" };
    case "sick_leave":
      return { emoji: "🏥", activity: "Больничный", details: "" };
    case "healthy":
      return { emoji: "💚", activity: "Выздоровление", details: "" };
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

/** С первой строки тела отчёта убираем `#training_done — `, остаётся «силовая, 60 мин…». */
function stripFirstLineTrainingDoneTag(text: string): string {
  const t = text.trim();
  if (t === "") return t;
  const nl = t.indexOf("\n");
  const first = nl >= 0 ? t.slice(0, nl) : t;
  const rest = nl >= 0 ? t.slice(nl + 1) : "";
  let stripped = first.replace(/^#training_done\s*[—–-]\s*/i, "").trim();
  if (stripped === first.trim()) {
    stripped = first.replace(/^#training_done\b\s*/i, "").trim();
  }
  const combined = rest ? `${stripped}\n${rest}` : stripped;
  return combined.trim();
}

/** Убирает ведущие строки с тегом — заголовок карточки уже передаёт смысл (#healthy / #sick_leave не дублируем мелким текстом). */
function stripLeadingFeedHashtag(text: string, tag: "#healthy" | "#sick_leave"): string {
  const esc = tag.replace("#", "\\#");
  return text.replace(new RegExp(`^(?:\\s*${esc}\\b[^\\n]*(?:\\n|$))+`, "gi"), "").trim();
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
  const trainingEmoji = d.type === "training_done" ? trainingDoneCategoryEmoji(d.text) : m.emoji;
  const isLeoNotice =
    d.type === "pack_join" ||
    d.type === "pack_rejoin" ||
    d.type === "daily_wisdom" ||
    d.type === "pack_removed";
  const newcomer = (d.username || "").trim() || `Участник ${d.user_id}`;
  const pic = resolveFeedAvatarUrl(d.author_photo_url);
  let commentRaw = d.text.trim();
  if (d.type === "healthy") {
    commentRaw = stripLeadingFeedHashtag(commentRaw, "#healthy");
  } else if (d.type === "sick_leave") {
    commentRaw = stripLeadingFeedHashtag(commentRaw, "#sick_leave");
  } else if (d.type === "training_done") {
    commentRaw = stripFirstLineTrainingDoneTag(commentRaw);
  }
  const maxComment =
    d.type === "training_done" ||
    d.type === "daily_wisdom" ||
    d.type === "pack_join" ||
    d.type === "pack_rejoin"
      ? 2000
      : 280;
  const comment =
    commentRaw.length > maxComment ? commentRaw.slice(0, maxComment - 1) + "…" : commentRaw;
  if (isLeoNotice) {
    let leoDetails = newcomer;
    if (d.type === "daily_wisdom") leoDetails = "Мудрость дня";
    else if (d.type === "pack_removed") leoDetails = newcomer;
    else if (d.type === "pack_join") leoDetails = "Новый участник";
    else if (d.type === "pack_rejoin") leoDetails = "Вернулся в стаю";
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
    avatar: (pic || "").trim() || avatarFor(d.username),
    name: d.is_you ? "Ты" : d.username || `Участник ${d.user_id}`,
    streak: d.streak_days,
    timeAgo: timeAgoFromISO(d.created_at),
    emoji: trainingEmoji,
    activity: d.type === "training_done" ? trainingDoneCategoryDisplayLabel(d.text) : m.activity,
    details: d.type === "training_done" ? "" : m.details,
    comment,
    trainingPhotoUrl: resolveTrainingPhotoUrl(d.training_photo_url),
  };
}
