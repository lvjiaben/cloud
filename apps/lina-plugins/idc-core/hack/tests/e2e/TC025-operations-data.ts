import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { OperationsDataPage } from "../pages/OperationsDataPage";

const asOf = Date.parse("2026-10-10T04:00:00Z");
const monthStart = Date.parse("2026-09-30T16:00:00Z");
const dayStart = Date.parse("2026-10-09T16:00:00Z");
function snapshot(currencyCode = "CNY") {
  return {
    currencies: ["CNY", "JPY"],
    currencyCode,
    currencyExponent: currencyCode === "JPY" ? 0 : 2,
    timezone: "Asia/Shanghai",
    asOf,
    periods: [
      {
        key: "today",
        startAt: dayStart,
        endAt: asOf,
        incomeMinor: "12345",
        paidOrders: 3,
      },
      {
        key: "yesterday",
        startAt: dayStart - 86400000,
        endAt: dayStart,
        incomeMinor: "0",
        paidOrders: 0,
      },
      {
        key: "month",
        startAt: monthStart,
        endAt: asOf,
        incomeMinor: "56789",
        paidOrders: 17,
      },
      {
        key: "all",
        startAt: 0,
        endAt: asOf,
        incomeMinor: "900719925474099301",
        paidOrders: 123,
      },
    ],
    trend: Array.from({ length: 31 }, (_, index) => ({
      date: `2026-10-${String(index + 1).padStart(2, "0")}`,
      previousDate:
        index < 30 ? `2026-09-${String(index + 1).padStart(2, "0")}` : "",
      startAt: monthStart + index * 86400000,
      endAt: monthStart + (index + 1) * 86400000,
      previousStartAt: index < 30 ? monthStart - (30 - index) * 86400000 : 0,
      previousEndAt: index < 30 ? monthStart - (29 - index) * 86400000 : 0,
      currentMinor: index === 9 ? "12345" : "0",
      currentCount: index === 9 ? 3 : 0,
      previousCount: index < 30 ? 2 : null,
    })),
    products: [
      {
        productNo: "prd_ui_001",
        nameZhCN: "香港云服务器",
        nameEnUS: "Hong Kong cloud server",
        quantity: 8,
      },
    ],
    customers: { today: 2, total: 12, online: 3 },
    expiringServers: 4,
    tickets: { pending: 5, processed: 7 },
  };
}
async function screenshot(page: import("@host-tests/support/playwright").Page, name: string) {
  const directory = resolve("../../temp/20261010");
  await mkdir(directory, { recursive: true });
  await page.screenshot({
    path: resolve(directory, `${Date.now()}-operations-${name}.png`),
    fullPage: true,
  });
}

