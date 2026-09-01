import { useCallback, useEffect, useState } from "react";
import {
  fetchAdminAnalytics,
  fetchAdminVisits,
  type AdminTable,
} from "../lib/adminApi";
import "./AdminDashboardScreen.css";

type Props = {
  initData: string;
  showAlert: (text: string) => void;
};

const PERIODS = [
  { days: 7, label: "7 дней" },
  { days: 30, label: "30 дней" },
  { days: 90, label: "90 дней" },
  { days: 0, label: "всё время" },
];

function parsePct(raw: string): number | null {
  const t = raw.trim();
  if (!t || t === "—" || t === "-") return null;
  const n = Number.parseFloat(t.replace("%", "").replace(",", "."));
  return Number.isFinite(n) ? n : null;
}

function parseCount(raw: string): number {
  const n = Number.parseInt(raw.replace(/\s/g, ""), 10);
  return Number.isFinite(n) ? n : 0;
}

function isFunnelTable(table: AdminTable) {
  return table.columns.some((c) => c.toLowerCase().includes("стадия"));
}

function isKpiTable(table: AdminTable) {
  return table.title.includes("KPI") || table.columns.some((c) => c.toLowerCase().includes("метрика"));
}

function isVisitsSummary(table: AdminTable) {
  return table.title.includes("Посещения") || table.columns.some((c) => c.toLowerCase().includes("метрика"));
}

function kpiTone(metric: string, value: number | null): "good" | "bad" | "neutral" {
  if (value == null) return "neutral";
  const m = metric.toLowerCase();
  if (m.includes("активация")) {
    if (value >= 35) return "good";
    if (value < 20) return "bad";
  }
  return "neutral";
}

