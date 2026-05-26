/** Смещение fixed-элементов (поле ввода) от низа layout viewport до верха таббара. */
export function measureBottomNavOffset(navEl: HTMLElement): number {
  const rect = navEl.getBoundingClientRect();
  const layoutH = Math.max(320, Math.floor(window.innerHeight || 320));
  const stackFromNavTop = Math.max(0, Math.ceil(layoutH - rect.top));

  const tg = window.Telegram?.WebApp;
  if (tg?.platform !== "android") {
    return stackFromNavTop;
  }

  // Android TG WebView: innerHeight − nav.top часто завышен (плавающая «+» в таббаре).
  // Привязываемся к ряду вкладок, а не к верху bbox навигации.
  const tabTops: number[] = [];
  navEl.querySelectorAll<HTMLElement>(".bottom-nav__item").forEach((item) => {
    tabTops.push(item.getBoundingClientRect().top);
  });
  if (tabTops.length === 0) {
    return stackFromNavTop;
  }

  const tabRowTop = Math.min(...tabTops);
  const stackFromTabRow = Math.max(0, Math.ceil(layoutH - tabRowTop));
  return Math.min(stackFromNavTop, stackFromTabRow);
}
