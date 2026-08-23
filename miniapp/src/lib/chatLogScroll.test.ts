import { describe, expect, it, vi } from "vitest";
import { createChatScrollScheduler, isChatLogNearBottom, scrollChatLogToEnd } from "./chatLogScroll";

function fakeLog(partial: Partial<HTMLElement> & { scrollHeight: number; scrollTop: number; clientHeight: number }) {
  return partial as HTMLElement;
}

describe("chatLogScroll", () => {
  it("scrollChatLogToEnd sets scrollTop to height", () => {
    const log = fakeLog({ scrollHeight: 500, scrollTop: 0, clientHeight: 200 });
    scrollChatLogToEnd(log);
    expect(log.scrollTop).toBe(500);
  });

  it("isChatLogNearBottom", () => {
    const near = fakeLog({ scrollHeight: 500, scrollTop: 430, clientHeight: 60 });
    const far = fakeLog({ scrollHeight: 500, scrollTop: 0, clientHeight: 60 });
    expect(isChatLogNearBottom(near)).toBe(true);
    expect(isChatLogNearBottom(far)).toBe(false);
  });

  it("createChatScrollScheduler coalesces to one rAF", () => {
    const callbacks: FrameRequestCallback[] = [];
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      callbacks.push(cb);
      return callbacks.length;
    });
    const onScroll = vi.fn();
    const schedule = createChatScrollScheduler(onScroll);
    schedule();
    schedule();
    schedule();
    expect(onScroll).not.toHaveBeenCalled();
    expect(callbacks).toHaveLength(1);
    callbacks[0]!(0);
    expect(onScroll).toHaveBeenCalledTimes(1);
    vi.unstubAllGlobals();
  });
});
