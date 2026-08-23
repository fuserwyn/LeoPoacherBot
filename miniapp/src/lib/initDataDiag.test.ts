// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { reportInitDataSource, __resetInitDataDiagForTests } from "./initDataDiag";

beforeEach(() => {
  __resetInitDataDiagForTests();
  vi.stubEnv("VITE_MINIAPP_API_URL", "https://api.example.test");
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(new Response(null, { status: 204 }))));
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

describe("reportInitDataSource", () => {
  it("шлёт beacon на /api/miniapp/diag/init-source с источником и has_init", () => {
    reportInitDataSource({ source: "hash", initData: "query_id=1&hash=x", platform: "ios", tgVersion: "7.0", triesMs: 300 });
    const fetchMock = fetch as unknown as ReturnType<typeof vi.fn>;
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, opts] = fetchMock.mock.calls[0];
    expect(url).toBe("https://api.example.test/api/miniapp/diag/init-source");
    const sent = JSON.parse((opts as RequestInit).body as string);
    expect(sent.source).toBe("hash");
    expect(sent.has_init).toBe(true);
    expect(sent.platform).toBe("ios");
    expect(sent.tries_ms).toBe(300);
  });

  it("шлёт ровно один beacon за сессию (повторные игнорируются)", () => {
    reportInitDataSource({ source: "none", initData: "" });
    reportInitDataSource({ source: "webapp", initData: "x" });
    const fetchMock = fetch as unknown as ReturnType<typeof vi.fn>;
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const sent = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string);
    expect(sent.source).toBe("none");
    expect(sent.has_init).toBe(false);
  });

  it("без VITE_MINIAPP_API_URL ничего не шлёт", () => {
    vi.stubEnv("VITE_MINIAPP_API_URL", "");
    reportInitDataSource({ source: "none", initData: "" });
    const fetchMock = fetch as unknown as ReturnType<typeof vi.fn>;
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
