import { describe, expect, it } from "vitest";
import {
  buildFeedPostTextForSave,
  extractEditableFeedPostText,
  feedItemKey,
  feedPostEditable,
  feedScrollBehavior,
  isFeedMessage,
  mergePackFeedFullWindow,
  mergePackFeedIncremental,
  packMessageCommentReplyToId,
  mergePackFeedReactions,
  optimisticTogglePackFeedReaction,
  optimisticToggleThreadReplyLike,
  planFeedFilterMotion,
  planFeedMotion,
  planFeedNarrowPath,
  planActivityCardNarrowPath,
  canShowFeedNarrow,
  canShowActivityCard,
  activityCardKey,
  feedWindowKey,
  isNarrowFeedViewport,
  NARROW_FEED_GUTTER_PX,
  NARROW_FEED_MIN_TAP_PX,
  reconcilePinnedFeed,
  sameFeedItems,
  sortPackFeedItemsDesc,
  type PackFeedItemDTO,
  type PackFeedThreadReplyDTO,
} from "./packFeed";

function item(partial: Partial<PackFeedItemDTO> & Pick<PackFeedItemDTO, "id">): PackFeedItemDTO {
  return {
    type: "training_done",
    created_at: "2026-07-01T12:00:00Z",
    text: "x",
    user_id: 1,
    username: "u",
    streak_days: 0,
    is_you: false,
    ...partial,
  };
}

describe("feedPostEditable", () => {
  it("only own training/healthy/pack_message", () => {
    expect(feedPostEditable("training_done", true)).toBe(true);
    expect(feedPostEditable("healthy", true)).toBe(true);
    expect(feedPostEditable("pack_message", true)).toBe(true);
    expect(feedPostEditable("training_done", false)).toBe(false);
    expect(feedPostEditable("sick_leave", true)).toBe(false);
  });
});