test.describe("TC-25 运营数据首页", () => {
  test("TC-25a: 独立币种、精确到账金额、零值与接口失败恢复", async ({
    adminPage,
  }) => {
    let fail = false;
    const currencies: string[] = [];
    await adminPage.route("**/admin/operations/data?*", async (route) => {
      const currency =
        new URL(route.request().url()).searchParams.get("currencyCode") ||
        "CNY";
      currencies.push(currency);
      await route.fulfill(
        fail
          ? { status: 503, json: { code: 50, message: "Unavailable" } }
          : { json: { code: 0, data: snapshot(currency) } },
      );
    });
    await adminPage.route("**/admin/operations/data", (route) =>
      route.fulfill({ json: { code: 0, data: snapshot() } }),
    );
    const view = new OperationsDataPage(adminPage);
    await view.goto();
    await expect(
      view.root().getByRole("heading", { name: "运营数据", exact: true }),
    ).toBeVisible();
    await expect(view.period("today")).toContainText("123.45");
    await expect(view.period("yesterday")).toContainText("0.00");
    await expect(view.period("all")).toContainText("9,007,199,254,740,993.01");
    await expect(
      view.root().getByTestId("operations-trend").locator("canvas"),
    ).toBeVisible();
    await expect(
      view.root().getByTestId("operations-products").locator("canvas"),
    ).toBeVisible();
    for (const label of [
      "本月交易趋势",
      "本月商品销量排行",
      "今日新增客户",
      "累计客户",
      "在线客户",
      "即将到期服务器",
      "待处理工单",
      "已处理工单",
    ]) {
      await expect(view.root()).toContainText(label);
    }
    await expect(view.root()).not.toContainText("plugin.idc-core.");
    await screenshot(adminPage, "loaded");
    await view
      .root()
      .getByTestId("operations-products")
      .scrollIntoViewIfNeeded();
    await screenshot(adminPage, "products");
    await view.selectCurrency("JPY");
    await expect(view.period("today")).toContainText("12,345");
    expect(currencies.at(-1)).toBe("JPY");
    fail = true;
    await view.refresh();
    await expect(view.root().getByRole("alert")).toContainText(
      "运营数据加载失败，请重试。",
    );
    await expect(view.period("today")).toHaveCount(0);
    await screenshot(adminPage, "failed");
    fail = false;
    await view.retry();
    await expect(view.period("today")).toContainText("12,345");
    await adminPage.setViewportSize({ width: 390, height: 844 });
    await expect
      .poll(() =>
        adminPage.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      )
      .toBe(true);
    await screenshot(adminPage, "mobile");
  });

  test("TC-25b: 指标跳转携带原统计范围、币种和快照时间", async ({
    adminPage,
  }) => {
    await adminPage.route(/\/admin\/operations\/data(?:\?|$)/u, (route) =>
      route.fulfill({ json: { code: 0, data: snapshot() } }),
    );
    await adminPage.route(
      /\/admin\/operations\/data\/records(?:\?|$)/u,
      (route) =>
        route.fulfill({
          json: {
            code: 0,
            data: { items: [], total: 0, page: 1, pageSize: 20 },
          },
        }),
    );
    const view = new OperationsDataPage(adminPage);
    const cases = [
      {
        action: () => view.income("yesterday"),
        path: "/idc/finance",
        metric: "income",
        start: String(dayStart - 86400000),
        end: String(dayStart),
      },
      {
        action: () => view.orders("month"),
        path: "/idc/orders",
        metric: "orders",
        start: String(monthStart),
        end: String(asOf),
      },
      {
        action: () => view.metric("今日新增客户"),
        path: "/idc/customers",
        metric: "customers",
        start: String(dayStart),
        end: String(asOf),
      },
      {
        action: () => view.metric("累计客户"),
        path: "/idc/customers",
        metric: "customers",
      },
      {
        action: () => view.metric("在线客户"),
        path: "/idc/customers",
        metric: "customers",
        status: "online",
      },
      {
        action: () => view.metric("即将到期服务器"),
        path: "/idc/resource/server-accessories",
        metric: "servers",
      },
      {
        action: () => view.metric("待处理工单"),
        path: "/idc/support",
        metric: "tickets",
        status: "pending",
      },
      {
        action: () => view.metric("已处理工单"),
        path: "/idc/support",
        metric: "tickets",
        status: "processed",
      },
    ];
    for (const item of cases) {
      await view.goto();
      await expect(view.period("today")).toContainText("123.45");
      await item.action();
      await expect
        .poll(() => new URL(adminPage.url()).pathname)
        .toBe(`/admin${item.path}`);
      const query = new URL(adminPage.url()).searchParams;
      expect(query.get("operationsMetric")).toBe(item.metric);
      expect(query.get("currencyCode")).toBe("CNY");
      expect(query.get("asOf")).toBe(String(asOf));
      if (item.start) expect(query.get("startAt")).toBe(item.start);
      if (item.end) expect(query.get("endAt")).toBe(item.end);
      if (item.status) expect(query.get("status")).toBe(item.status);
    }
    await view.goto();
    await expect(
      view.root().getByTestId("operations-trend").locator("canvas"),
    ).toBeVisible();
    await view.chartIncome();
    await expect
      .poll(() => new URL(adminPage.url()).searchParams.get("operationsMetric"))
      .toBe("income");
    expect(new URL(adminPage.url()).searchParams.get("startAt")).toBe(
      String(dayStart),
    );
    expect(new URL(adminPage.url()).searchParams.get("endAt")).toBe(
      String(dayStart + 86400000),
    );
    await view.goto();
    await expect(
      view.root().getByTestId("operations-products").locator("canvas"),
    ).toBeVisible();
    await view.chartProduct();
    await expect
      .poll(() => new URL(adminPage.url()).searchParams.get("operationsMetric"))
      .toBe("products");
    expect(new URL(adminPage.url()).searchParams.get("productNo")).toBe(
      "prd_ui_001",
    );
    expect(new URL(adminPage.url()).searchParams.get("startAt")).toBe(
      String(monthStart),
    );
  });
});
