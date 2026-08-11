/** В поле `username` ленты для юзеров с TG-ником лежит «@nick»,
 *  иначе — отображаемое имя (имя/фамилия). */

/** TG-ник без «@» из username ленты; undefined, если это не похоже на настоящий @username. */
export function feedTgUsername(raw: string | undefined): string | undefined {
  const t = (raw ?? "").trim();
  if (!t.startsWith("@")) return undefined;
  const nick = t.slice(1);
  return /^[a-zA-Z0-9_]{4,32}$/.test(nick) ? nick : undefined;
}
