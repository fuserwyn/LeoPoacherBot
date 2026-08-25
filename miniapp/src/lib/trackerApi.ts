// ... существующие импорты ...

export async function trackerDonateStars(
  initData: string,
  taskId: number,
  stars: number
): Promise<{ ok: boolean }> {
  const res = await fetch(`/api/tracker/${taskId}/donate`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Telegram-Init-Data": initData,
    },
    body: JSON.stringify({ stars }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

// ... остальной код файла ...