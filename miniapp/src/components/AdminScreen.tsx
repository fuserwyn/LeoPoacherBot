import { useCallback, useEffect, useState } from "react";
import { formatChatTime, timeAgoFromISO } from "../lib/timeAgo";
import { tgConfirm } from "../lib/tgConfirm";
import {
  fetchAdminHidden,
  fetchAdminOverview,
  fetchAdminPaywallPrice,
  fetchAdminReports,
  fetchAdminSupportInbox,
  fetchAdminSupportThread,
  fetchAdminUserCard,
  fetchAdminUsers,
  publishAdminPost,
  resetAdminPaywallPrice,
  restoreAdminHidden,
  saveAdminPaywallPrice,
  sendAdminReportAction,
  sendAdminSupportReply,
  sendAdminUserAction,
  type AdminHiddenItem,
  type AdminOverview,
  type AdminPaywallPrice,
  type AdminReport,
  type AdminSupportConv,
  type AdminSupportMsg,
  type AdminUserAction,
  type AdminUserCard,
  type AdminUserRow,
} from "../lib/adminApi";
import { AdminDataScreen } from "./AdminDataScreen";
import { AdminOpsScreen, type AdminOpsSection } from "./AdminOpsScreen";
import { AdminResourcesScreen } from "./AdminResourcesScreen";
import { LeoLabScreen } from "./LeoLabScreen";
import { TrackerScreen } from "./TrackerScreen";
import "./AdminScreen.css";

type Page =
  | "home"
  | "support"
  | "thread"
  | "reports"
  | "hidden"
  | "users"
  | "card"
  | "announce"
  | "price"
  | "leolab"
  | AdminOpsSection;

/** Вкладки админского таббара: у админа свои разделы, пользовательские тут ни к чему. */
type AdminTab = "community" | "system" | "data" | "resources" | "tracker";

const ADMIN_TABS: { key: AdminTab; icon: string; label: string }[] = [
  { key: "community", icon: "👥", label: "Сообщество" },
  { key: "system", icon: "⚙️", label: "Система" },
  { key: "data", icon: "🗄", label: "Данные" },
  { key: "resources", icon: "💵", label: "Ресурсы" },
  { key: "tracker", icon: "🗂", label: "Трекер" },
];

type Props = {
  initData: string;
  inTelegram: boolean;
  showAlert: (m: string) => void;
  onClose: () => void;
};

const EMPTY_OVERVIEW: AdminOverview = {
  users: 0,
  reports_open: 0,
  support_waiting: 0,
  hidden: 0,
  payments: 0,
  access_price_rub: 0,
};

function clip(text: string, n: number) {
  const t = text.trim();
  if (t.length <= n) return t;
  return `${t.slice(0, n - 1)}…`;
}

function userLabel(name?: string, username?: string, id?: number) {
  const dn = name?.trim();
  if (dn) return dn;
  const un = username?.trim();
  if (un) return un.startsWith("@") ? un : `@${un}`;
  return id ? `id ${id}` : "пользователь";
}

function reportKind(type: string) {
  switch (type) {
    case "thread_reply":
      return "комментарий";
    case "pack_group_message":
      return "чат стаи";
    default:
      return "пост";
  }
}

function hiddenKind(kind: string) {
  switch (kind) {
    case "thread_reply":
      return "комментарий";
    case "pack_group_message":
      return "чат стаи";
    default:
      return "пост";
  }
}

