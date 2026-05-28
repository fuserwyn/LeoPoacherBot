/** Смещение fixed-контейнеров (чат) от низа видимой области до верха таббара. */
export function measureBottomNavOffset(navEl: HTMLElement): number {
  const rect = navEl.getBoundingClientRect();
  const vv = window.visualViewport;
  // rect.height иногда занижен на Android; innerHeight завышен — считаем по
  // visualViewport: расстояние от верха nav до низа видимой WebView-области.
  if (vv) {
    const visualBottom = vv.height + (vv.offsetTop ?? 0);
    return Math.max(Math.ceil(rect.height), Math.ceil(visualBottom - rect.top));
  }
  return Math.max(Math.ceil(rect.height), Math.ceil(window.innerHeight - rect.top));
}
