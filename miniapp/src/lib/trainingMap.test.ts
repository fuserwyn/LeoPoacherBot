import { describe, expect, it } from "vitest";
import { buildTrainingMapSnapshot, TRAINING_MAP_PATH } from "./trainingMap";

describe("buildTrainingMapSnapshot", () => {
  it("marks the first node as next when there are no workouts", () => {
    const s = buildTrainingMapSnapshot(0);
    expect(s.completed).toBe(0);
    expect(s.remaining).toBe(TRAINING_MAP_PATH.length);
    expect(s.nodes[0]?.status).toBe("next");
    expect(s.nodes[1]?.status).toBe("remaining");
  });

  it("counts completed vs remaining on the current lap", () => {
    const s = buildTrainingMapSnapshot(3);
    expect(s.completed).toBe(3);
    expect(s.remaining).toBe(5);
    expect(s.nextIndex).toBe(3);
    expect(s.nodes.filter((n) => n.status === "done")).toHaveLength(3);
    expect(s.nodes[3]?.status).toBe("next");
    expect(s.nodes[3]?.id).toBe("walk");
  });

  it("starts a new lap after a full circle", () => {
    const s = buildTrainingMapSnapshot(8);
    expect(s.lap).toBe(2);
    expect(s.completed).toBe(0);
    expect(s.nodes[0]?.status).toBe("next");
  });
});
