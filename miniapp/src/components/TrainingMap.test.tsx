// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { TrainingMap } from "./TrainingMap";
import { buildTrainingMapSnapshot } from "../lib/trainingMap";

afterEach(cleanup);

describe("TrainingMap", () => {
  it("shows completed vs remaining progress and opens the next workout in one tap", () => {
    const picked: string[] = [];
    const map = buildTrainingMapSnapshot(2);
    render(<TrainingMap map={map} onSelectWorkout={(id) => picked.push(id)} />);

    expect(screen.getByLabelText("Карта тренировок")).toBeTruthy();
    expect(screen.getByText("2/8")).toBeTruthy();
    expect(screen.getByText(/Пройдено 2 из 8/)).toBeTruthy();
    expect(screen.getByText(/осталось 6/)).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: /Следующая: .* Йога/ }));
    expect(picked).toEqual(["yoga"]);
  });

  it("lets tapping a map node select that workout", () => {
    const picked: string[] = [];
    render(
      <TrainingMap map={buildTrainingMapSnapshot(0)} onSelectWorkout={(id) => picked.push(id)} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Бег: следующая тренировка/ }));
    expect(picked).toEqual(["run"]);
  });
});