describe("extractEditableFeedPostText / buildFeedPostTextForSave", () => {
  it("strips #healthy for edit and restores prefix on save", () => {
    const raw = "#healthy\nУра, выздоровел!";
    expect(extractEditableFeedPostText(raw, "healthy")).toBe("Ура, выздоровел!");
    expect(buildFeedPostTextForSave(raw, "healthy", "Снова в строю")).toBe("#healthy\nСнова в строю");
  });

  it("keeps training header kind when editing body", () => {
    const raw = "#training_done — танцы, 90 мин, инт. 2/5\nТанцы!";
    expect(extractEditableFeedPostText(raw, "training_done")).toContain("Танцы!");
    const saved = buildFeedPostTextForSave(raw, "training_done", "Новый комментарий");
    expect(saved).toMatch(/^#training_done/);
    expect(saved).toContain("танцы,");
    expect(saved).toContain("Новый комментарий");
  });

  it("pack_message is plain text", () => {
    expect(extractEditableFeedPostText("привет", "pack_message")).toBe("привет");
    expect(buildFeedPostTextForSave("привет", "pack_message", "пока")).toBe("пока");
  });
});

describe("feed keys / message detection", () => {
  it("feedItemKey namespaces by source", () => {
    expect(feedItemKey({ id: 5 })).toBe("feed:5");
    expect(feedItemKey({ id: 5, source: "message" })).toBe("message:5");
  });

  it("isFeedMessage", () => {
    expect(isFeedMessage({ type: "pack_message" })).toBe(true);
    expect(isFeedMessage({ type: "training_done", source: "message" })).toBe(true);
    expect(isFeedMessage({ type: "training_done", source: "feed" })).toBe(false);
  });

  it("packMessageCommentReplyToId targets a comment or the card", () => {
    expect(packMessageCommentReplyToId(10)).toBe(10);
    expect(packMessageCommentReplyToId(10, 0)).toBe(10);
    expect(packMessageCommentReplyToId(10, 77)).toBe(77);
  });
});

describe("reactions", () => {
  it("mergePackFeedReactions puts mine first", () => {
    const merged = mergePackFeedReactions(["💪", "❤️", "🔥"], [
      { emoji: "🔥", count: 2, me: true },
      { emoji: "💪", count: 1, me: false },
    ]);
    expect(merged[0]?.emoji).toBe("🔥");
    expect(merged[0]?.me).toBe(true);
    expect(merged.find((r) => r.emoji === "❤️")?.count).toBe(0);
  });

  it("optimisticTogglePackFeedReaction toggles same emoji off", () => {
    const after = optimisticTogglePackFeedReaction([{ emoji: "❤️", count: 1, me: true }], "❤️");
    expect(after).toEqual([]);
  });

  it("optimisticTogglePackFeedReaction switches emoji", () => {
    const after = optimisticTogglePackFeedReaction([{ emoji: "❤️", count: 2, me: true }], "🔥");
    expect(after.find((r) => r.emoji === "❤️")).toEqual({ emoji: "❤️", count: 1, me: false, voters: undefined });
    expect(after.find((r) => r.emoji === "🔥")).toEqual({ emoji: "🔥", count: 1, me: true });
  });

  it("optimisticToggleThreadReplyLike", () => {
    const thread: PackFeedThreadReplyDTO[] = [
      {
        id: 1,
        user_id: 1,
        username: "a",
        text: "a",
        created_at: "2026-07-01T00:00:00Z",
        is_you: false,
        like_count: 0,
        like_me: false,
      },
      {
        id: 2,
        user_id: 2,
        username: "b",
        text: "b",
        created_at: "2026-07-01T00:00:00Z",
        is_you: true,
        like_count: 3,
        like_me: true,
      },
    ];
    const next = optimisticToggleThreadReplyLike(thread, 2);
    expect(next[1]?.like_me).toBe(false);
    expect(next[1]?.like_count).toBe(2);
    expect(next[0]?.like_count).toBe(0);
  });
});

describe("pinned / sort", () => {
  it("reconcilePinnedFeed pins and unpins", () => {
    const prev = [
      item({ id: 1, is_pinned: true, pinned_at: "2026-07-01T10:00:00Z" }),
      item({ id: 2, is_pinned: false }),
    ];
    const pinned = [item({ id: 2, is_pinned: true, pinned_at: "2026-07-02T10:00:00Z" })];
    const next = reconcilePinnedFeed(prev, pinned);
    expect(next.find((x) => x.id === 1)?.is_pinned).toBe(false);
    expect(next.find((x) => x.id === 2)?.is_pinned).toBe(true);
  });

  it("sortPackFeedItemsDesc: newer created_at first", () => {
    const items = [
      item({ id: 1, created_at: "2026-07-03T00:00:00Z" }),
      item({ id: 2, created_at: "2026-07-01T00:00:00Z" }),
      item({ id: 3, created_at: "2026-07-04T00:00:00Z" }),
    ];
    const sorted = sortPackFeedItemsDesc(items);
    expect(sorted.map((x) => x.id)).toEqual([3, 1, 2]);
  });
});

describe("feed motion / smooth window merge", () => {
  const reply = (id: number): PackFeedThreadReplyDTO => ({
    id,
    user_id: 1,
    username: "a",
    text: "hi",
    created_at: "2026-07-01T01:00:00Z",
    is_you: false,
  });

  it("happy path: incoming updates thread without dropping older pages", () => {
    const older = item({ id: 1, created_at: "2026-06-01T00:00:00Z" });
    const windowItem = item({ id: 10, created_at: "2026-07-01T00:00:00Z", thread: [] });
    const prev = [windowItem, older];
    const incoming = [item({ id: 10, created_at: "2026-07-01T00:00:00Z", thread: [reply(7)] })];
    const next = mergePackFeedFullWindow(prev, incoming);
    expect(next.find((x) => x.id === 1)).toBeTruthy();
    expect(next.find((x) => x.id === 10)?.thread?.map((t) => t.id)).toEqual([7]);
    const path = planFeedMotion({ prev, next });
    expect(path.apply).toBe(true);
    expect(path.reason).toBe("ok");
  });

  it("empty incoming on a loaded feed is a silent reject", () => {
    const prev = [item({ id: 1 })];
    expect(mergePackFeedFullWindow(prev, [])).toBe(prev);
    expect(mergePackFeedIncremental(prev, [])).toBe(prev);
    const path = planFeedMotion({ prev, next: [], reset: false });
    expect(path.apply).toBe(false);
    expect(path.reason).toBe("empty");
  });

  it("repeat of the same window keeps the previous array", () => {
    const prev = [item({ id: 1 }), item({ id: 2, created_at: "2026-06-01T00:00:00Z" })];
    const incoming = [item({ id: 1 })];
    const next = mergePackFeedFullWindow(prev, incoming);
    expect(next).toBe(prev);
    expect(sameFeedItems(prev, next)).toBe(true);
    const path = planFeedMotion({ prev, next });
    expect(path.reason).toBe("repeat");
    expect(path.apply).toBe(false);
  });

  it("old reset path still replaces the window", () => {
    const prev = [item({ id: 1 }), item({ id: 99, created_at: "2026-01-01T00:00:00Z" })];
    const incoming = [item({ id: 2, created_at: "2026-07-02T00:00:00Z" })];
    const path = planFeedMotion({ prev, next: incoming, reset: true });
    expect(path.apply).toBe(true);
    expect(path.reason).toBe("ok");
    // reset на клиенте подставляет incoming как есть — старые страницы не тащим.
    expect(incoming.map((x) => x.id)).toEqual([2]);
  });
});

describe("planFeedFilterMotion", () => {
  it("happy path: new filter scrolls to top", () => {
    const path = planFeedFilterMotion(
      { scope: "all", type: "all", cats: [] },
      { scope: "mine", type: "all", cats: [] },
    );
    expect(path.apply).toBe(true);
    expect(path.scrollToTop).toBe(true);
    expect(path.animate).toBe(true);
    expect(path.reason).toBe("ok");
    expect(feedScrollBehavior(false)).toBe("smooth");
  });

  it("repeat of the same filter stays put", () => {
    const cur = { scope: "all", type: "all", cats: [] as string[] };
    expect(planFeedFilterMotion(cur, cur).reason).toBe("repeat");
    expect(planFeedFilterMotion(cur, cur).apply).toBe(false);
    expect(planFeedFilterMotion(cur, cur).scrollToTop).toBe(false);
  });

  it("empty cats vs empty cats is still a repeat", () => {
    const path = planFeedFilterMotion(
      { scope: "friends", type: "training", cats: [] },
      { scope: "friends", type: "training", cats: [] },
    );
    expect(path.reason).toBe("repeat");
    expect(path.apply).toBe(false);
  });

  it("reduced motion still applies but without animate", () => {
    const path = planFeedFilterMotion(
      { scope: "all", type: "all", cats: [] },
      { scope: "friends", type: "all", cats: [] },
      true,
    );
    expect(path.apply).toBe(true);
    expect(path.animate).toBe(false);
    expect(path.reducedMotion).toBe(true);
    expect(path.scrollToTop).toBe(true);
    expect(feedScrollBehavior(true)).toBe("auto");
  });
});

describe("пользовательский путь на узком экране", () => {
  const snap = { cardCount: 3, windowKey: "feed:1,feed:2,feed:3" };

  it("320px: stacked, in-bounds, 44px tap, swipe-y, no horizontal overflow", () => {
    const path = planFeedNarrowPath(320, snap);
    expect(path.visible).toBe(true);
    expect(path.reason).toBe("ok");
    expect(path.narrow).toBe(true);
    expect(path.stacked).toBe(true);
    expect(path.actionFullWidth).toBe(true);
    expect(path.compactHeader).toBe(true);
    expect(path.filtersScrollX).toBe(true);
    expect(path.reactionsScrollX).toBe(true);
    expect(path.pullToRefresh).toBe(true);
    expect(path.swipeAxis).toBe("y");
    expect(path.tapPx).toBeGreaterThanOrEqual(NARROW_FEED_MIN_TAP_PX);
    expect(path.maxWidthPx).toBe(320 - NARROW_FEED_GUTTER_PX * 2);
    expect(path.maxWidthPx).toBeLessThanOrEqual(320);
    expect(path.overflowsHorizontally).toBe(false);
    expect(isNarrowFeedViewport(320)).toBe(true);
  });

  it("wide preview keeps the compact column (old scenario)", () => {
    const path = planFeedNarrowPath(768, snap);
    expect(path.visible).toBe(true);
    expect(path.narrow).toBe(false);
    expect(path.stacked).toBe(false);
    expect(path.actionFullWidth).toBe(false);
    expect(path.filtersScrollX).toBe(false);
    expect(path.compactHeader).toBe(false);
    expect(path.overflowsHorizontally).toBe(false);
    expect(path.swipeAxis).toBe("both");
    expect(path.tapPx).toBeGreaterThanOrEqual(NARROW_FEED_MIN_TAP_PX);
  });

  it("empty input on a phone does not open a feed path", () => {
    expect(canShowFeedNarrow(null).reason).toBe("empty");
    expect(canShowFeedNarrow(undefined).reason).toBe("empty");
    expect(canShowFeedNarrow({ cardCount: -1 }).reason).toBe("empty");
    const path = planFeedNarrowPath(320, null);
    expect(path.visible).toBe(false);
    expect(path.reason).toBe("empty");
    expect(path.narrow).toBe(true);
    expect(path.stacked).toBe(true);
  });

  it("повтор after dismiss stays closed on a narrow screen", () => {
    const key = feedWindowKey([item({ id: 1 }), item({ id: 2, source: "message" })]);
    expect(key).toBe("feed:1,message:2");
    const path = planFeedNarrowPath(360, { cardCount: 2, windowKey: key }, key);
    expect(path.visible).toBe(false);
    expect(path.reason).toBe("repeat");
    expect(path.narrow).toBe(true);
  });
});

describe("карточка тренировки на узком экране", () => {
  const workout = { name: "Анна", activity: "Велосипед", comment: "Доехала до фонтана" };

  it("320px: stacked, in-bounds, 44px tap, no horizontal overflow", () => {
    const path = planActivityCardNarrowPath(320, workout);
    expect(path.visible).toBe(true);
    expect(path.reason).toBe("ok");
    expect(path.narrow).toBe(true);
    expect(path.stacked).toBe(true);
    expect(path.actionFullWidth).toBe(true);
    expect(path.tapPx).toBeGreaterThanOrEqual(NARROW_FEED_MIN_TAP_PX);
    expect(path.maxWidthPx).toBe(320 - NARROW_FEED_GUTTER_PX * 2);
    expect(path.overflowsHorizontally).toBe(false);
    expect(path.commentExpandable).toBe(true);
    expect(canShowActivityCard(workout).reason).toBe("ok");
  });

  it("wide preview keeps the compact row (old scenario)", () => {
    const path = planActivityCardNarrowPath(768, workout);
    expect(path.visible).toBe(true);
    expect(path.narrow).toBe(false);
    expect(path.stacked).toBe(false);
    expect(path.actionFullWidth).toBe(false);
    expect(path.overflowsHorizontally).toBe(false);
  });

  it("empty input on a phone does not open a card", () => {
    const path = planActivityCardNarrowPath(320, { name: "", activity: "", comment: "" });
    expect(path.visible).toBe(false);
    expect(path.reason).toBe("empty");
    expect(canShowActivityCard(null).reason).toBe("empty");
  });

  it("повтор after dismiss stays closed on a narrow screen", () => {
    const path = planActivityCardNarrowPath(360, workout, activityCardKey(workout));
    expect(path.visible).toBe(false);
    expect(path.reason).toBe("repeat");
  });
});
