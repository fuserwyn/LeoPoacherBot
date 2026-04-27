import "./RulesScreen.css";

export function RulesScreen() {
  return (
    <div className="rules">
      <header className="rules__header">
        <h1 className="rules__title">Правила стаи</h1>
        <p className="rules__lead muted">Кратко: как не потерять место в группе и зачем теги.</p>
      </header>

      <section className="rules__section">
        <h2 className="rules__h2">Отчёт о тренировке</h2>
        <p>
          Сообщение в чате стаи с тегом <code className="rules__code">#training_done</code> — это отчёт. По нему
          перезапускается таймер активности.
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Таймер и напоминания</h2>
        <p>
          Долго без отчёта — сначала предупреждение, затем возможно исключение из чата по правилам бота. Следи за
          стриком и не пропускай дедлайн.
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Больничный</h2>
        <p>
          Нужна пауза — отправь <code className="rules__code">#sick_leave</code>. Вернулся —{" "}
          <code className="rules__code">#healthy</code>, чтобы снова пошёл отсчёт.
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Мини-апп и Лео</h2>
        <p>
          Здесь — лента стаи и чат: ИИ отвечает, если в сообщении есть <code className="rules__code">@leo</code> или
          имя бота, как в Telegram. Личные вопросы Лео — вкладка «Лео».
        </p>
      </section>

      <section className="rules__section">
        <h2 className="rules__h2">Полная справка</h2>
        <p className="muted">В Telegram открой чат с ботом и отправь команду /help — там все команды списком.</p>
      </section>
    </div>
  );
}
