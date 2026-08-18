// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import { TabKeepAlive } from "./TabKeepAlive";

function stubScroll(initial = 0) {
  let y = initial;
  Object.defineProperty(window, "scrollY", { configurable: true, get: () => y });
  Object.defineProperty(document.documentElement, "scrollTop", {
    configurable: true,
    get: () => y,
    set: (v: number) => {
      y = Number(v) || 0;
    },
  });
  Object.defineProperty(document.body, "scrollTop", {
    configurable: true,
    get: () => y,
    set: (v: number) => {
      y = Number(v) || 0;
    },
  });
  window.scrollTo = ((...args: unknown[]) => {
    if (typeof args[0] === "number") {
      y = Number(args[1]) || 0;
      return;
    }
    const opts = args[0] as { top?: number } | undefined;
    if (opts && typeof opts.top === "number") y = opts.top;
  }) as typeof window.scrollTo;
  return {
    get: () => y,
    set: (n: number) => {
      y = n;
    },
  };
}

afterEach(cleanup);

describe("TabKeepAlive scroll restore", () => {
  it("restores the saved window scroll when the pane becomes active again", () => {
    const scroll = stubScroll(0);
    const { rerender } = render(
      <TabKeepAlive active>
        <div>лента</div>
      </TabKeepAlive>,
    );

    scroll.set(640);
    rerender(
      <TabKeepAlive active={false}>
        <div>лента</div>
      </TabKeepAlive>,
    );

    // Как после display:none — браузер обрезал бы scrollY.
    scroll.set(0);

    rerender(
      <TabKeepAlive active>
        <div>лента</div>
      </TabKeepAlive>,
    );

    expect(scroll.get()).toBe(640);
  });

  it("does not replay a fade class on the active pane", () => {
    const { container, rerender } = render(
      <TabKeepAlive active>
        <div>лента</div>
      </TabKeepAlive>,
    );
    expect(container.querySelector(".tab-pane--active")).toBeTruthy();
    expect(container.querySelector(".tab-pane--enter")).toBeNull();

    rerender(
      <TabKeepAlive active={false}>
        <div>лента</div>
      </TabKeepAlive>,
    );
    rerender(
      <TabKeepAlive active>
        <div>лента</div>
      </TabKeepAlive>,
    );
    expect(container.querySelector(".tab-pane--enter")).toBeNull();
    expect(container.querySelector(".tab-pane--active")).toBeTruthy();
  });

  it("keeps children mounted while inactive", () => {
    const { rerender, getByText } = render(
      <TabKeepAlive active>
        <div>карточка</div>
      </TabKeepAlive>,
    );
    rerender(
      <TabKeepAlive active={false}>
        <div>карточка</div>
      </TabKeepAlive>,
    );
    expect(getByText("карточка")).toBeTruthy();
  });
});
