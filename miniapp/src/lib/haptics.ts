/**
 * Тактильная отдача через Telegram WebApp HapticFeedback.
 * Вне Telegram / на старых клиентах — тихий no-op (метод может отсутствовать).
 */
type NotificationType = "error" | "success" | "warning";
type ImpactStyle = "light" | "medium" | "heavy" | "rigid" | "soft";

function haptic() {
  return window.Telegram?.WebApp?.HapticFeedback;
}

/** Короткий удар (нажатия, действия). */
export function hapticImpact(style: ImpactStyle = "medium") {
  try {
    haptic()?.impactOccurred?.(style);
  } catch {
    /* no-op — не все клиенты поддерживают HapticFeedback */
  }
}

/** Уведомление об исходе операции (успех/ошибка/предупреждение). */
export function hapticNotification(type: NotificationType) {
  try {
    haptic()?.notificationOccurred?.(type);
  } catch {
    /* no-op */
  }
}
