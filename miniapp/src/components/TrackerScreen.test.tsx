// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { TrackerTask } from "../lib/trackerApi";

const trackerList = vi.fn();
const trackerRefresh = vi.fn();
const trackerAuthors = vi.fn();

vi.mock("../lib/trackerApi", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/trackerApi")>();
  return {
    ...actual,
    trackerList: (...args: unknown[]) => trackerList(...args),
    trackerRefresh: (...args: unknown[]) => trackerRefresh(...args),
    trackerAuthors: (...args: unknown[]) => trackerAuthors(...args),
    trackerAvatarUrl: () => "",
  };
});

import { TrackerScreen } from "./TrackerScreen";

afterEach(() => {
  cleanup();
  trackerList.mockReset();
  trackerRefresh.mockReset();
  trackerAuthors.mockReset();
});

const pending: TrackerTask = {
  id: 11,
  num: 1,
  prompt: "починить кнопку обновить",
  repo: "",
  when: "20.08 09:19",
  repeat: "разово",
  kind: "task",
  status: "pending",
  status_label: "Ожидает",
  status_icon: "⏳",
  done: false,
  active: true,
  can_delete: true,
  auto_review: false,
  manual_qa: false,
  fast_track: false,
  error: "",
  has_result: false,
  phase: "todo",
  qa_status: null,
  qa_label: "",
  qa_icon: "",
  auto_qa_running: false,
  dev_column: "todo",
  qa_column: null,
  handed_to_qa: false,
  attachments_count: 0,
  has_attachments: false,
  author_id: 42,
};

const running: TrackerTask = {
  ...pending,
  status: "running",
  status_label: "В работе",
  status_icon: "🔧",
  phase: "doing",
  dev_column: "doing",
};

const reviewed: TrackerTask = {
  ...running,
  status: "reviewing",
  status_label: "Review",
  status_icon: "👀",
  phase: "review",
  dev_column: "review",
  has_result: true,
  result: "⏰ Задача #1 выполнена.\n\nГотово.\n- Подпись теперь только «сгорит через …».",
  live_step: "Агент сдал результат",
};

describe("TrackerScreen refresh button", () => {
  it("calls refresh and moves a due card into work", async () => {
    trackerList.mockResolvedValue({ tasks: [pending], started: 0 });
    trackerRefresh.mockResolvedValue({ tasks: [running], started: 1 });
    trackerAuthors.mockResolvedValue([]);
    const alerts: string[] = [];

    render(<TrackerScreen initData="admin" showAlert={(t) => alerts.push(t)} />);
    await waitFor(() => expect(screen.getByText("#1")).toBeTruthy());
    expect(document.querySelector('[data-col="todo"]')?.textContent).toContain("#1");
    expect(trackerList).toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Обновить" }));
    await waitFor(() => expect(trackerRefresh).toHaveBeenCalledWith("admin"));
    await waitFor(() => {
      expect(document.querySelector('[data-col="doing"]')?.textContent).toContain("#1");
    });
    expect(alerts.some((a) => a.includes("Взяли 1"))).toBe(true);
  });

  it("shows a completed result on the Review column, not in work", async () => {
    trackerList.mockResolvedValue({ tasks: [reviewed], started: 0 });
    trackerAuthors.mockResolvedValue([]);

    render(<TrackerScreen initData="admin" showAlert={() => undefined} />);
    await waitFor(() => expect(screen.getByText("#1")).toBeTruthy());
    expect(document.querySelector('[data-col="doing"]')?.textContent).not.toContain("#1");
    expect(document.querySelector('[data-col="review"]')?.textContent).toContain("#1");
    expect(document.querySelector('[data-col="review"]')?.textContent).toContain("👀");
    expect(document.querySelector(".tracker-card__live--result")?.textContent).toContain("выполнена");
  });
});
