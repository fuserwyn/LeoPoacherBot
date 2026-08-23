import { describe, expect, it } from "vitest";
import {
  parseTrainingDoneCategory,
  parseTrainingDoneCategories,
  trainingDoneCategoryDisplayLabel,
  trainingDoneMatchesCategory,
  trainingDoneMatchesAnyCategory,
} from "./workoutCategories";

describe("parseTrainingDoneCategories — мультивыбор", () => {
  it("разбирает один вид", () => {
    expect(parseTrainingDoneCategories("бег, 15 мин, инт. 3/5")).toEqual(["run"]);
  });

  it("разбирает несколько видов через «+»", () => {
    expect(parseTrainingDoneCategories("бег + плавание, 30 мин, инт. 3/5")).toEqual(["run", "swim"]);
  });

  it("схлопывает дубли и поддерживает «/» как разделитель", () => {
    expect(parseTrainingDoneCategories("плавание / бег / плавание, 20 мин, инт. 2/5")).toEqual([
      "swim",
      "run",
    ]);
  });

  it("неизвестный вид → other", () => {
    expect(parseTrainingDoneCategories("пилатес, 30 мин, инт. 2/5")).toEqual(["other"]);
  });

  it("нераспознанный формат → пустой массив", () => {
    expect(parseTrainingDoneCategories("просто текст")).toEqual([]);
  });

  it("первый вид — для эмодзи/заголовка", () => {
    expect(parseTrainingDoneCategory("йога + плавание, 30 мин, инт. 3/5")).toBe("yoga");
  });
});

describe("фильтр ленты по нескольким видам", () => {
  const text = "бег + плавание, 30 мин, инт. 3/5";

  it("матчится по каждому из своих видов", () => {
    expect(trainingDoneMatchesCategory(text, "run")).toBe(true);
    expect(trainingDoneMatchesCategory(text, "swim")).toBe(true);
    expect(trainingDoneMatchesCategory(text, "yoga")).toBe(false);
  });

  it("мультифильтр: совпадение хотя бы по одному виду", () => {
    expect(trainingDoneMatchesAnyCategory(text, new Set(["swim"]))).toBe(true);
    expect(trainingDoneMatchesAnyCategory(text, new Set(["yoga", "bike"]))).toBe(false);
    expect(trainingDoneMatchesAnyCategory(text, new Set())).toBe(true);
  });
});

describe("заголовок карточки", () => {
  it("показывает оба вида как ввёл пользователь", () => {
    expect(trainingDoneCategoryDisplayLabel("бег + плавание, 30 мин, инт. 3/5")).toBe("бег + плавание");
  });
});
