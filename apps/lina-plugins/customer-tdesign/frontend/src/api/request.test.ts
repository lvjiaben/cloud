import { beforeEach, afterEach, describe, it, expect, vi } from "vitest";
import { requestClient, saveSession, getSession } from "./request";
let storage: Record<string, string>;
beforeEach(() => {
  storage = {};
  vi.stubGlobal("sessionStorage", {
    getItem: (k: string) => storage[k] || null,
    setItem: (k: string, v: string) => (storage[k] = v),
    removeItem: (k: string) => delete storage[k],
  });
  vi.stubGlobal("window", new EventTarget());
  vi.stubGlobal("location", { origin: "https://test.example" });
  saveSession(null);
});
afterEach(() => vi.unstubAllGlobals());
const ok = (data: unknown) => Response.json({ code: 0, data });
describe("session refresh safety", () => {
  it("refreshes concurrent 401 responses once and replays with the new token", async () => {
    saveSession({ accessToken: "old", refreshToken: "r1" });
    let count = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string | URL, options: RequestInit) => {
        if (String(url).includes("/auth/refresh")) {
          count++;
          await new Promise((r) => setTimeout(r, 5));
          return ok({ accessToken: "new", refreshToken: "r2" });
        }
        return (options.headers as Record<string, string>).Authorization ===
          "Bearer new"
          ? ok({ value: 1 })
          : new Response("", { status: 401 });
      }),
    );
    expect(
      await Promise.all([requestClient.get("/a"), requestClient.get("/b")]),
    ).toEqual([{ value: 1 }, { value: 1 }]);
    expect(count).toBe(1);
  });
  it("cannot restore a session after logout while refresh is in flight", async () => {
    saveSession({ accessToken: "old", refreshToken: "r1" });
    let release: (r: Response) => void = () => {};
    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string | URL) =>
        String(url).includes("/auth/refresh")
          ? new Promise<Response>((r) => (release = r))
          : new Response("", { status: 401 }),
      ),
    );
    const pending = requestClient.get("/a");
    const rejected = expect(pending).rejects.toThrow("登录");
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    saveSession(null);
    release(ok({ accessToken: "new" }));
    await rejected;
    expect(getSession()).toBeNull();
  });
  it("never automatically retries sensitive reveal or identity submissions", async () => {
    for (const endpoint of [
      "/x/idc-resource/api/v1/reveal",
      "/x/idc-compliance/api/v1/personal-verifications",
    ]) {
      saveSession({ accessToken: "old", refreshToken: "r1" });
      const fetcher = vi.fn(async () => new Response("", { status: 401 }));
      vi.stubGlobal("fetch", fetcher);
      await expect(requestClient.post(endpoint, {})).rejects.toThrow("登录");
      expect(fetcher).toHaveBeenCalledTimes(1);
    }
  });
  it("does not treat an HTML fallback response as API success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("<html>fallback</html>")),
    );
    await expect(requestClient.get("/a")).rejects.toThrow("操作未完成");
  });
});

describe("session replacement isolation", () => {
  it("does not share an old account refresh with a newly signed-in account", async () => {
    let finishOld: (r: Response) => void = () => {};
    saveSession({ accessToken: "account-a", refreshToken: "refresh-a" });
    const fetcher = vi.fn(async (url: string | URL, options: RequestInit) => {
      if (String(url).includes("/auth/refresh")) {
        const token = JSON.parse(String(options.body)).refreshToken;
        if (token === "refresh-a")
          return new Promise<Response>((r) => {
            finishOld = r;
          });
        return ok({ accessToken: "account-b-renewed" });
      }
      return (options.headers as Record<string, string>).Authorization ===
        "Bearer account-b-renewed"
        ? ok({ owner: "b" })
        : new Response("", { status: 401 });
    });
    vi.stubGlobal("fetch", fetcher);
    const old = expect(requestClient.get("/old")).rejects.toThrow("登录");
    await vi.waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
    saveSession({ accessToken: "account-b", refreshToken: "refresh-b" });
    const next = requestClient.get("/new");
    const result = next.then(
      (value) => ({ value }),
      (error) => ({ error }),
    );
    await vi.waitFor(() =>
      expect(
        fetcher.mock.calls.some(([url]) => String(url).endsWith("/new")),
      ).toBe(true),
    );
    finishOld(ok({ accessToken: "account-a-renewed" }));
    await old;
    expect(await result).toEqual({ value: { owner: "b" } });
    expect(getSession()?.accessToken).toBe("account-b-renewed");
  });
  it("rejects an old successful response after the session is replaced", async () => {
    let finish: (r: Response) => void = () => {};
    saveSession({ accessToken: "account-a" });
    vi.stubGlobal(
      "fetch",
      vi.fn(
        () =>
          new Promise<Response>((r) => {
            finish = r;
          }),
      ),
    );
    const result = requestClient.get("/private");
    const rejected = expect(result).rejects.toThrow("登录状态已变更");
    saveSession({ accessToken: "account-b" });
    finish(ok({ privateData: "account-a" }));
    await rejected;
    expect(getSession()?.accessToken).toBe("account-b");
  });
  it("keeps the current session after permission denial without refreshing or replaying", async () => {
    saveSession({ accessToken: "current", refreshToken: "refresh" });
    const fetcher = vi.fn(async () => new Response("", { status: 403 }));
    vi.stubGlobal("fetch", fetcher);
    await expect(requestClient.post("/forbidden", {})).rejects.toThrow(
      "当前账号无法办理这项业务",
    );
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(getSession()?.accessToken).toBe("current");
  });
});

describe("same-token login generation", () => {
  it("does not sign out a new login when an older refresh finishes with the same access token", async () => {
    saveSession({ accessToken: "same-token", refreshToken: "old-refresh" });
    let finish: (r: Response) => void = () => {};
    const fetcher = vi.fn(async (url: string | URL) =>
      String(url).includes("/auth/refresh")
        ? new Promise<Response>((r) => {
            finish = r;
          })
        : new Response("", { status: 401 }),
    );
    vi.stubGlobal("fetch", fetcher);
    const old = expect(requestClient.get("/old")).rejects.toThrow("登录");
    await vi.waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
    saveSession({ accessToken: "same-token", refreshToken: "new-refresh" });
    finish(ok({ accessToken: "old-rotated" }));
    await old;
    expect(getSession()).toEqual({
      accessToken: "same-token",
      refreshToken: "new-refresh",
    });
  });
});