export function AdminScreen({ initData, inTelegram, showAlert, onClose }: Props) {
  const [page, setPage] = useState<Page>("home");
  // Кик — единственное необратимое действие в карточке, поэтому подтверждаем
  // его своей модалкой: нативный confirm Telegram на части клиентов молча
  // не показывается, и участник улетал из стаи с одного тапа.
  const [kickAsk, setKickAsk] = useState(false);
  const [tab, setTab] = useState<AdminTab>("community");
  const [overview, setOverview] = useState<AdminOverview>(EMPTY_OVERVIEW);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const [convs, setConvs] = useState<AdminSupportConv[]>([]);
  const [threadUser, setThreadUser] = useState<AdminSupportConv | null>(null);
  const [thread, setThread] = useState<AdminSupportMsg[]>([]);
  const [reply, setReply] = useState("");

  const [reports, setReports] = useState<AdminReport[]>([]);
  const [hidden, setHidden] = useState<AdminHiddenItem[]>([]);

  const [users, setUsers] = useState<AdminUserRow[]>([]);
  const [userQuery, setUserQuery] = useState("");
  const [card, setCard] = useState<AdminUserCard | null>(null);

  const [announce, setAnnounce] = useState("");
  const [announceAuthor, setAnnounceAuthor] = useState<"admin" | "leo">("admin");

  const [price, setPrice] = useState<AdminPaywallPrice | null>(null);
  const [priceInput, setPriceInput] = useState("");

  const loadOverview = useCallback(async () => {
    if (!inTelegram || !initData) return;
    try {
      const j = await fetchAdminOverview(initData);
      setOverview(j.overview);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось загрузить админку");
    } finally {
      setLoading(false);
    }
  }, [inTelegram, initData, showAlert]);

  useEffect(() => {
    void loadOverview();
  }, [loadOverview]);

  const openSupport = async () => {
    setPage("support");
    try {
      const j = await fetchAdminSupportInbox(initData);
      setConvs(j.conversations ?? []);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось загрузить поддержку");
    }
  };

  const openThread = async (c: AdminSupportConv) => {
    setThreadUser(c);
    setPage("thread");
    try {
      const j = await fetchAdminSupportThread(initData, c.user_id);
      setThread(j.messages ?? []);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось открыть диалог");
    }
  };

  const sendReply = async () => {
    if (!threadUser || busy) return;
    const text = reply.trim();
    if (!text) return;
    setBusy(true);
    try {
      await sendAdminSupportReply(initData, threadUser.user_id, text);
      setReply("");
      const j = await fetchAdminSupportThread(initData, threadUser.user_id);
      setThread(j.messages ?? []);
      void loadOverview();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось ответить");
    } finally {
      setBusy(false);
    }
  };

  const openReports = async () => {
    setPage("reports");
    try {
      const j = await fetchAdminReports(initData);
      setReports(j.reports ?? []);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось загрузить жалобы");
    }
  };

  const actReport = async (id: number, action: "dismiss" | "hide") => {
    if (busy) return;
    if (action === "hide" && !(await tgConfirm("Скрыть контент у всех и закрыть жалобу?"))) return;
    setBusy(true);
    try {
      await sendAdminReportAction(initData, id, action);
      setReports((prev) => prev.filter((r) => r.id !== id));
      void loadOverview();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось обработать жалобу");
    } finally {
      setBusy(false);
    }
  };

  const openHidden = async () => {
    setPage("hidden");
    try {
      const j = await fetchAdminHidden(initData);
      setHidden(j.items ?? []);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось загрузить скрытое");
    }
  };

  const restoreHidden = async (item: AdminHiddenItem) => {
    if (busy) return;
    if (!(await tgConfirm("Вернуть этот контент в ленту / чат?"))) return;
    setBusy(true);
    try {
      await restoreAdminHidden(initData, item.kind, item.id);
      setHidden((prev) => prev.filter((h) => !(h.kind === item.kind && h.id === item.id)));
      void loadOverview();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось вернуть");
    } finally {
      setBusy(false);
    }
  };

  const loadUsers = async (query = userQuery) => {
    try {
      const j = await fetchAdminUsers(initData, query.trim());
      setUsers(j.users ?? []);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось загрузить пользователей");
    }
  };

  const openUsers = async () => {
    setPage("users");
    await loadUsers("");
  };

  const openCard = async (userId: number) => {
    setPage("card");
    try {
      const j = await fetchAdminUserCard(initData, userId);
      setCard(j.user);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Пользователь не найден");
      setPage("users");
    }
  };

  const runUserAction = async (action: AdminUserAction) => {
    if (!card || busy) return;
    setBusy(true);
    try {
      await sendAdminUserAction(initData, card.user_id, action);
      const j = await fetchAdminUserCard(initData, card.user_id);
      setCard(j.user);
      void loadOverview();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось выполнить действие");
    } finally {
      setBusy(false);
    }
  };

  const actUser = async (action: AdminUserAction, confirmText: string) => {
    if (!card || busy) return;
    if (!(await tgConfirm(confirmText))) return;
    setBusy(true);
    try {
      await sendAdminUserAction(initData, card.user_id, action);
      const j = await fetchAdminUserCard(initData, card.user_id);
      setCard(j.user);
      void loadOverview();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось выполнить действие");
    } finally {
      setBusy(false);
    }
  };

  const openPrice = async () => {
    setPage("price");
    try {
      const j = await fetchAdminPaywallPrice(initData);
      setPrice(j.price);
      setPriceInput(String(j.price.amount_rub));
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось загрузить цену");
    }
  };

  const savePrice = async () => {
    if (busy) return;
    const n = Number.parseInt(priceInput.trim(), 10);
    if (!Number.isFinite(n) || n < 1 || n > 100000) {
      showAlert("Цена — целое число от 1 до 100000 ₽");
      return;
    }
    if (!(await tgConfirm(`Выставить цену доступа ${n} ₽? Новые оплаты пойдут по этой сумме.`))) return;
    setBusy(true);
    try {
      const j = await saveAdminPaywallPrice(initData, n);
      setPrice(j.price);
      setPriceInput(String(j.price.amount_rub));
      setOverview((prev) => ({ ...prev, access_price_rub: j.price.amount_rub }));
      showAlert(`Цена доступа: ${j.price.amount_rub} ₽`);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось сохранить цену");
    } finally {
      setBusy(false);
    }
  };

  const resetPrice = async () => {
    if (busy || !price) return;
    if (!(await tgConfirm(`Вернуть цену к значению сервера (${price.default_amount_rub} ₽)?`))) return;
    setBusy(true);
    try {
      const j = await resetAdminPaywallPrice(initData);
      setPrice(j.price);
      setPriceInput(String(j.price.amount_rub));
      setOverview((prev) => ({ ...prev, access_price_rub: j.price.amount_rub }));
      showAlert(`Цена сброшена: ${j.price.amount_rub} ₽`);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось сбросить цену");
    } finally {
      setBusy(false);
    }
  };

  const publish = async () => {
    const text = announce.trim();
    if (!text || busy) return;
    if (!(await tgConfirm("Опубликовать объявление в ленте стаи?"))) return;
    setBusy(true);
    try {
      await publishAdminPost(initData, text, announceAuthor);
      setAnnounce("");
      showAlert("Опубликовано в ленте");
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось опубликовать");
    } finally {
      setBusy(false);
    }
  };

  const back = () => {
    if (page === "thread") {
      setPage("support");
      void openSupport();
      return;
    }
    if (page === "card") {
      setPage("users");
      return;
    }
    if (page === "home") {
      onClose();
      return;
    }
    setPage("home");
  };

  const title =
    page === "home"
      ? (ADMIN_TABS.find((t) => t.key === tab)?.label ?? "Админка")
      : page === "support"
        ? "Поддержка"
        : page === "thread"
          ? userLabel(threadUser?.display_name, undefined, threadUser?.user_id)
          : page === "reports"
            ? "Жалобы"
            : page === "hidden"
              ? "Скрытое"
              : page === "users"
                ? "Участники"
                : page === "card"
                  ? userLabel(card?.display_name, card?.username, card?.user_id)
                  : page === "price"
                    ? "Цена доступа"
                    : page === "analytics"
                        ? "Аналитика"
                        : page === "visits"
                          ? "Посещения бота"
                          : page === "payments"
                            ? "Оплаты"
                            : page === "admins"
                              ? "Админы"
                              : page === "scheduled"
                                ? "Отложенные посты"
                                : page === "poll"
                                  ? "Опрос в ленту"
                                  : page === "leolab"
                                    ? "Тест Лео"
                                    : page === "wipe"
                                    ? "Очистить ленту и чат"
                                    : "Объявление";

  return (
    <div className="admin">
      <header className="admin__head">
        <button type="button" className="admin__back" onClick={back}>
          ←
        </button>
        <h1 className="admin__title">{title}</h1>
        <span className="admin__head-spacer" />
      </header>

      {page === "home" && tab === "community" && (
        <div className="admin__body">
          {loading ? (
            <p className="admin__muted">Загрузка…</p>
          ) : (
            <>
              <div className="admin__stats">
                <div className="admin__stat">
                  <strong>{overview.users}</strong>
                  <span>в стае</span>
                </div>
                <div className="admin__stat">
                  <strong>{overview.support_waiting}</strong>
                  <span>ждут ответа</span>
                </div>
                <div className="admin__stat">
                  <strong>{overview.reports_open}</strong>
                  <span>жалоб</span>
                </div>
                <div className="admin__stat">
                  <strong>{overview.hidden}</strong>
                  <span>скрыто</span>
                </div>
              </div>
              <div className="admin__tiles">
                <button type="button" className="admin__tile" onClick={() => void openSupport()}>
                  <span className="admin__tile-ico">💬</span>
                  <span className="admin__tile-text">
                    <b>Поддержка</b>
                    <small>{overview.support_waiting > 0 ? `${overview.support_waiting} без ответа` : "все отвечены"}</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => void openReports()}>
                  <span className="admin__tile-ico">🚩</span>
                  <span className="admin__tile-text">
                    <b>Жалобы</b>
                    <small>{overview.reports_open > 0 ? `${overview.reports_open} открытых` : "пусто"}</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => void openHidden()}>
                  <span className="admin__tile-ico">🙈</span>
                  <span className="admin__tile-text">
                    <b>Скрытое</b>
                    <small>вернуть посты и комментарии</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => void openUsers()}>
                  <span className="admin__tile-ico">👥</span>
                  <span className="admin__tile-text">
                    <b>Участники</b>
                    <small>поиск, больничный, кик</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => setPage("announce")}>
                  <span className="admin__tile-ico">📣</span>
                  <span className="admin__tile-text">
                    <b>Объявление</b>
                    <small>пост в ленту от админа или Лео</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => setPage("poll")}>
                  <span className="admin__tile-ico">🗳</span>
                  <span className="admin__tile-text">
                    <b>Опрос</b>
                    <small>вопрос и варианты в ленту</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => setPage("scheduled")}>
                  <span className="admin__tile-ico">📅</span>
                  <span className="admin__tile-text">
                    <b>Отложенные</b>
                    <small>посты по расписанию</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => setPage("payments")}>
                  <span className="admin__tile-ico">💳</span>
                  <span className="admin__tile-text">
                    <b>Оплаты</b>
                    <small>заявки и статусы доступа</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => void openPrice()}>
                  <span className="admin__tile-ico">💰</span>
                  <span className="admin__tile-text">
                    <b>Цена доступа</b>
                    <small>
                      {overview.access_price_rub > 0
                        ? `${overview.access_price_rub} ₽ · вход и возврат`
                        : "сколько платят за вход и возврат"}
                    </small>
                  </span>
                </button>
              </div>
              <p className="admin__muted admin__hint">
                Всё то же есть и в боте: /admin. Кубки и стрики конкретного человека — в карточке участника.
              </p>
            </>
          )}
        </div>
      )}

      {page === "home" && tab === "system" && (
        <div className="admin__body">
          <div className="admin__tiles">
                <button type="button" className="admin__tile" onClick={() => setPage("analytics")}>
                  <span className="admin__tile-ico">📈</span>
                  <span className="admin__tile-text">
                    <b>Аналитика</b>
                    <small>воронки, KPI, каналы</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => setPage("visits")}>
                  <span className="admin__tile-ico">📊</span>
                  <span className="admin__tile-text">
                    <b>Посещения бота</b>
                    <small>кто и как часто заходит</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => setPage("admins")}>
                  <span className="admin__tile-ico">🛡</span>
                  <span className="admin__tile-text">
                    <b>Админы</b>
                    <small>выдать и снять права</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => setPage("leolab")}>
                  <span className="admin__tile-ico">🐆</span>
                  <span className="admin__tile-text">
                    <b>Тест Лео</b>
                    <small>спросить, сменить промпт, научить</small>
                  </span>
                </button>
                <button type="button" className="admin__tile" onClick={() => setPage("wipe")}>
                  <span className="admin__tile-ico">🗑</span>
                  <span className="admin__tile-text">
                    <b>Очистить ленту и чат</b>
                    <small>удалить всё содержимое стаи</small>
                  </span>
                </button>
          </div>
          <p className="admin__muted admin__hint">
            Технические разделы: метрики продукта, доступы и разовые операции над содержимым стаи.
          </p>
        </div>
      )}

      {page === "home" && tab === "data" && (
        <div className="admin__body">
          <AdminDataScreen initData={initData} showAlert={showAlert} />
        </div>
      )}

      {page === "home" && tab === "resources" && (
        <div className="admin__body">
          <AdminResourcesScreen initData={initData} showAlert={showAlert} />
        </div>
      )}

      {page === "home" && tab === "tracker" && (
        <div className="admin__body">
          <TrackerScreen initData={initData} showAlert={showAlert} />
        </div>
      )}

      {page === "leolab" && (
        <div className="admin__body">
          <LeoLabScreen initData={initData} showAlert={showAlert} />
        </div>
      )}

      {(page === "analytics" ||
        page === "visits" ||
        page === "payments" ||
        page === "admins" ||
        page === "scheduled" ||
        page === "poll" ||
        page === "wipe") && (
        <div className="admin__body">
          <AdminOpsScreen section={page} initData={initData} showAlert={showAlert} />
        </div>
      )}

      {page === "support" && (
        <div className="admin__body">
          {convs.length === 0 ? (
            <p className="admin__muted">Диалогов пока нет</p>
          ) : (
            <ul className="admin__list">
              {convs.map((c) => (
                <li key={c.user_id}>
                  <button type="button" className="admin__row" onClick={() => void openThread(c)}>
                    <div className="admin__row-top">
                      <b>{userLabel(c.display_name, undefined, c.user_id)}</b>
                      {c.needs_reply ? <span className="admin__badge">ждёт</span> : null}
                    </div>
                    <p className="admin__row-sub">{clip(c.last_text, 90)}</p>
                    <span className="admin__row-meta">{timeAgoFromISO(c.last_created)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {page === "thread" && threadUser && (
        <div className="admin__thread">
          <div className="admin__thread-log">
            {thread.map((m) => (
              <div key={m.id} className={`admin__bubble admin__bubble--${m.role}`}>
                <p>{m.text}</p>
                <time>{timeAgoFromISO(m.created_at)}</time>
              </div>
            ))}
          </div>
          <form
            className="admin__composer"
            onSubmit={(e) => {
              e.preventDefault();
              void sendReply();
            }}
          >
            <input
              value={reply}
              onChange={(e) => setReply(e.target.value)}
              placeholder="Ответ пользователю…"
              maxLength={4000}
            />
            <button type="submit" disabled={busy || !reply.trim()}>
              ➤
            </button>
          </form>
        </div>
      )}

      {page === "reports" && (
        <div className="admin__body">
          {reports.length === 0 ? (
            <p className="admin__muted">Открытых жалоб нет</p>
          ) : (
            <ul className="admin__list">
              {reports.map((r) => (
                <li key={r.id} className="admin__card">
                  <div className="admin__row-top">
                    <b>{reportKind(r.target_type)}</b>
                    <span className="admin__row-meta">{timeAgoFromISO(r.created_at)}</span>
                  </div>
                  <p className="admin__row-sub">{clip(r.target_text || "—", 160)}</p>
                  <p className="admin__row-meta">
                    на {userLabel(r.target_name, undefined, r.target_user_id)} · от{" "}
                    {userLabel(r.reporter_name, undefined, r.reporter_user_id)}
                  </p>
                  <div className="admin__actions">
                    <button type="button" className="admin__btn admin__btn--ghost" onClick={() => void actReport(r.id, "dismiss")} disabled={busy}>
                      Закрыть
                    </button>
                    <button type="button" className="admin__btn admin__btn--danger" onClick={() => void actReport(r.id, "hide")} disabled={busy}>
                      Скрыть
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {page === "hidden" && (
        <div className="admin__body">
          {hidden.length === 0 ? (
            <p className="admin__muted">Скрытого контента нет</p>
          ) : (
            <ul className="admin__list">
              {hidden.map((h) => (
                <li key={`${h.kind}-${h.id}`} className="admin__card">
                  <div className="admin__row-top">
                    <b>{hiddenKind(h.kind)}</b>
                    <span className="admin__row-meta">{timeAgoFromISO(h.created_at)}</span>
                  </div>
                  <p className="admin__row-sub">{clip(h.text || "—", 160)}</p>
                  <p className="admin__row-meta">{userLabel(h.author_name, undefined, h.author_user_id)}</p>
                  <div className="admin__actions">
                    <button type="button" className="admin__btn" onClick={() => void restoreHidden(h)} disabled={busy}>
                      Вернуть
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {page === "users" && (
        <div className="admin__body">
          <form
            className="admin__search"
            onSubmit={(e) => {
              e.preventDefault();
              void loadUsers(userQuery);
            }}
          >
            <input
              value={userQuery}
              onChange={(e) => setUserQuery(e.target.value)}
              placeholder="Имя, @ник или id"
            />
            <button type="submit">Найти</button>
          </form>
          {users.length === 0 ? (
            <p className="admin__muted">Никого не нашли</p>
          ) : (
            <ul className="admin__list">
              {users.map((u) => (
                <li key={u.user_id}>
                  <button type="button" className="admin__row" onClick={() => void openCard(u.user_id)}>
                    <div className="admin__row-top">
                      <b>{userLabel(u.display_name, u.username, u.user_id)}</b>
                      {u.is_deleted ? <span className="admin__badge admin__badge--muted">кикнут</span> : null}
                    </div>
                    <p className="admin__row-meta">
                      {u.cups} кубков · стрик {u.streak_days}
                      {u.has_active_paywall ? " · доступ есть" : ""}
                    </p>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {page === "card" && card && (
        <div className="admin__body">
          <div className="admin__card">
            <p className="admin__id">
              id {card.user_id}
              {card.username ? ` · @${card.username.replace(/^@/, "")}` : " · ника нет"}
            </p>
            <dl className="admin__dl">
              <div>
                <dt>Кубки</dt>
                <dd>{card.cups}</dd>
              </div>
              <div>
                <dt>Тренировок</dt>
                <dd>{card.workouts_total}</dd>
              </div>
              <div>
                <dt>Уровень</dt>
                <dd>
                  {card.level} · {card.level_name}
                </dd>
              </div>
              <div>
                <dt>Стрик</dt>
                <dd>
                  {card.streak_days} (рекорд {card.max_streak_days})
                </dd>
              </div>
              <div>
                <dt>Без тренировки</dt>
                <dd>{card.days_since_last_training < 0 ? "—" : `${card.days_since_last_training} дн.`}</dd>
              </div>
              <div>
                <dt>Больничный</dt>
                <dd>{card.sick_leave}</dd>
              </div>
              <div>
                <dt>Спасти стрик</dt>
                <dd>
                  {card.streak_save_attempts_avail}/{card.streak_save_attempts_max}
                </dd>
              </div>
              <div>
                <dt>Доступ</dt>
                <dd>{card.has_active_paywall ? "оплачен" : "нет"}</dd>
              </div>
              <div>
                <dt>В стае</dt>
                <dd>{card.is_deleted ? "кикнут" : "да"}</dd>
              </div>
            </dl>
            {card.ugc_muted_until ? <p className="admin__warn">UGC-мьют до {formatChatTime(card.ugc_muted_until)}</p> : null}
            <p className="admin__row-meta">Нарушений UGC: {card.ugc_violations}</p>
          </div>
          <div className="admin__actions admin__actions--stack">
            {card.sick_leave === "активен" || card.sick_leave === "ожидает" ? (
              <button type="button" className="admin__btn" disabled={busy} onClick={() => void actUser("sick_cancel", "Снять больничный?")}>
                Снять больничный
              </button>
            ) : (
              <button type="button" className="admin__btn" disabled={busy || card.is_deleted} onClick={() => void actUser("sick_set", "Оформить больничный этому участнику?")}>
                Выставить больничный
              </button>
            )}
            <button type="button" className="admin__btn" disabled={busy} onClick={() => void actUser("grant_save", "Выдать +1 попытку спасти стрик?")}>
              +1 спасти стрик
            </button>
            {card.ugc_muted_until ? (
              <button type="button" className="admin__btn" disabled={busy} onClick={() => void actUser("unmute", "Снять UGC-мьют?")}>
                Снять мьют
              </button>
            ) : (
              <button type="button" className="admin__btn" disabled={busy} onClick={() => void actUser("mute", "Замьютить публикации на 24 часа?")}>
                Мьют 24ч
              </button>
            )}
            {card.is_deleted ? (
              <>
                <button type="button" className="admin__btn" disabled={busy} onClick={() => void actUser("restore_full", "Вернуть в стаю с достижениями?")}>
                  Вернуть с прогрессом
                </button>
                <button type="button" className="admin__btn admin__btn--ghost" disabled={busy} onClick={() => void actUser("restore_scratch", "Вернуть в стаю с нуля?")}>
                  Вернуть с нуля
                </button>
              </>
            ) : (
              <button type="button" className="admin__btn admin__btn--danger" disabled={busy} onClick={() => setKickAsk(true)}>
                Удалить из стаи
              </button>
            )}
          </div>

          {kickAsk ? (
            <div className="admin__modal" role="dialog" aria-modal="true">
              <div className="admin__modal-box">
                <h3>Удалить из стаи?</h3>
                <p>
                  {userLabel(card.display_name, card.username, card.user_id)} потеряет доступ к ленте и чату. Кубки и
                  стрик сохранятся — вернуть можно кнопкой «Вернуть с прогрессом».
                </p>
                <div className="admin__modal-actions">
                  <button type="button" onClick={() => setKickAsk(false)}>
                    Отмена
                  </button>
                  <button
                    type="button"
                    className="admin__btn--danger"
                    disabled={busy}
                    onClick={() => {
                      setKickAsk(false);
                      void runUserAction("kick");
                    }}
                  >
                    Удалить
                  </button>
                </div>
              </div>
            </div>
          ) : null}
        </div>
      )}

      {page === "announce" && (
        <div className="admin__body">
          <div className="admin__voice" role="group" aria-label="От чьего имени">
            <button type="button" className={announceAuthor === "admin" ? "is-on" : ""} onClick={() => setAnnounceAuthor("admin")}>
              Админ
            </button>
            <button type="button" className={announceAuthor === "leo" ? "is-on" : ""} onClick={() => setAnnounceAuthor("leo")}>
              Лео
            </button>
          </div>
          <textarea
            className="admin__textarea"
            value={announce}
            onChange={(e) => setAnnounce(e.target.value)}
            placeholder="Текст объявления в ленту…"
            maxLength={4000}
            rows={7}
          />
          <button type="button" className="admin__btn admin__btn--wide" disabled={busy || !announce.trim()} onClick={() => void publish()}>
            Опубликовать
          </button>
        </div>
      )}

      {page === "price" && (
        <div className="admin__body">
          <p className="admin__muted admin__hint">
            Один платёж за вход в стаю и за возврат после удаления. Уже оплатившие не платят снова. Звёзды Telegram
            задаются отдельно на сервере.
          </p>
          <label className="admin__price">
            <span>Цена, ₽</span>
            <input
              type="number"
              inputMode="numeric"
              min={1}
              max={100000}
              step={1}
              value={priceInput}
              onChange={(e) => setPriceInput(e.target.value)}
            />
          </label>
          {price ? (
            <p className="admin__row-meta">
              Сейчас {price.amount_rub} ₽
              {price.is_custom ? " · задана админом" : " · значение сервера"}
              {price.is_custom ? ` · сброс вернёт ${price.default_amount_rub} ₽` : ""}
            </p>
          ) : (
            <p className="admin__muted">Загрузка…</p>
          )}
          <button type="button" className="admin__btn admin__btn--wide" disabled={busy || !priceInput.trim()} onClick={() => void savePrice()}>
            Сохранить
          </button>
          {price?.is_custom ? (
            <button type="button" className="admin__btn admin__btn--ghost admin__btn--wide" disabled={busy} onClick={() => void resetPrice()}>
              Сбросить к {price.default_amount_rub} ₽
            </button>
          ) : null}
        </div>
      )}
      <nav className="admin__tabs" aria-label="Разделы админки">
        {ADMIN_TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            className={`admin__tab${tab === t.key ? " on" : ""}`}
            aria-current={tab === t.key ? "page" : undefined}
            onClick={() => {
              setTab(t.key);
              setPage("home");
            }}
          >
            <span className="admin__tab-ico" aria-hidden>
              {t.icon}
            </span>
            <span className="admin__tab-label">{t.label}</span>
          </button>
        ))}
      </nav>
    </div>
  );
}

