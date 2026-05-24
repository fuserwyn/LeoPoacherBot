import { CUP_LEVEL_STARTS, MINIAPP_LEVEL_NAMES } from "../lib/miniappLevel";
import "./RulesScreen.css";

const LEVEL_ROWS: { level: number; name: string; from: number }[] = MINIAPP_LEVEL_NAMES.slice(1).map((name, i) => ({
  level: i + 1,
  name,
  from: CUP_LEVEL_STARTS[i] ?? 0,
}));

const ACTIVITY_COEFFS: { coeff: string; types: string }[] = [
  { coeff: "×0.8", types: "йога, растяжка, ходьба" },
  { coeff: "×1.0", types: "силовая, гиря, гребля, воркаут, танцы, другое" },
  { coeff: "×1.2", types: "бег, велосипед, плавание, кардио, скакалка" },
  { coeff: "×1.5", types: "кроссфит, HIIT" },
];

export function RulesScreen() {
  return (
    <div className="rules">
      <header className="rules__header">
        <h1 className="rules__title">Правила стаи</h1>
        <p className="rules__lead muted">
          Тренируешься регулярно — растёшь по саванне от Суриката до Слона. Пропадаешь надолго — стая теряет тебя.
        </p>
      </header>

      <section className="rules__section">
        <h2 className="rules__h2">Как отправить тренировку</h2>
        <p>
          Жми «+» внизу мини-аппа: выбери тип, минуты, интенсивность 1–5, по желанию фото и комментарий. Аппа отправит
          отчёт о тренировке через кнопку «+» в мини-аппе и начислит кубки.
        </p>
        <p className="rules__note">
          Из Telegram это тоже работает — отправь в чат стаи строку вида
          {" "}<code className="rules__code">бег, 30 мин, инт. 3/5</code>.
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Кубки за тренировку 🏆</h2>
        <p>
          Формула: <code className="rules__code">кубки = (минуты × интенсивность × коэф. типа) / 5</code>, минимум 1.
          Длительность учитывается в диапазоне 1–480 мин, интенсивность 1–5.
        </p>
        <table className="rules__table">
          <thead>
            <tr>
              <th>Коэф.</th>
              <th>Типы тренировок</th>
            </tr>
          </thead>
          <tbody>
            {ACTIVITY_COEFFS.map((row) => (
              <tr key={row.coeff}>
                <td className="rules__table-coeff">{row.coeff}</td>
                <td>{row.types}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Уровни саванны 🦓</h2>
        <p>
          С каждым уровнем растёт кап попыток для стрика (см. ниже). Леопард — это Лео и бренд аппы, не уровень.
        </p>
        <table className="rules__table">
          <thead>
            <tr>
              <th>Ур.</th>
              <th>Имя</th>
              <th className="rules__table-num">Кубков</th>
            </tr>
          </thead>
          <tbody>
            {LEVEL_ROWS.map((row) => (
              <tr key={row.level}>
                <td>L{row.level}</td>
                <td>{row.name}</td>
                <td className="rules__table-num">{row.from === 0 ? "0" : row.from.toLocaleString("ru-RU")}+</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Стрик и ачивки 🔥</h2>
        <p>
          Стрик — дни подряд с тренировкой. Пропустил день — стрик уходит в 0 (если не было попытки спасения).
          Ачивки даются на отметках <strong>7 / 14 / 30 / 42 / 60 / 90 / 180 / 365</strong> дней.
        </p>
        <p className="rules__note">
          День считается по <strong>твоему</strong> локальному часовому поясу — мини-апп определяет его автоматически.
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Попытки спасения стрика 🛡️</h2>
        <p>
          На старте 1 попытка, +1 за каждый новый уровень, максимум 7. При пропуске тренировки попытка автоматически
          закрывает день, и стрик не падает. Покупать попытки нельзя — только зарабатывать ростом по уровням.
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Таймер неактивности ⏰</h2>
        <p>
          Если 8 дней без тренировок и без больничного — стая исключает из чата. Любая тренировка обнуляет таймер.
          Время удаления видно в шапке ленты и на карточке профиля.
        </p>
        <p className="rules__note">
          Возврат после кика — отдельная reactivation-плата, см. в боте команду{" "}
          <code className="rules__code">/help</code>.
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Больничный 🤒</h2>
        <p>
          Болеешь / в отпуске / травма — отправь Лео в личку{" "}
          <code className="rules__code">#sick_leave</code> с коротким обоснованием. Таймер удаления и стрик ставятся на
          паузу. Возвращаешься — <code className="rules__code">#healthy</code>, и счёт идёт дальше.
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Лента и Лео 💬</h2>
        <p>
          Во вкладке «Стая» — отчёты тренировок, реакции и треды-комментарии. Во вкладке «Лео» — личный чат: поставь
          лайк сообщению Лео, спроси совет, разбери срыв.
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Полная справка</h2>
        <p>
          В Telegram открой чат с ботом и отправь команду <code className="rules__code">/help</code> — там все команды
          и хэштеги списком.
        </p>
      </section>
    </div>
  );
}
