import { afterEach, describe, expect, it, vi } from "vitest";
import {
  listCustomerAnnouncements,
  getCustomerAnnouncement,
  readCustomerAnnouncement,
} from "./announcements";
afterEach(() => vi.unstubAllGlobals());
describe("announcement request routing", () => {
  it("sends the actual fetch URLs to the source plugin namespace", async () => {
    vi.stubGlobal("location", { origin: "https://customer.example" });
    vi.stubGlobal("sessionStorage", { getItem: () => null });
    const fetcher = vi.fn(async (_url: string | URL, _options?: RequestInit) =>
      Response.json({ code: 0, data: { list: [], total: 0 } }),
    );
    vi.stubGlobal("fetch", fetcher);
    await listCustomerAnnouncements(2, 10);
    await getCustomerAnnouncement(321);
    await readCustomerAnnouncement(321);
    expect(fetcher.mock.calls.map((call) => String(call[0]))).toEqual([
      "https://customer.example/x/content-notice/api/v1/notice/announcements?pageNum=2&pageSize=10",
      "https://customer.example/x/content-notice/api/v1/notice/announcements/321",
      "https://customer.example/x/content-notice/api/v1/notice/announcements/321/read",
    ]);
    expect(fetcher.mock.calls[2]?.[1]).toMatchObject({ method: "PUT" });
  });
});
