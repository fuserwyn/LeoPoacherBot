import { useCallback, useEffect, useState } from "react";
import {
  fetchAdminDbTable,
  fetchAdminDbTables,
  runAdminDbQuery,
  type AdminDbTable,
  type AdminQueryResult,
} from "../lib/adminApi";
import "./AdminOpsScreen.css";

type Props = {
  initData: string;
  showAlert: (text: string) => void;
};

const PAGE = 50;

/** Вкладка «Данные»: таблицы базы стаи и SQL-редактор (только чтение). */
export function AdminDataScreen({ initData, showAlert }: Props) {
  const [tables, setTables] = useState<AdminDbTable[]>([]);
  const [table, setTable] = useState("");
  const [offset, setOffset] = useState(0);
  const [result, setResult] = useState<AdminQueryResult | null>(null);
  const [sql, setSql] = useState("");
  const [busy, setBusy] = useState(false);
  const [mode, setMode] = useState<"tables" | "sql">("tables");

  const fail = useCallback(
    (e: unknown, fallback: string) => showAlert(e instanceof Error ? e.message : fallback),
    [showAlert],
  );

  useEffect(() => {
    void (async () => {
      try {
        const j = await fetchAdminDbTables(initData);
        setTables(j.tables ?? []);
      } catch (e) {
        fail(e, "Не удалось получить список таблиц");
      }
    })();
  }, [initData, fail]);

  const openTable = useCallback(
    async (name: string, nextOffset: number) => {
      setBusy(true);
      try {
        const j = await fetchAdminDbTable(initData, name, PAGE, nextOffset);
        setTable(name);
        setOffset(nextOffset);
        setResult(j.result);
        setMode("tables");
      } catch (e) {
        fail(e, "Не удалось открыть таблицу");
      } finally {
        setBusy(false);
      }
    },
    [initData, fail],
  );

  const runQuery = async () => {
    const q = sql.trim();
    if (!q) {
      showAlert("Напиши запрос.");
      return;
    }
    setBusy(true);
    try {
      const j = await runAdminDbQuery(initData, q);
      setResult(j.result);
      setTable("");
    } catch (e) {
      fail(e, "Запрос не выполнился");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="ops">
      <div className="ops-periods">
        <button type="button" className={mode === "tables" ? "on" : ""} onClick={() => setMode("tables")}>
          Таблицы
        </button>
        <button type="button" className={mode === "sql" ? "on" : ""} onClick={() => setMode("sql")}>
          SQL
        </button>
      </div>

      {mode === "sql" ? (
        <div className="ops-card">
          <textarea
            value={sql}
            onChange={(e) => setSql(e.target.value)}
            placeholder="SELECT user_id, streak FROM training_state ORDER BY streak DESC LIMIT 20"
            spellCheck={false}
          />
          <div className="ops-row">
            <button type="button" className="ops-primary" disabled={busy} onClick={() => void runQuery()}>
              Выполнить
            </button>
          </div>
          <p className="ops-muted">
            Только чтение: SELECT / WITH / EXPLAIN / SHOW / TABLE / VALUES, один запрос без «;». Отдаётся не больше 200
            строк, таймаут 20 секунд.
          </p>
        </div>
      ) : (
        <div className="ops-chips">
          {tables.map((t) => (
            <button
              key={t.name}
              type="button"
              className={`ops-chip${table === t.name ? " on" : ""}`}
              disabled={busy}
              onClick={() => void openTable(t.name, 0)}
            >
              {t.name}
              <small>~{t.rows}</small>
            </button>
          ))}
          {tables.length === 0 ? <p className="ops-muted">Таблиц не видно</p> : null}
        </div>
      )}

      {result ? (
        <section className="ops-table">
          <h3 className="ops-table__title">{table || "Результат"}</h3>
          <p className="ops-table__subtitle">
            Строк: {result.rows.length}
            {result.truncated ? " (обрезано до 200)" : ""} · {result.took}
          </p>
          <div className="ops-table__scroll">
            <table>
              <thead>
                <tr>
                  {result.columns.map((c) => (
                    <th key={c}>{c}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {result.rows.map((row, i) => (
                  <tr key={i}>
                    {row.map((cell, j) => (
                      <td key={j} title={cell}>
                        {cell.length > 80 ? `${cell.slice(0, 80)}…` : cell}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {result.rows.length === 0 ? <p className="ops-muted">Пусто</p> : null}
          {table ? (
            <div className="ops-row">
              <button type="button" disabled={busy || offset === 0} onClick={() => void openTable(table, Math.max(0, offset - PAGE))}>
                ◀ Назад
              </button>
              <button type="button" disabled={busy || result.rows.length < PAGE} onClick={() => void openTable(table, offset + PAGE)}>
                Вперёд ▶
              </button>
            </div>
          ) : null}
        </section>
      ) : null}
    </div>
  );
}
