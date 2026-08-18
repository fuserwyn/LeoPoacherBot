const api = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type AdminOverview = {
  users: number;
  reports_open: number;
  support_waiting: number;
  hidden: number;
  payments: number;
  access_price_rub: number;
};

export type AdminPaywallPrice = {
  amount_rub: number;
  amount_minor: number;
  currency: string;
  is_custom: boolean;
  default_amount_rub: number;
};

export type AdminSupportConv = {
  user_id: number;
  display_name: string;
  last_role: string;
  last_text: string;
  last_created: string;
  needs_reply: boolean;
};

export type AdminSupportMsg = {
  id: number;
  role: "user" | "support";
  text: string;
  created_at: string;
};

export type AdminReport = {
  id: number;
  reporter_user_id: number;
  reporter_name?: string;
  target_type: string;
  user_message_id: number;
  thread_reply_id?: number;
  target_user_id: number;
  target_name?: string;
  target_text: string;
  status: string;
  created_at: string;
};

export type AdminHiddenItem = {
  kind: "feed_post" | "thread_reply" | "pack_group_message" | string;
  id: number;
  parent_id?: number;
  author_user_id: number;
  author_name?: string;
  text: string;
  created_at: string;
  reason?: string;
};

export type AdminUserRow = {
  user_id: number;
  username?: string;
  display_name?: string;
  cups: number;
  streak_days: number;
  is_deleted: boolean;
  has_active_paywall: boolean;
};

export type AdminUserCard = {
  user_id: number;
  username?: string;
  display_name?: string;
  cups: number;
  level: number;
  level_name: string;
  streak_days: number;
  max_streak_days: number;
  days_since_last_training: number;
  last_training_date?: string;
  inactivity_removal_at?: string;
  streak_save_attempts_used: number;
  streak_save_attempts_max: number;
  streak_save_attempts_avail: number;
  sick_leave: string;
  is_deleted: boolean;
  has_active_paywall: boolean;
  ugc_violations: number;
  ugc_muted_until?: string;
  workouts_total: number;
};

export type AdminUserAction =
  | "sick_set"
  | "sick_cancel"
  | "restore_full"
  | "restore_scratch"
  | "mute"
  | "unmute"
  | "grant_save"
  | "kick";

type ErrBody = { error?: string; message?: string };

