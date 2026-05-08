import { useCallback, useEffect, useRef, useState } from "react";
import { drainLeoPersonalInbox } from "../lib/leoPersonalInbox";
import { formatChatTime } from "../lib/timeAgo";
import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import "./ChatScreen.css";

const envApi = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

type Props = {
  name: string;
  initData: string;
  inTelegram: boolean;
  showAlert: (m: string) => void;
  /** После забора очереди лички с сервера (для сброса бейджа). */
  onInboxDrained?: () => void;
};

// Сообщение в локальном UI: серверное (с числовым id и ISO created_at) и
// оптимистичное (id вида "local-..." без серверного id; ждёт следующий polling,
// после которого замещается серверным дубликатом). Источник правды — сервер
// (/api/miniapp/personal-chat/feed): локальный кеш живёт только до replace.
type ChatMsg = {
  uiKey: string;
  serverID?: number;
  role: "user" | "leo";
  text: string;
  // ISO-строка от сервера или ISO от Date.now() для оптимистичных.
  createdAt: string;
  likeCount?: number;
  likeMe?: boolean;
};

type ServerMsg = { id: number; role: "user" | "leo"; text: string; created_at: string; like_count?: number; like_me?: boolean };

function nowId() {
  return `local-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

// Подмерживает серверную пачку в текущий список:
//   • удаляет локальные оптимистичные user-сообщения, чьи (text, role) совпали с пришедшими;
//   • добавляет новые серверные.
function mergeFromServer(prev: ChatMsg[], incoming: ServerMsg[]): ChatMsg[] {
  if (incoming.length === 0) return prev;
  const haveServerIDs = new Set<number>();
  for (const m of prev) {
    if (m.serverID) haveServerIDs.add(m.serverID);
  }
  const fresh = incoming.filter((m) => !haveServerIDs.has(m.id));
  const updated = prev.map((p) => {
    if (!p.serverID) return p;
    const sm = incoming.find((x) => x.id === p.serverID);
    if (!sm) return p;
    return { ...p, likeCount: sm.like_count ?? 0, likeMe: Boolean(sm.like_me) };
  });
  if (fresh.length === 0) return updated;
  // Сматчим оптимистичные user-сообщения по тексту (порядок прилёта совпадает).
  const out = updated.slice();
  for (const sm of fresh) {
    if (sm.role === "user") {
      const idx = out.findIndex(
        (m) => !m.serverID && m.role === "user" && m.text.trim() === sm.text.trim()
      );
      if (idx >= 0) {
        out[idx] = {
          uiKey: `s-${sm.id}`,
          serverID: sm.id,
          role: "user",
          text: sm.text,
          createdAt: sm.created_at,
          likeCount: sm.like_count ?? 0,
          likeMe: Boolean(sm.like_me),
        };
        continue;
      }
    }
    out.push({
      uiKey: `s-${sm.id}`,
      serverID: sm.id,
      role: sm.role,
      text: sm.text,
      createdAt: sm.created_at,
      likeCount: sm.like_count ?? 0,
      likeMe: Boolean(sm.like_me),
    });
  }
  // Сортировка по серверному id (где есть), потом по времени; локальные без id — в конец.
  out.sort((a, b) => {
    if (a.serverID && b.serverID) return a.serverID - b.serverID;
    if (a.serverID && !b.serverID) return -1;
    if (!a.serverID && b.serverID) return 1;
    return new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
  });
  return out;
}

// Максимальный серверный id в списке — для since_id-инкрементального запроса.
function maxServerID(items: ChatMsg[]): number {
  let max = 0;
  for (const m of items) {
    if (m.serverID && m.serverID > max) max = m.serverID;
  }
  return max;
}

export function ChatScreen({ name, initData, inTelegram, showAlert, onInboxDrained }: Props) {
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  /** После POST /messages: ответ ИИ пишется асинхронно — «печатает» до появления строки Лео в фиде. */
  const [leoTyping, setLeoTyping] = useState(false);
  const [items, setItems] = useState<ChatMsg[]>([]);
  const [loaded, setLoaded] = useState(false);
  const logRef = useRef<HTMLDivElement | null>(null);
  const endRef = useRef<HTMLDivElement | null>(null);
  const didInitialScrollRef = useRef(false);
  /** true — после отправки сообщения пользователем: всегда скроллим вниз, даже если был наверху */
  const forceScrollRef = useRef(false);
  /** max(server_id) перед отправкой пользователя — новый ответ Лео с id выше этого. */
  const baselineMaxForPendingLeoRef = useRef(0);
  /** Высота видимой области (ужимается при открытии клавиатуры) */
  const [chatH, setChatH] = useState<number | undefined>(undefined);
  /** bottom-offset для формы ввода: undefined = CSS-дефолт (над BottomNav), число = над клавиатурой */
  const [formBottom, setFormBottom] = useState<number | undefined>(undefined);

  // visualViewport даёт реальную высоту видимой зоны — работает и на iOS WKWebView.
  useEffect(() => {
    const vv = window.visualViewport;
    if (!vv) return;
    const update = () => {
      const kbH = Math.max(0, Math.floor(window.innerHeight - vv.height - vv.offsetTop));
      setChatH(Math.floor(vv.height) - 8);
      // Клавиатура открыта (>50px) — позиционируем форму прямо над ней.
      setFormBottom(kbH > 50 ? kbH + 4 : undefined);
    };
    vv.addEventListener("resize", update);
    update();
    return () => vv.removeEventListener("resize", update);
  }, []);

  useEffect(() => {
    const el = logRef.current;
    if (!el || !loaded) return;
    if (!didInitialScrollRef.current) {
      el.scrollTop = el.scrollHeight;
      didInitialScrollRef.current = true;
      return;
    }
    // Пользователь отправил сообщение — всегда показываем его, даже если был наверху.
    if (forceScrollRef.current) {
      forceScrollRef.current = false;
      el.scrollTop = el.scrollHeight;
      return;
    }
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 120;
    if (nearBottom) {
      el.scrollTop = el.scrollHeight;
    }
  }, [items, sending, leoTyping, loaded]);

  useEffect(() => {
    if (!leoTyping) return;
    const baseline = baselineMaxForPendingLeoRef.current;
    const hasNewLeo = items.some(
      (m) => m.role === "leo" && m.serverID !== undefined && m.serverID > baseline
    );
    if (hasNewLeo) setLeoTyping(false);
  }, [items, leoTyping]);

  /** Если ответ так и не пришёл из БД — не держать «печатает» бесконечно. */
  useEffect(() => {
    if (!leoTyping) return;
    const timer = window.setTimeout(() => setLeoTyping(false), 180_000);
    return () => window.clearTimeout(timer);
  }, [leoTyping]);

  // Первая загрузка истории + инкрементальный polling каждые 3 секунды.
  // Источник правды — БД на сервере (см. /api/miniapp/personal-chat/feed).
  // Это даёт синхронизацию между всеми устройствами одного юзера (Telegram
  // Desktop / iPhone и т.д.).
  useEffect(() => {
    if (!envApi || !inTelegram || !initData?.trim()) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const fetchSlice = async (sinceID: number) => {
      try {
        const res = await fetch(`${envApi}/api/miniapp/personal-chat/feed`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, since_id: sinceID }),
        });
        if (!res.ok) return;
        const j = (await res.json().catch(() => ({}))) as { ok?: boolean; messages?: ServerMsg[] };
        if (cancelled) return;
        const incoming = j.messages ?? [];
        if (incoming.length > 0) {
          setItems((prev) => mergeFromServer(prev, incoming));
        }
      } catch {
        // тихо: следующая итерация цикла исправит, если сеть моргнула
      }
    };

    const loop = async () => {
      // Первое чтение — since_id=0 (последние ~200 сообщений).
      // Дальше — since_id = max(server_id) уже виденных, инкрементально.
      let sinceID = 0;
      while (!cancelled) {
        await fetchSlice(sinceID);
        if (cancelled) return;
        setLoaded(true);
        // Берём актуальный max через функциональный setState.
        await new Promise<void>((r) => {
          setItems((prev) => {
            sinceID = Math.max(sinceID, maxServerID(prev));
            return prev;
          });
          // даём React отрисовать
          timer = setTimeout(() => r(), 0);
        });
        await new Promise<void>((r) => {
          timer = setTimeout(r, 3000);
        });
      }
    };
    void loop();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [inTelegram, initData]);

  /** Дополнительно дёрнуть очередь поллинга-warning'ов: бейдж в табах сбрасываем. */
  useEffect(() => {
    if (!envApi || !inTelegram || !initData?.trim()) return;
    let cancelled = false;
    (async () => {
      const parts = await drainLeoPersonalInbox(initData);
      if (cancelled) return;
      // Сами тексты подгружаются через основной poll feed (Лео туда же сохраняет).
      // Здесь только сброс бейджа.
      if (parts.length > 0) {
        // ничего не добавляем в state — основной поллер подтянет
      }
      onInboxDrained?.();
    })();
    return () => {
      cancelled = true;
    };
  }, [inTelegram, initData, onInboxDrained]);

  const send = useCallback(async () => {
    const t = text.trim();
    if (!t || sending) return;
    if (!envApi) {
      showAlert(
        "Сборка без API: в Railway у сервисе мини-аппа задай Build Variable VITE_MINIAPP_API_URL = публичный https URL сервиса с ботом (ms_leo), затем Redeploy."
      );
      return;
    }
    if (!inTelegram || !initData) {
      showAlert("Открой мини-апп из Telegram (нужен initData).");
      return;
    }

    forceScrollRef.current = true;
    /** Макс. server id до оптимистичного сообщения — так отличим новый ответ Лео от старых. */
    let baselineMaxBeforeSend = 0;
    setItems((prev) => {
      baselineMaxBeforeSend = maxServerID(prev);
      return [
        ...prev,
        { uiKey: nowId(), role: "user", text: t, createdAt: new Date().toISOString() },
      ];
    });

    setSending(true);
    setText("");
    try {
      const w = window.Telegram?.WebApp;
      w?.HapticFeedback?.impactOccurred?.("light");
      const res = await fetch(`${envApi}/api/miniapp/messages`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData, text: t }),
      });
      const j = (await res.json().catch(() => ({}))) as {
        error?: string;
        ok?: boolean;
        pending?: boolean;
        reply_text?: string;
      };
      if (!res.ok) {
        showAlert(j.error ?? `Ошибка ${res.status}`);
        setLeoTyping(false);
        return;
      }
      baselineMaxForPendingLeoRef.current = baselineMaxBeforeSend;
      setLeoTyping(true);
      // Основной поллер фида подхватит юзер-сообщение (с серверным id) и ответ Лео из БД — тогда items → useEffect выше снимает leoTyping.
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
      setLeoTyping(false);
    } finally {
      setSending(false);
    }
  }, [text, sending, inTelegram, initData, showAlert]);

  const toggleLike = useCallback(
    async (messageID?: number) => {
      if (!messageID || !envApi || !inTelegram || !initData) return;
      try {
        const res = await fetch(`${envApi}/api/miniapp/personal-chat/like`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, message_id: messageID }),
        });
        if (!res.ok) return;
      } catch {
        return;
      }
      setItems((prev) =>
        prev.map((m) =>
          m.serverID === messageID
            ? {
                ...m,
                likeMe: !m.likeMe,
                likeCount: Math.max(0, (m.likeCount ?? 0) + (m.likeMe ? -1 : 1)),
              }
            : m,
        ),
      );
    },
    [inTelegram, initData],
  );

  const showTypingCue = sending || leoTyping;

  const chatStyle = chatH != null
    ? { height: chatH, maxHeight: chatH, minHeight: chatH }
    : undefined;
  const formStyle = formBottom != null ? { bottom: formBottom } : undefined;

  return (
    <div className="chat" style={chatStyle}>
      {!import.meta.env.VITE_MINIAPP_API_URL && (
        <div className="chat__configwarn" role="status">
          Нет <code className="chat__code">VITE_MINIAPP_API_URL</code> при сборке. В Railway → сервис
          <strong> miniapp</strong> → Variables → <strong>Build</strong> → укажи https URL сервиса
          <strong> бота</strong> (ms_leo), Redeploy.
        </div>
      )}
      <header className="chat__head">
        <div className="chat__head-avatarwrap">
          <img className="chat__head-avatar" src={LEO_AVATAR_URL} width={52} height={52} alt="Лео" loading="eager" />
          {showTypingCue && (
            <span className="chat__head-typing" aria-hidden="true">
              <span className="chat__head-typing-dots">
                <span className="chat__dot" />
                <span className="chat__dot" />
                <span className="chat__dot" />
              </span>
            </span>
          )}
        </div>
        <div className="chat__head-text">
          <h1 className="chat__title">Лео</h1>
          <p className="chat__sub">{showTypingCue ? "печатает…" : name}</p>
        </div>
      </header>
      <div className="chat__log" role="log" aria-label="Сообщения с ботом" ref={logRef}>
        {loaded && items.length === 0 && (
          <div className="chat__row chat__row--sys">
            <img className="chat__bubble-avatar" src={LEO_AVATAR_URL} width={36} height={36} alt="" aria-hidden="true" />
            <div className="chat__bubble chat__bubble--sys">
              Привет! Здесь личный чат с Лео — ИИ отвечает на любой текст, есть #training_done и /start. История
              синхронизируется между всеми твоими устройствами.
            </div>
          </div>
        )}
        {items.map((m) =>
          m.role === "user" ? (
            <div key={m.uiKey} className="chat__row chat__row--user">
              <div className="chat__bubble-wrap chat__bubble-wrap--user">
                <div className="chat__bubble chat__bubble--user">{m.text}</div>
                <div className="chat__time chat__time--user">{formatChatTime(m.createdAt)}</div>
              </div>
            </div>
          ) : (
            <div key={m.uiKey} className="chat__row chat__row--sys">
              <img className="chat__bubble-avatar" src={LEO_AVATAR_URL} width={36} height={36} alt="" aria-hidden="true" />
              <div className="chat__bubble-wrap chat__bubble-wrap--sys">
                <div className="chat__bubble chat__bubble--sys">{m.text}</div>
                <div className="chat__meta">
                  <div className="chat__time chat__time--sys">{formatChatTime(m.createdAt)}</div>
                  {m.serverID != null && (
                    <button
                      type="button"
                      className={`chat__like${m.likeMe ? " chat__like--mine" : ""}`}
                      onClick={() => void toggleLike(m.serverID)}
                    >
                      ❤️ {m.likeCount ?? 0}
                    </button>
                  )}
                </div>
              </div>
            </div>
          )
        )}
        {showTypingCue && (
          <div className="chat__row chat__row--sys" role="status" aria-live="polite" aria-label="Лео печатает">
            <img className="chat__bubble-avatar" src={LEO_AVATAR_URL} width={36} height={36} alt="" aria-hidden="true" />
            <div className="chat__bubble chat__bubble--sys chat__bubble--typing" aria-hidden="true">
              <span className="chat__typing-dots">
                <span className="chat__dot" />
                <span className="chat__dot" />
                <span className="chat__dot" />
              </span>
            </div>
          </div>
        )}
        <div ref={endRef} />
      </div>
      <form
        className="chat__form"
        style={formStyle}
        onSubmit={(e) => {
          e.preventDefault();
          void send();
        }}
      >
        <input
          className="chat__input"
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Сообщение…"
          maxLength={4000}
          autoComplete="off"
          enterKeyHint="send"
        />
        <button type="submit" className="chat__send" disabled={sending || !text.trim()}>
          {sending ? "…" : "➤"}
        </button>
      </form>
    </div>
  );
}
