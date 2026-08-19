import { useCallback, useEffect, useState } from "react";
import { adminStand, fetchAdminResources, type AdminResources, type AdminStand } from "../lib/adminApi";
import "./AdminOpsScreen.css";

type Props = {
  initData: string;
  showAlert: (text: string) => void;
};

const money = (usd: number) => `${usd < 0 ? "−" : ""}$${Math.abs(usd).toFixed(2)}`;

/** Вкладка «Ресурсы»: сколько сервер съел, сколько принесли оплаты, разница. */
export function AdminResourcesScreen({ initData, showAlert }: Props) {
  const [data, setData] = useState<AdminResources | null>(null);
  const [loading, setLoading] = useState(true);
  /** Тестовый стенд: он тратит деньги, пока крутится, и гасить его удобнее отсюда. */
  const [stand, setStand] = useState<AdminStand | null>(null);
  const [standBusy, setStandBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const j = await fetchAdminResources(initData);
      setData(j.resources);
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось посчитать ресурсы");
    } finally {
      setLoading(false);
    }
  }, [initData, showAlert]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    void (async () => {
      try {
        setStand((await adminStand(initData)).stand);
      } catch {
        // Стенда может не быть — тогда блок просто не показываем.
        setStand(null);
      }
    })();
  }, [initData]);

  const switchStand = async (action: "start" | "stop") => {
    setStandBusy(true);
    try {
      const j = await adminStand(initData, action);
      setStand(j.stand);
      showAlert(action === "start" ? "Стенд поднимается." : "Стенд погашен.");
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не получилось");
    } finally {
      setStandBusy(false);
    }
  };

  if (loading) return <p className="ops-muted">Считаю…</p>;
  if (!data) return <p className="ops-muted">Нет данных</p>;

  return (
    <div className="ops">
      <div className="ops-money">
        <div className="ops-money__cell">
          <small>Расход</small>
          <b>{money(data.cost_usd)}</b>
        </div>
        <div className="ops-money__cell">
          <small>Доход</small>
          <b>{money(data.income_usd)}</b>
        </div>
        <div className={`ops-money__cell ops-money__cell--net${data.net_usd < 0 ? " is-minus" : ""}`}>
          <small>Разница</small>
          <b>{money(data.net_usd)}</b>
        </div>
      </div>
      <p className="ops-muted">Месяц {data.month}. Всё в долларах, по текущую дату.</p>

      {stand?.configured ? (
        <section className="ops-table">
          <h3 className="ops-table__title">🧪 Тестовый стенд</h3>
          {/* Только переключатель: состояние сервисов и ссылку убрали — админу
              здесь нужно одно решение, включён стенд или нет. */}
          <button
            type="button"
            className={stand.running ? "ops-danger" : "ops-primary"}
            disabled={standBusy}
            onClick={() => void switchStand(stand.running ? "stop" : "start")}
          >
            {stand.running ? "Выключить стенд" : "Включить стенд"}
          </button>
        </section>
      ) : null}

      <section className="ops-table">
        <h3 className="ops-table__title">🖥 Ресурсы Railway</h3>
        <p className="ops-table__subtitle">{data.cost_note}</p>
        {data.cost_parts.length > 0 ? (
          <div className="ops-table__scroll">
            <table>
              <thead>
                <tr>
                  <th>Ресурс</th>
                  <th>Накоплено</th>
                  <th>Стоимость</th>
                </tr>
              </thead>
              <tbody>
                {data.cost_parts.map((p) => (
                  <tr key={p.key}>
                    <td>{p.label}</td>
                    <td>{p.raw.toFixed(2)}</td>
                    <td>{money(p.usd)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>

      <section className="ops-table">
        <h3 className="ops-table__title">💰 Оплаты за месяц</h3>
        <p className="ops-table__subtitle">{data.payments_note || data.rates_note}</p>
        {data.income.length > 0 ? (
          <div className="ops-table__scroll">
            <table>
              <thead>
                <tr>
                  <th>Валюта</th>
                  <th>Оплат</th>
                  <th>Сумма</th>
                  <th>В долларах</th>
                </tr>
              </thead>
              <tbody>
                {data.income.map((i) => (
                  <tr key={i.currency}>
                    <td>{i.currency}</td>
                    <td>{i.count}</td>
                    <td>{i.amount.toFixed(2)}</td>
                    <td>{money(i.usd)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>

      <button type="button" className="ops-primary" onClick={() => void load()}>
        Обновить
      </button>
    </div>
  );
}
