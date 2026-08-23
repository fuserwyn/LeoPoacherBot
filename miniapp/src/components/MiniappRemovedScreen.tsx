import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import "./MiniappRemovedScreen.css";

export function MiniappRemovedScreen() {
  return (
    <div className="miniapp-removed" role="status" aria-live="polite">
      <div className="miniapp-removed__inner">
        <img className="miniapp-removed__avatar" src={LEO_AVATAR_URL} width={104} height={104} alt="Лео" loading="eager" />
        <p className="miniapp-removed__eyebrow">Лео уже написал тебе в бот.</p>
        <h1 className="miniapp-removed__title">Как хорошо, что ты не занимался.</h1>
        <p className="miniapp-removed__text">
          Тебя убрали из стаи за неактивность. Дальше работает логика повторного входа через личку с ботом.
        </p>
      </div>
    </div>
  );
}