async function post<T>(path: string, initData: string, extra: Record<string, unknown> = {}): Promise<T> {
  if (!api) throw new Error("API не настроен");
  const res = await fetch(`${api}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ init_data: initData, ...extra }),
  });
  const j = (await res.json().catch(() => ({}))) as T & ErrBody & { ok?: boolean };
  if (!res.ok || j.ok === false) {
    throw new Error(j.message || adminErrorLabel(j.error) || `Ошибка ${res.status}`);
  }
  return j;
}

export function adminErrorLabel(code?: string): string {
  switch (code) {
    case "forbidden":
      return "Нет прав администратора";
    case "chat_mismatch":
      return "Открой мини-апп из чата стаи";
    case "not_found":
      return "Не найдено или уже обработано";
    case "invalid_action":
      return "Некорректное действие";
    case "empty_text":
      return "Напиши текст";
    case "text_too_long":
      return "Слишком длинный текст";
    case "moderation_blocked":
      return "Текст не прошёл модерацию";
    case "invalid_price":
      return "Цена должна быть от 1 до 100000 ₽";
    case "tracker_not_configured":
      return "Трекер не настроен: нет BOARD_SSO_SECRET";
    default:
      return code ?? "";
  }
}

export function fetchAdminOverview(initData: string) {
  return post<{ overview: AdminOverview }>("/api/miniapp/admin/overview", initData);
}

export function fetchAdminSupportInbox(initData: string) {
  return post<{ conversations: AdminSupportConv[] }>("/api/miniapp/admin/support/inbox", initData);
}

export function fetchAdminSupportThread(initData: string, targetUserId: number) {
  return post<{ messages: AdminSupportMsg[] }>("/api/miniapp/admin/support/thread", initData, {
    target_user_id: targetUserId,
  });
}

export function sendAdminSupportReply(initData: string, targetUserId: number, text: string) {
  return post("/api/miniapp/admin/support/reply", initData, { target_user_id: targetUserId, text });
}

export function fetchAdminReports(initData: string) {
  return post<{ reports: AdminReport[] }>("/api/miniapp/admin/reports", initData);
}

export function sendAdminReportAction(initData: string, reportId: number, action: "dismiss" | "hide") {
  return post("/api/miniapp/admin/reports/action", initData, { report_id: reportId, action });
}

export function fetchAdminHidden(initData: string) {
  return post<{ items: AdminHiddenItem[] }>("/api/miniapp/admin/hidden", initData);
}

export function restoreAdminHidden(initData: string, kind: string, id: number) {
  return post("/api/miniapp/admin/hidden/restore", initData, { kind, id });
}

export function fetchAdminUsers(initData: string, query = "", offset = 0) {
  return post<{ users: AdminUserRow[] }>("/api/miniapp/admin/users", initData, { query, offset });
}

export function fetchAdminUserCard(initData: string, targetUserId: number) {
  return post<{ user: AdminUserCard }>("/api/miniapp/admin/users/card", initData, {
    target_user_id: targetUserId,
  });
}

export function sendAdminUserAction(initData: string, targetUserId: number, action: AdminUserAction) {
  return post("/api/miniapp/admin/users/action", initData, { target_user_id: targetUserId, action });
}

export function publishAdminPost(initData: string, text: string, author: "leo" | "admin") {
  return post("/api/miniapp/admin/publish", initData, { text, author });
}

export function fetchAdminPaywallPrice(initData: string) {
  return post<{ price: AdminPaywallPrice }>("/api/miniapp/admin/paywall-price", initData);
}

export function saveAdminPaywallPrice(initData: string, amountRub: number) {
  return post<{ price: AdminPaywallPrice }>("/api/miniapp/admin/paywall-price/set", initData, {
    amount_rub: amountRub,
  });
}

export function resetAdminPaywallPrice(initData: string) {
  return post<{ price: AdminPaywallPrice }>("/api/miniapp/admin/paywall-price/set", initData, {
    reset: true,
  });
}

/* Разделы, переехавшие из чат-админки: аналитика, посещения, оплаты, админы,
   отложенные посты, опросы и очистка ленты. Считает всё бэкенд теми же
   запросами, что и для чата, — здесь только показываем. */

export type AdminTable = {
  title: string;
  subtitle: string;
  columns: string[];
  rows: string[][];
};

export type AdminAnalytics = {
  period: string;
  last_event_at: string;
  tables: AdminTable[];
};

export type AdminPerson = {
  user_id: number;
  username: string;
  static: boolean;
  added_at: string;
};

export type AdminScheduledPost = {
  id: number;
  author: string;
  text: string;
  scheduled_at: string;
};

export type AdminWipeCounts = {
  feed_posts: number;
  feed_threads: number;
  feed_reports: number;
  pack_chat_messages: number;
};

export function fetchAdminAnalytics(initData: string, days: number) {
  return post<{ analytics: AdminAnalytics }>("/api/miniapp/admin/analytics", initData, { days });
}

export function fetchAdminVisits(initData: string) {
  return post<{ tables: AdminTable[] }>("/api/miniapp/admin/visits", initData);
}

export function fetchAdminPayments(initData: string, offset = 0, limit = 20) {
  return post<{ payments: { total: number; offset: number; limit: number; table: AdminTable } }>(
    "/api/miniapp/admin/payments",
    initData,
    { offset, limit },
  );
}

export function fetchAdminAdmins(initData: string) {
  return post<{ admins: AdminPerson[] }>("/api/miniapp/admin/admins", initData);
}

export function addAdminPerson(initData: string, query: string) {
  return post<{ user_id: number }>("/api/miniapp/admin/admins/add", initData, { query });
}

export function removeAdminPerson(initData: string, userId: number) {
  return post<Record<string, never>>("/api/miniapp/admin/admins/remove", initData, { user_id: userId });
}

export function fetchAdminScheduledPosts(initData: string) {
  return post<{ posts: AdminScheduledPost[] }>("/api/miniapp/admin/scheduled", initData);
}

/** at — «2026-08-20T09:00» по МСК, как в поле datetime-local. */
export function addAdminScheduledPost(initData: string, author: string, text: string, at: string) {
  return post<{ id: number }>("/api/miniapp/admin/scheduled/add", initData, { author, text, at });
}

export function cancelAdminScheduledPost(initData: string, id: number) {
  return post<Record<string, never>>("/api/miniapp/admin/scheduled/cancel", initData, { id });
}

export function publishAdminPoll(initData: string, question: string, options: string[]) {
  return post<Record<string, never>>("/api/miniapp/admin/poll", initData, { question, options });
}

/** confirm=false — только посчитать, что удалится; true — удалить. */
export function wipePackFeed(initData: string, confirm: boolean) {
  return post<{ counts: AdminWipeCounts; done: boolean }>("/api/miniapp/admin/wipe", initData, { confirm });
}
