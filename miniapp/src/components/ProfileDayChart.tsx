import { formatChartDayLabel, type ProfileChartDay } from "../lib/profileCharts";
import "./ProfileDayChart.css";

type Props = {
  title: string;
  subtitle?: string;
  points: ProfileChartDay[];
  emptyHint?: string;
  variant?: "workouts" | "streak";
};

export function ProfileDayChart({
  title,
  subtitle,
  points,
  emptyHint = "Пока нет данных за выбранный период",
  variant = "workouts",
}: Props) {
  const max = Math.max(1, ...points.map((p) => p.value));
  const hasData = points.some((p) => p.value > 0);

  return (
    <div className={`profile-day-chart profile-day-chart--${variant}`}>
      <div className="profile-day-chart__head">
        <span className="profile-day-chart__title">{title}</span>
        {subtitle ? <span className="profile-day-chart__subtitle">{subtitle}</span> : null}
      </div>
      {!hasData ? (
        <p className="profile-day-chart__empty muted">{emptyHint}</p>
      ) : (
        <div className="profile-day-chart__scroll" role="img" aria-label={title}>
          <div className="profile-day-chart__plot">
            {points.map((p, i) => {
              const showLabel = i === 0 || i === points.length - 1 || (i + 1) % 7 === 0;
              const heightPct = p.value > 0 ? Math.max(8, (p.value / max) * 100) : 0;
              return (
                <div
                  key={p.date}
                  className={`profile-day-chart__col${p.value > 0 ? " has-value" : ""}`}
                  title={`${formatChartDayLabel(p.date)}: ${p.value}`}
                >
                  <div className="profile-day-chart__bar-wrap" aria-hidden>
                    {p.value > 0 ? (
                      <div className="profile-day-chart__bar" style={{ height: `${heightPct}%` }} />
                    ) : null}
                  </div>
                  {showLabel ? (
                    <span className="profile-day-chart__label">{formatChartDayLabel(p.date)}</span>
                  ) : (
                    <span className="profile-day-chart__label profile-day-chart__label--spacer" aria-hidden />
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
