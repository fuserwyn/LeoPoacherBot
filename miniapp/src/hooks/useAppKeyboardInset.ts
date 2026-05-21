import { useEffect } from "react";

function isTextEntry(el: Element | null): boolean {
  if (!el || !(el instanceof HTMLElement)) return false;
  const tag = el.tagName;
  if (tag === "TEXTAREA") return true;
  if (tag === "INPUT") {
    const type = (el as HTMLInputElement).type;
    return !["button", "submit", "reset", "checkbox", "radio", "file", "hidden", "image"].includes(
      type,
    );
  }
  return el.isContentEditable;
}

/**
 * Глобально: при фокусе в поле ввода таббар остаётся на bottom:0, но скрыт (не едет над клавиатурой).
 * --app-keyboard-bottom — для форм ввода (чат, поддержка), чтобы сидеть над клавиатурой.
 */
export function useAppKeyboardInset() {
  useEffect(() => {
    const root = document.documentElement;
    let debounce: ReturnType<typeof setTimeout> | undefined;
    let textFocused = false;

    const readKeyboard = () => {
      const vv = window.visualViewport;
      const layoutH = Math.max(320, Math.floor(window.innerHeight || 320));
      const visualH = Math.floor(vv?.height ?? layoutH);
      const offsetTop = Math.floor(vv?.offsetTop ?? 0);
      const kb = Math.max(0, layoutH - visualH - offsetTop);
      const px = `${kb}px`;
      root.style.setProperty("--app-keyboard-bottom", px);
      root.style.setProperty("--feed-keyboard-bottom", px);
    };

    const scheduleRead = () => {
      if (debounce) window.clearTimeout(debounce);
      debounce = window.setTimeout(readKeyboard, 80);
    };

    const syncOpenClass = () => {
      root.classList.toggle("app-keyboard-open", textFocused);
    };

    const onFocusIn = (e: FocusEvent) => {
      if (!isTextEntry(e.target as Element)) return;
      textFocused = true;
      syncOpenClass();
      readKeyboard();
    };

    const onFocusOut = () => {
      window.setTimeout(() => {
        if (isTextEntry(document.activeElement)) return;
        textFocused = false;
        syncOpenClass();
      }, 80);
    };

    readKeyboard();
    syncOpenClass();

    document.addEventListener("focusin", onFocusIn);
    document.addEventListener("focusout", onFocusOut);

    const vv = window.visualViewport;
    vv?.addEventListener("resize", scheduleRead);
    vv?.addEventListener("scroll", scheduleRead);
    window.addEventListener("orientationchange", readKeyboard);

    const tg = window.Telegram?.WebApp as { onEvent?: (e: string, fn: () => void) => void } | undefined;
    tg?.onEvent?.("viewportChanged", scheduleRead);

    return () => {
      if (debounce) window.clearTimeout(debounce);
      document.removeEventListener("focusin", onFocusIn);
      document.removeEventListener("focusout", onFocusOut);
      vv?.removeEventListener("resize", scheduleRead);
      vv?.removeEventListener("scroll", scheduleRead);
      window.removeEventListener("orientationchange", readKeyboard);
      const tgOff = window.Telegram?.WebApp as { offEvent?: (e: string, fn: () => void) => void } | undefined;
      tgOff?.offEvent?.("viewportChanged", scheduleRead);
      root.classList.remove("app-keyboard-open");
      root.style.removeProperty("--app-keyboard-bottom");
      root.style.removeProperty("--feed-keyboard-bottom");
    };
  }, []);
}
