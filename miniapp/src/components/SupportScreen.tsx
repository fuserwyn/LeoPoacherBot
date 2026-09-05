import { useCallback, useEffect, useRef, useState } from "react";
import { formatChatTime } from "../lib/timeAgo";
import "./ChatScreen.css";
import "./SupportScreen.css";

const envApi = (import.meta.env.VITE_MINIAPP_API_URL as string | undefined)?.replace(/\/$/, "") ?? "";

type Props = {
  initData: string;
  inTelegram: boolean;
  showAlert: (m: string) => void;
  onClose: () => void;
};

type SupportMsg = {
  uiKey: string;
  serverID?: number;
  role: "user" | "support";
  text: string;
  createdAt: string;
};

type ServerMsg = { id: number; role: "user" | "support"; text: string; created_at: string };

function nowId() {
  return `support-local-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

function mergeFromServer(prev: SupportMsg[], incoming: ServerMsg[]): SupportMsg[] {
  if (incoming.length === 0) return prev;
  const haveServerIDs = new Set<number>();
  for (const m of prev) {
    if (m.serverID) haveServerIDs.add(m.serverID);
  }
  const fresh = incoming.filter((m) => !haveServerIDs.has(m.id));
  const out = prev.slice();
  for (const sm of fresh) {
    if (sm.role === "user") {
      const idx = out.findIndex((m) => !m.serverID && m.role === "user" && m.text.trim() === sm.text.trim());
      if (idx >= 0) {
        out[idx] = {
          uiKey: `support-${sm.id}`,
          serverID: sm.id,
          role: "user",
          text: sm.text,
          createdAt: sm.created_at,
        };
        continue;
      }
    }
    out.push({
      uiKey: `support-${sm.id}`,
      serverID: sm.id,
      role: sm.role,
      text: sm.text,
      createdAt: sm.created_at,
    });
  }
  out.sort((a, b) => {
    if (a.serverID && b.serverID) return a.serverID - b.serverID;
    if (a.serverID && !b.serverID) return -1;
    if (!a.serverID && b.serverID) return 1;
    return new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
  });
  return out;
}

function maxServerID(items: SupportMsg[]): number {
  let max = 0;
  for (const m of items) {
    if (m.serverID && m.serverID > max) max = m.serverID;
  }
  return max;
}

export function SupportScreen({ initData, inTelegram, showAlert, onClose }: Props) {
  const sheetRef = useRef<HTMLDivElement>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  // Перед закрытием снимаем фокус: иначе app-keyboard-open зависает после свайпа вниз.
  const closeSheet = useCallback(() => {
    const el = document.activeElement;
    if (el instanceof HTMLElement) el.blur();
    onCloseRef.current();
  }, []);

  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
  const sendingRef = useRef(sending);
  sendingRef.current = sending;
  const [items, setItems] = useState<SupportMsg[]>([]);
  const [loaded, setLoaded] = useState(false);
  const logRef = useRef<HTMLDivElement | null>(null);
  const didInitialScrollRef = useRef(false);
  const forceScrollRef = useRef(false);

  useEffect(() => {
    const el = logRef.current;
    if (!el || !loaded) return;
    if (!didInitialScrollRef.current) {
      if (items.length === 0) return;
      didInitialScrollRef.current = true;
      requestAnimationFrame(() => {
        if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight;
      });
      return;
    }
    if (forceScrollRef.current) {
      forceScrollRef.current = false;
      el.scrollTop = el.scrollHeight;
      return;
    }
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 120;
    if (nearBottom) el.scrollTop = el.scrollHeight;
  }, [items, sending, loaded]);

  useEffect(() => {
    if (!envApi || !inTelegram || !initData?.trim()) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const fetchSlice = async (sinceID: number) => {
      try {
        const res = await fetch(`${envApi}/api/miniapp/support/feed`, {
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
        return;
      }
    };

    const loop = async () => {
      let sinceID = 0;
      while (!cancelled) {
        await fetchSlice(sinceID);
        if (cancelled) return;
        setLoaded(true);
        await new Promise<void>((r) => {
          setItems((prev) => {
            sinceID = Math.max(sinceID, maxServerID(prev));
            return prev;
          });
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

  const send = useCallback(async () => {
    const t = text.trim();
    if (!t || sending) return;
    if (!envApi) {
      showAlert("Не настроен API для поддержки.");
      return;
    }
    if (!inTelegram || !initData) {
      showAlert("Открой мини-апп из Telegram (нужен initData).");
      return;
    }
    forceScrollRef.current = true;
    setItems((prev) => [...prev, { uiKey: nowId(), role: "user", text: t, createdAt: new Date().toISOString() }]);
    setSending(true);
    setText("");
    try {
      const res = await fetch(`${envApi}/api/miniapp/support/send`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ init_data: initData, text: t }),
      });
      const j = (await res.json().catch(() => ({}))) as { error?: string; ok?: boolean };
      if (!res.ok || !j.ok) {
        showAlert(j.error ?? `Ошибка ${res.status}`);
      }
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Сеть");
    } finally {
      setSending(false);
    }
  }, [text, sending, inTelegram, initData, showAlert]);

  // Свайп шторки вниз = закрыть (как у формы тренировки).
  useEffect(() => {
    const sheet = sheetRef.current;
    const log = logRef.current;
    if (!sheet) return;
    let startY = 0;
    let dy = 0;
    let tracking = false;
    let dragging = false;

    const onStart = (e: TouchEvent) => {
      if (sendingRef.current) return;
      const t = e.touches[0];
      if (!t) return;
      const target = e.target as HTMLElement | null;
      if (target?.closest("textarea, input, select, [contenteditable='true']")) return;
      if (log?.contains(target) && (log?.scrollTop ?? 0) > 1) return;
      startY = t.clientY;
      dy = 0;
      dragging = false;
      tracking = true;
    };

    const onMove = (e: TouchEvent) => {
      if (!tracking) return;
      const t = e.touches[0];
      if (!t) return;
      dy = t.clientY - startY;
      if (!dragging) {
        if (dy > 14) {
          dragging = true;
          sheet.style.transition = "none";
        } else {
          if (dy < -8) tracking = false;
          return;
        }
      }
      e.preventDefault();
      sheet.style.transform = `translateY(${Math.max(0, dy)}px)`;
    };

    const onEnd = () => {
      if (!tracking) return;
      tracking = false;
      if (!dragging) return;
      dragging = false;
      if (dy > 96) {
        const active = document.activeElement;
        if (active instanceof HTMLElement) active.blur();
        sheet.style.transition = "transform 0.18s ease-in";
        sheet.style.transform = "translateY(105%)";
        window.setTimeout(() => onCloseRef.current(), 170);
      } else {
        sheet.style.transition = "transform 0.18s ease-out";
        sheet.style.transform = "";
      }
    };

    sheet.addEventListener("touchstart", onStart, { passive: true });
    sheet.addEventListener("touchmove", onMove, { passive: false });
    sheet.addEventListener("touchend", onEnd);
    sheet.addEventListener("touchcancel", onEnd);
    return () => {
      sheet.removeEventListener("touchstart", onStart);
      sheet.removeEventListener("touchmove", onMove);
      sheet.removeEventListener("touchend", onEnd);
      sheet.removeEventListener("touchcancel", onEnd);
    };
  }, []);

  return (
    <>
      <div
        className="sup-backdrop"
        aria-hidden="true"
        onClick={() => {
          if (!sending) closeSheet();
        }}
      />
      <div ref={sheetRef} className="sup" role="dialog" aria-modal="true" aria-label="Поддержка">
        <div className="sup__grabber" aria-hidden="true" />
        <div className="chat support-chat">
          <header className="chat__head">
            <div className="support-chat__head-avatar" aria-hidden>
              ?
            </div>
            <div className="chat__head-text">
              <h1 className="chat__title">Поддержка</h1>
              <p className="chat__sub">Здесь отвечает человек, не Лео</p>
            </div>
            <button type="button" className="sup__close" onClick={closeSheet} aria-label="Закрыть">
              ✕
            </button>
          </header>
          <div className="chat__log" role="log" aria-label="Сообщения поддержки" ref={logRef}>
            {loaded && items.length === 0 && (
              <div className="chat__row chat__row--sys">
                <div className="support-chat__bubble-avatar" aria-hidden>
                  ?
                </div>
                <div className="chat__bubble chat__bubble--sys">
                  Напиши в поддержку, если что-то сломалось, нужна помощь с оплатой или вопрос не для Лео.
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
                  <div className="support-chat__bubble-avatar" aria-hidden>
                    ?
                  </div>
                  <div className="chat__bubble-wrap chat__bubble-wrap--sys">
                    <div className="chat__bubble chat__bubble--sys">{m.text}</div>
                    <div className="chat__time chat__time--sys">{formatChatTime(m.createdAt)}</div>
                  </div>
                </div>
              ),
            )}
          </div>
          <form
            className="chat__form"
            onSubmit={(e) => {
              e.preventDefault();
              void send();
            }}
          >
            <input
              className="chat__input"
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder="Сообщение в поддержку…"
              maxLength={4000}
              autoComplete="off"
              enterKeyHint="send"
            />
            <button type="submit" className="chat__send" disabled={sending || !text.trim()}>
              {sending ? "…" : "➤"}
            </button>
          </form>
        </div>
      </div>
    </>
  );
}
