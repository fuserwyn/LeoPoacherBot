/** Подтверждение действия модалкой. В Telegram WebView нативный window.confirm часто
 *  не показывается (блокируется), поэтому используем Telegram WebApp.showConfirm, а вне
 *  Telegram — обычный confirm как фоллбэк. Возвращает true, если пользователь подтвердил. */
type TgWebApp = {
  showConfirm?: (message: string, callback: (ok: boolean) => void) => void;
};

export function tgConfirm(message: string): Promise<boolean> {
  const wa = (window as unknown as { Telegram?: { WebApp?: TgWebApp } }).Telegram?.WebApp;
  if (wa?.showConfirm) {
    return new Promise<boolean>((resolve) => {
      try {
        wa.showConfirm!(message, (ok) => resolve(Boolean(ok)));
      } catch {
        resolve(typeof window.confirm === "function" ? window.confirm(message) : false);
      }
    });
  }
  return Promise.resolve(typeof window.confirm === "function" ? window.confirm(message) : false);
}
