/** Прокрутка внутренней ленты чата — без scrollIntoView, чтобы не дёргать жест. */
export function scrollChatLogToEnd(log: HTMLElement) {
  log.scrollTop = log.scrollHeight;
}

export function isChatLogNearBottom(log: HTMLElement, threshold = 80): boolean {
  return log.scrollHeight - log.scrollTop - log.clientHeight < threshold;
}

/** Coalesce частых scrollToEnd (ResizeObserver, поллинг) в один кадр. */
export function createChatScrollScheduler(onScroll: () => void) {
  let raf = 0;
  return () => {
    if (raf) return;
    raf = requestAnimationFrame(() => {
      raf = 0;
      onScroll();
    });
  };
}
