import { useCallback, useEffect, useRef, useState } from "react";
import { formatChatTime, timeAgoFromISO } from "../lib/timeAgo";
import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import "./PackGroupChatPanel.css";

const apiBase = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

export type PackGroupMessage = {
  id: number;
  user_id: number;
  username: string;
  text: string;
  created_at: string;
  is_leo: boolean;
  author_photo_url?: string;
};

type Props = {
  initData: string;
  inTelegram: boolean;
  meId: number;
  showAlert: (m: string) => void;
  onHaptic?: () => void;
};

export function PackGroupChatPanel({ initData, inTelegram, meId, showAlert, onHaptic }: Props) {
  const [items, setItems] = useState<PackGroupMessage[]>([]);
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const endRef = useRef<HTMLDivElement | null>(null);
  const logRef = useRef<HTMLDivElement | null>(null);
  /** true когда пользователь прокрутил вверх — не перебиваем позицию при поллинге */
  const userScrolledUpRef = useRef(false);
  const initialLoadedRef = useRef(false);

  const load = useCallback(async () => {
    if (!apiBase || !inTelegram || !initData) return;
    setErr(null);
    try {
      const res = await fetch(`${apiBase}/api/miniapp/pack-group/feed`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData }),
      });
      const j = (await res.json().catch(() => ({}))) as { ok?: boolean; messages?: PackGroupMessage[]; error?: string };
      if (!res.ok) {
        if (res.status === 403) {
          setErr("Нет доступа к чату стаи.");
          setItems([]);
          return;
        }
        setErr(j.error ?? `Ошибка ${res.status}`);
        return;
      }
      setItems(j.messages ?? []);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Сеть");
    }
  }, [inTelegram, initData]);

  useEffect(() => {
    void load();
  }, [load]);

  // Как в ChatScreen: без body--lock iOS/WebView подтягивает страницу к фокусу — поле «плывёт» вверх.
  useEffect(() => {
    document.body.classList.add("body--lock");
    return () => document.body.classList.remove("body--lock");
  }, []);

  useEffect(() => {
    if (!apiBase || !inTelegram || !initData) return;
    const t = setInterval(() => void load(), 5000);
    return () => clearInterval(t);
  }, [load, inTelegram, initData]);

  // Отслеживаем, прокрутил ли пользователь вверх, чтобы не перебивать позицию при поллинге.
  useEffect(() => {
    const el = logRef.current;
    if (!el) return;
    const onScroll = () => {
      const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
      userScrolledUpRef.current = !nearBottom;
    };
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => el.removeEventListener("scroll", onScroll);
  }, []);

  useEffect(() => {
    const el = logRef.current;
    if (!el || items.length === 0) return;
    if (!initialLoadedRef.current) {
      // Первая загрузка — сразу на последнее сообщение (без анимации).
      el.scrollTop = el.scrollHeight;
      initialLoadedRef.current = true;
      return;
    }
    // Поллинг: скроллим только если пользователь и так в конце.
    if (!userScrolledUpRef.current) {
      endRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [items]);

  const send = useCallback(async () => {
    const t = text.trim();
    if (!t || sending) return;
    if (!apiBase) {
      showAlert("Сборка без VITE_MINIAPP_API_URL.");
      return;
    }
    if (!inTelegram || !initData) {
      showAlert("Открой мини-апп из Telegram.");
      return;
    }
    setSending(true);
    onHaptic?.();
    setText("");
    // После отправки пользователь хочет видеть своё сообщение внизу.
    userScrolledUpRef.current = false;
    try {
      const res = await fetch(`${apiBase}/api/miniapp/pack-group/messages`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData, text: t }),
      });
      const j = (await res.json().catch(() => ({}))) as { error?: string; ok?: boolean; reply_text?: string };
      if (!res.ok) {
        showAlert(j.error ?? `Ошибка ${res.status}`);
        return;
      }
      await load();
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
    } finally {
      setSending(false);
    }
  }, [text, sending, inTelegram, initData, showAlert, load, onHaptic]);

  const removeMine = useCallback(
    async (messageID: number) => {
      if (!apiBase || !inTelegram || !initData || !messageID) return;
      try {
        const res = await fetch(`${apiBase}/api/miniapp/pack-group/messages/delete`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ init_data: initData, message_id: messageID }),
        });
        const j = (await res.json().catch(() => ({}))) as { error?: string };
        if (!res.ok) {
          showAlert(j.error ?? `Ошибка ${res.status}`);
          return;
        }
        setItems((prev) => prev.filter((m) => m.id !== messageID));
      } catch (e) {
        showAlert(e instanceof Error ? e.message : "Сеть");
      }
    },
    [inTelegram, initData, showAlert],
  );

  if (!apiBase) {
    return <p className="packroom__warn muted">Нет API URL в билде.</p>;
  }

  return (
    <div className="packroom">
      <p className="packroom__hint">
        Общий чат стаи живёт здесь, в мини-аппе. Лео отвечает, если написать <strong>@leo</strong> или
        <strong> @…</strong> (username бота); без @ — без ответа ИИ.
      </p>
      {err && <p className="packroom__err">{err}</p>}
      <div className="packroom__log" role="log" aria-label="Чат стаи" ref={logRef}>
        {items.map((m) => {
          const mine = !m.is_leo && m.user_id === meId;
          return (
            <div
              key={m.id}
              className={`packroom__row ${m.is_leo ? "packroom__row--leo" : mine ? "packroom__row--me" : "packroom__row--oth"}`}
            >
              <div className="packroom__row-inner">
                <div className="packroom__ava" aria-hidden>
                  {m.is_leo ? (
                    <img className="packroom__avatar" src={LEO_AVATAR_URL} width={32} height={32} alt="" loading="lazy" />
                  ) : m.author_photo_url ? (
                    <img className="packroom__avatar" src={m.author_photo_url} width={32} height={32} alt="" loading="lazy" />
                  ) : (
                    <span className="packroom__avatar-ph">🐾</span>
                  )}
                </div>
                <div className="packroom__content">
                  <div className="packroom__meta">
                    {m.is_leo ? "Лео" : m.username} · {formatChatTime(m.created_at)} · {timeAgoFromISO(m.created_at)}
                  </div>
                  <div className="packroom__bubble-wrap">
                    <div className="packroom__bubble">{m.text}</div>
                    {mine && (
                      <button
                        type="button"
                        className="packroom__del"
                        onClick={() => {
                          if (!window.confirm("Удалить сообщение?")) return;
                          void removeMine(m.id);
                        }}
                      >
                        Удалить
                      </button>
                    )}
                  </div>
                </div>
              </div>
            </div>
          );
        })}
        <div ref={endRef} />
      </div>
      <form
        className="packroom__form"
        onSubmit={(e) => {
          e.preventDefault();
          void send();
        }}
      >
        <input
          className="packroom__input"
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Сообщение… @leo — чтобы ответил бот"
          maxLength={4000}
          autoComplete="off"
          onFocus={() => {
            window.setTimeout(() => {
              logRef.current?.scrollTo({ top: logRef.current.scrollHeight, behavior: "smooth" });
            }, 300);
          }}
        />
        <button type="submit" className="packroom__send" disabled={sending || !text.trim()}>
          {sending ? "…" : "➤"}
        </button>
      </form>
    </div>
  );
}