function FunnelChart({ table }: { table: AdminTable }) {
  const stageIdx = table.columns.findIndex((c) => c.toLowerCase().includes("стадия"));
  const usersIdx = table.columns.findIndex((c) => c.toLowerCase().includes("юзер"));
  const convIdx = table.columns.findIndex((c) => c.toLowerCase().includes("конв"));
  if (stageIdx < 0 || usersIdx < 0) return null;

  const stages = table.rows.map((row) => ({
    label: row[stageIdx] ?? "",
    users: parseCount(row[usersIdx] ?? "0"),
    conv: convIdx >= 0 ? row[convIdx] ?? "—" : "—",
  }));
  const max = Math.max(1, ...stages.map((s) => s.users));

  return (
    <section className="dash-panel">
      <h3 className="dash-panel__title">{table.title.replace(/^[^\s]+\s/, "")}</h3>
      {table.subtitle ? <p className="dash-panel__subtitle">{table.subtitle}</p> : null}
      <ul className="dash-funnel">
        {stages.map((s) => (
          <li key={s.label} className="dash-funnel__row">
            <div className="dash-funnel__head">
              <span className="dash-funnel__label">{s.label}</span>
              <span className="dash-funnel__meta">
                <b>{s.users}</b>
                {s.conv !== "—" ? <small>{s.conv}</small> : null}
              </span>
            </div>
            <div className="dash-funnel__track" aria-hidden>
              <div className="dash-funnel__fill" style={{ width: `${(s.users / max) * 100}%` }} />
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

function KpiGrid({ table }: { table: AdminTable }) {
  const metricIdx = table.columns.findIndex((c) => c.toLowerCase().includes("метрика"));
  const valueIdx = table.columns.findIndex((c) => c.toLowerCase().includes("знач"));
  const targetIdx = table.columns.findIndex((c) => c.toLowerCase().includes("цель"));
  if (metricIdx < 0 || valueIdx < 0) return null;

  return (
    <section className="dash-panel">
      <h3 className="dash-panel__title">KPI</h3>
      {table.subtitle ? <p className="dash-panel__subtitle">{table.subtitle}</p> : null}
      <div className="dash-kpi">
        {table.rows.map((row) => {
          const metric = row[metricIdx] ?? "";
          const valueRaw = row[valueIdx] ?? "—";
          const value = parsePct(valueRaw);
          const tone = kpiTone(metric, value);
          return (
            <div key={metric} className={`dash-kpi__cell dash-kpi__cell--${tone}`}>
              <small>{metric}</small>
              <b>{valueRaw}</b>
              {targetIdx >= 0 && row[targetIdx] && row[targetIdx] !== "—" ? (
                <span className="dash-kpi__target">цель {row[targetIdx]}</span>
              ) : null}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function StatGrid({ table }: { table: AdminTable }) {
  const keyIdx = table.columns.findIndex((c) => c.toLowerCase().includes("метрика") || c.toLowerCase().includes("юзер"));
  const valIdx = table.columns.length > 1 ? (keyIdx === 0 ? 1 : table.columns.length - 1) : 1;
  if (table.rows.length === 0) return null;

  return (
    <section className="dash-panel">
      <h3 className="dash-panel__title">{table.title.replace(/^[^\s]+\s/, "")}</h3>
      <div className="dash-stats">
        {table.rows.map((row) => (
          <div key={row[keyIdx]} className="dash-stats__cell">
            <b>{row[valIdx]}</b>
            <small>{row[keyIdx]}</small>
          </div>
        ))}
      </div>
    </section>
  );
}

function RetentionBars({ table }: { table: AdminTable }) {
  const labelIdx = 0;
  const usersIdx = 1;
  const rows = table.rows.map((row) => ({
    label: row[labelIdx] ?? "",
    users: parseCount(row[usersIdx] ?? "0"),
  }));
  const max = Math.max(1, ...rows.map((r) => r.users));

  return (
    <section className="dash-panel">
      <h3 className="dash-panel__title">{table.title.replace(/^[^\s]+\s/, "")}</h3>
      {table.subtitle ? <p className="dash-panel__subtitle">{table.subtitle}</p> : null}
      <ul className="dash-retention">
        {rows.map((r) => (
          <li key={r.label} className="dash-retention__row">
            <span className="dash-retention__label">{r.label}</span>
            <div className="dash-retention__track" aria-hidden>
              <div className="dash-retention__fill" style={{ width: `${(r.users / max) * 100}%` }} />
            </div>
            <span className="dash-retention__val">{r.users}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Визуальные дашборды: KPI, воронки и посещения поверх тех же данных, что и таблицы. */
export function AdminDashboardScreen({ initData, showAlert }: Props) {
  const [period, setPeriod] = useState(30);
  const [loading, setLoading] = useState(true);
  const [note, setNote] = useState("");
  const [tables, setTables] = useState<AdminTable[]>([]);
  const [visits, setVisits] = useState<AdminTable[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [analytics, visitData] = await Promise.all([
        fetchAdminAnalytics(initData, period),
        fetchAdminVisits(initData),
      ]);
      setTables(analytics.analytics.tables ?? []);
      setVisits(visitData.tables ?? []);
      setNote(
        analytics.analytics.last_event_at
          ? `Период: ${analytics.analytics.period} · последнее событие ${analytics.analytics.last_event_at} (МСК)`
          : `Период: ${analytics.analytics.period}`,
      );
    } catch (e) {
      showAlert(e instanceof Error ? e.message : "Не удалось загрузить дашборды");
    } finally {
      setLoading(false);
    }
  }, [initData, period, showAlert]);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) return <p className="dash-muted">Загрузка…</p>;

  const kpiTable = tables.find(isKpiTable);
  const funnelTables = tables.filter(isFunnelTable);
  const retentionTable = tables.find((t) => t.title.includes("Retention"));
  const visitsSummary = visits.find(isVisitsSummary);

  return (
    <div className="dash">
      <div className="dash-periods">
        {PERIODS.map((p) => (
          <button
            key={p.days}
            type="button"
            className={period === p.days ? "on" : ""}
            onClick={() => setPeriod(p.days)}
          >
            {p.label}
          </button>
        ))}
      </div>
      {note ? <p className="dash-muted">{note}</p> : null}

      {visitsSummary ? <StatGrid table={visitsSummary} /> : null}
      {kpiTable ? <KpiGrid table={kpiTable} /> : null}

      {funnelTables.map((t) => (
        <FunnelChart key={t.title} table={t} />
      ))}

      {retentionTable ? <RetentionBars table={retentionTable} /> : null}

      {tables.length === 0 && visits.length === 0 ? (
        <p className="dash-muted">Событий пока нет — дашборды заполнятся после первых действий пользователей.</p>
      ) : null}

      <p className="dash-muted dash-hint">
        Цифры считаются на сервере теми же запросами, что и в чате /admin. Подробные таблицы — в разделе «Аналитика».
      </p>
    </div>
  );
}
