// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { FeedScreen } from "./FeedScreen";

class ROStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
class IOStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(globalThis as unknown as { ResizeObserver: typeof ROStub }).ResizeObserver = ROStub;
(globalThis as unknown as { IntersectionObserver: typeof IOStub }).IntersectionObserver = IOStub;
(globalThis as unknown as { matchMedia: (q: string) => MediaQueryList }).matchMedia = (q: string) =>
  ({
    matches: false,
    media: q,
    addEventListener() {},
    removeEventListener() {},
  }) as unknown as MediaQueryList;

afterEach(cleanup);

const baseProps = {
  streak: 1,
  userId: 1,
  initData: "",
  inTelegram: false,
  showAlert: () => {},
};

describe("FeedScreen tab keep-alive", () => {
  it("does not show Загрузка… when leaving and returning to the tab", async () => {
    const { rerender } = render(<FeedScreen {...baseProps} active refreshToken={0} />);
    await waitFor(() => expect(screen.queryByText("Загрузка…")).toBeNull());

    rerender(<FeedScreen {...baseProps} active={false} refreshToken={0} />);
    rerender(<FeedScreen {...baseProps} active refreshToken={0} />);
    expect(screen.queryByText("Загрузка…")).toBeNull();
  });

  it("does not show Загрузка… when only onOptimisticConsumed identity changes", async () => {
    const { rerender } = render(
      <FeedScreen {...baseProps} active refreshToken={0} onOptimisticConsumed={() => {}} />,
    );
    await waitFor(() => expect(screen.queryByText("Загрузка…")).toBeNull());

    rerender(
      <FeedScreen {...baseProps} active refreshToken={0} onOptimisticConsumed={() => {}} />,
    );
    expect(screen.queryByText("Загрузка…")).toBeNull();
  });
});
