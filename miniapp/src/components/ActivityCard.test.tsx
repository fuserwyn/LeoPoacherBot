// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, fireEvent } from "@testing-library/react";
import { ActivityCard } from "./ActivityCard";

class ROStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(globalThis as unknown as { ResizeObserver: typeof ROStub }).ResizeObserver = ROStub;
// Эмулируем мышь, чтобы hover открывал поповер «кто отреагировал».
(globalThis as unknown as { matchMedia: (q: string) => MediaQueryList }).matchMedia = (q: string) =>
  ({ matches: q.includes("hover: hover"), media: q, addEventListener() {}, removeEventListener() {} }) as unknown as MediaQueryList;

afterEach(cleanup);

const baseProps = {
  avatar: "🦁",
  name: "Иван",
  timeAgo: "5 мин назад",
  emoji: "🏃",
  activity: "Бег",
  details: "15 мин, инт. 3/5",
};

describe("ActivityCard streak pill", () => {
  it("renders streak number with correctly declined aria-label", () => {
    render(<ActivityCard {...baseProps} streak={21} />);
    const pill = screen.getByLabelText("Стрик: 21 день подряд");
    expect(pill).toBeTruthy();
    expect(pill.textContent).toContain("21");
    expect(pill.textContent).toContain("Стрик");
  });

  it("uses 'дня' form for 3", () => {
    render(<ActivityCard {...baseProps} streak={3} />);
    expect(screen.getByLabelText("Стрик: 3 дня подряд")).toBeTruthy();
  });

  it("uses 'дней' form for 7", () => {
    render(<ActivityCard {...baseProps} streak={7} />);
    expect(screen.getByLabelText("Стрик: 7 дней подряд")).toBeTruthy();
  });

  it("hides the streak pill when hideStreak is set", () => {
    render(<ActivityCard {...baseProps} streak={10} hideStreak />);
    expect(screen.queryByLabelText(/^Стрик:/)).toBeNull();
  });
});

describe("ActivityCard reactions popover", () => {
  // Регресс: голос без фото и без имени не должен ронять рендер (был «чёрный экран»
  // из-за v.name.trim() при отсутствии ErrorBoundary).
  it("does not crash for a voter with no photo and missing name", () => {
    const { container } = render(
      <ActivityCard
        {...baseProps}
        streak={4}
        onReactionClick={() => {}}
        reactions={[
          // @ts-expect-error malformed voter: no name, no photo
          { emoji: "🔥", count: 1, me: false, voters: [{}] },
        ]}
      />,
    );
    const chip = container.querySelector(".act-card__react-btn");
    fireEvent.mouseEnter(chip!);
    const pop = document.querySelector(".act-card__likers");
    expect(pop).toBeTruthy();
    expect(pop!.textContent).toContain("Участник");
  });
});

describe("ActivityCard training photo fallback", () => {
  it("renders the photo when trainingPhotoUrl is present", () => {
    const { container } = render(
      <ActivityCard {...baseProps} trainingPhotoUrl="https://example.test/p.jpg" />,
    );
    const img = container.querySelector(".act-card__photo") as HTMLImageElement | null;
    expect(img).toBeTruthy();
    expect(img!.src).toContain("https://example.test/p.jpg");
    expect(container.querySelector(".act-card__photo-fallback")).toBeNull();
  });

  it("shows a retry fallback instead of silently hiding a broken photo", () => {
    const { container } = render(
      <ActivityCard {...baseProps} trainingPhotoUrl="https://example.test/broken.jpg" />,
    );
    const img = container.querySelector(".act-card__photo") as HTMLImageElement;
    fireEvent.error(img);
    // Фото больше не рендерится, но и не исчезает молча — есть кликабельная заглушка.
    expect(container.querySelector(".act-card__photo")).toBeNull();
    const fallback = container.querySelector(".act-card__photo-fallback");
    expect(fallback).toBeTruthy();
    expect(fallback!.textContent).toContain("Фото не загрузилось");
  });

  it("re-requests the photo with a cache-bust param on retry", () => {
    const { container } = render(
      <ActivityCard {...baseProps} trainingPhotoUrl="https://example.test/broken.jpg" />,
    );
    fireEvent.error(container.querySelector(".act-card__photo")!);
    fireEvent.click(container.querySelector(".act-card__photo-fallback")!);
    const img = container.querySelector(".act-card__photo") as HTMLImageElement;
    expect(img).toBeTruthy();
    expect(img.src).toContain("r=1");
  });
});
