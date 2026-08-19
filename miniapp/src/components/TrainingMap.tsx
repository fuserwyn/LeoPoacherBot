import { hapticImpact } from "../lib/haptics";
import { workoutsWordRu } from "../lib/achievements";
import type { TrainingMapNode, TrainingMapSnapshot } from "../lib/trainingMap";
import type { WorkoutCategoryId } from "../lib/workoutCategories";
import "./TrainingMap.css";

type Props = {
  map: TrainingMapSnapshot;
  onSelectWorkout: (type: WorkoutCategoryId) => void;
};

function nodeAria(node: TrainingMapNode): string {
  if (node.status === "done") return `${node.label}: пройдено`;
  if (node.status === "next") return `${node.label}: следующая тренировка`;
  return `${node.label}: ещё впереди`;
}

export function TrainingMap({ map, onSelectWorkout }: Props) {
  const next = map.nodes[map.nextIndex] ?? map.nodes[0];
  const d = map.nodes
    .map((n, i) => `${i === 0 ? "M" : "L"} ${n.x} ${n.y}`)
    .join(" ");
  const progressPct = map.nodes.length === 0 ? 0 : Math.round((map.completed / map.nodes.length) * 100);

  const pick = (node: TrainingMapNode) => {
    hapticImpact(node.status === "next" ? "medium" : "light");
    onSelectWorkout(node.id);
  };

  return (
    <section className="training-map" aria-label="Карта тренировок">
      <div className="training-map__head">
        <h2 className="section-title training-map__title">Карта тренировок</h2>
        <span className="training-map__count">
          {map.completed}/{map.nodes.length}
        </span>
      </div>
      <p className="training-map__progress-label muted">
        Пройдено {map.completed} из {map.nodes.length} на круге {map.lap}
        {" · "}
        осталось {map.remaining}
        {map.workoutsTotal > 0 ? ` · всего ${map.workoutsTotal} ${workoutsWordRu(map.workoutsTotal)}` : ""}
      </p>
      <div
        className="training-map__meter"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={map.nodes.length}
        aria-valuenow={map.completed}
        aria-label={`Прогресс карты: ${map.completed} из ${map.nodes.length}`}
      >
        <div className="training-map__meter-fill" style={{ width: `${progressPct}%` }} />
      </div>

      <div className="training-map__canvas">
        <svg className="training-map__path" viewBox="0 0 100 100" aria-hidden>
          <path d={d} fill="none" pathLength={1} />
        </svg>
        {map.nodes.map((node) => (
          <button
            key={`${node.index}-${node.id}`}
            type="button"
            className={`training-map__node training-map__node--${node.status}`}
            style={{ left: `${node.x}%`, top: `${node.y}%` }}
            aria-label={nodeAria(node)}
            aria-current={node.status === "next" ? "step" : undefined}
            onClick={() => pick(node)}
          >
            <span className="training-map__emoji" aria-hidden>
              {node.status === "done" ? "✓" : node.emoji}
            </span>
            <span className="training-map__label">{node.label}</span>
          </button>
        ))}
      </div>

      {next ? (
        <button type="button" className="training-map__cta" onClick={() => pick(next)}>
          Следующая: {next.emoji} {next.label}
        </button>
      ) : null}
    </section>
  );
}
