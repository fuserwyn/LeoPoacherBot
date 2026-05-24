import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import "./MiniappRemovedScreen.css";

/** Нет оплаты / нет доступа в стаю — закрытый мини-апп, вход через бота. */
export function MiniappLockedScreen() {
  return (
    <div className="miniapp-removed" role="status" aria-live="polite">
      <div className="miniapp-removed__inner">
        <img className="miniapp-removed__avatar" src={LEO_AVATAR_URL} width={104} height={104} alt="Лео" loading="eager" />
        <p className="miniapp-removed__eyebrow">Вход в стаю</p>
        <h1 className="miniapp-removed__title">Сначала оплата в боте</h1>
        <p className="miniapp-removed__text">
          Доступ к мини-приложению открывается после оплаты. Закрой мини-апп, напиши боту в личку{" "}
          <strong>/start</strong> и выбери способ оплаты.
        </p>
      </div>
    </div>
  );
}
