import type { Page } from "@host-tests/support/playwright";

export async function installCustomerSupportListMock(page: Page) {
  const state = {
    failNext: false,
    denyNext: false,
    requests: [] as Array<{ cursor: string; page: string; query: string; limit: string }>,
  };
  const records = Array.from({ length: 12 }, (_, index) => ({
    ticketNo: `TK2026090400${String(index + 1).padStart(2, "0")}`,
    subject: index === 0 ? "网络连接需要协助" : `客户工单列表回归 ${index + 1}`,
    category: {
      categoryNo: "CAT_NETWORK",
      nameZhCN: "网络连接",
      nameEnUS: "Networking",
    },
    status: ["new", "in_progress", "waiting_customer", "resolved", "closed"][
      index % 5
    ],
    priority: "normal",
    currentRound: 1,
    version: 1,
    createdAt: 1_788_470_400_000 + index * 60_000,
    updatedAt: 1_788_470_400_000 + index * 60_000,
    lastActivityAt: 1_788_470_400_000 + index * 60_000,
  }));
  await page.route(/\/account\/support\/tickets(?:\?|$)/u, async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    const url = new URL(route.request().url());
    const query = url.searchParams.get("query") || "";
    const cursor = url.searchParams.get("cursor") || "";
    const currentPage = url.searchParams.get("page") || "";
    const limit = Number(url.searchParams.get("limit") || "10");
    state.requests.push({
      cursor,
      page: currentPage,
      query,
      limit: url.searchParams.get("limit") || "",
    });
    if (state.failNext) {
      state.failNext = false;
      await route.fulfill({
        status: 503,
        json: { code: 50, message: "暂时无法读取工单，请稍后重试" },
      });
      return;
    }
    if (state.denyNext) {
      state.denyNext = false;
      await route.fulfill({ status: 403, json: { code: 403, message: "无权查看工单" } });
      return;
    }
    const matching = records.filter(
      (item) =>
        !query ||
        item.ticketNo === query ||
        item.subject.toLocaleLowerCase().startsWith(query.toLocaleLowerCase()),
    );
    const offset = currentPage ? (Number(currentPage) - 1) * limit : cursor === "next-10" ? 10 : 0;
    await route.fulfill({
      json: {
        code: 0,
        data: {
          tickets: matching.slice(offset, offset + limit),
          total: matching.length,
          nextCursor: matching.length > offset + limit ? "next-10" : "",
        },
      },
    });
  });
  return state;
}
