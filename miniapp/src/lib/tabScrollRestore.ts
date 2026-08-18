/** Запоминаем и возвращаем window-скролл вкладки: display:none сжимает документ
 *  и браузер успевает обрезать scrollY, если читать его уже после скрытия. */

export function captureScrollY(): number {
  return window.scrollY || document.documentElement.scrollTop || document.body.scrollTop || 0;
}

export function applyScrollY(y: number): void {
  const top = Math.max(0, Number.isFinite(y) ? y : 0);
  window.scrollTo(0, top);
  document.documentElement.scrollTop = top;
  document.body.scrollTop = top;
}

/** Ключ фильтров ленты — смена любого измерения = новый состав списка. */
export function feedFilterEpoch(
  scope: string,
  type: string,
  categoryIds: readonly string[],
): string {
  return `${scope}|${type}|${categoryIds.join(",")}`;
}
