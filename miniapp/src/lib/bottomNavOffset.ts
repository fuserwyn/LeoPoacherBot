/** Смещение fixed-элементов (поле ввода) от низа layout viewport до верха таббара. */
export function measureBottomNavOffset(navEl: HTMLElement): number {
  // Таббар приклеен к bottom:0, поэтому композеру нужно подняться ровно на его
  // отрисованную высоту. rect.height одинаково корректен на iOS и Android и не
  // зависит от innerHeight, который Android TG WebView часто завышает (из-за чего
  // прежняя формула innerHeight − nav.top уводила поле ввода под таббар).
  const rect = navEl.getBoundingClientRect();
  return Math.max(0, Math.ceil(rect.height));
}
