// UI contract fixture only; authoritative stock and settlement are PostgreSQL tests.
import type { Page } from "@host-tests/support/playwright";
import type { TrafficPackage } from "../../../frontend/pages/traffic-package-client";

export async function installTrafficPackageMock(page: Page) {
  const rows: TrafficPackage[] = Array.from({ length: 25 }, (_, index) => ({
    packageNo: `tpk_ui_${index + 1}`, version: 1, nameZhCN: `回归流量包 ${index + 1}`,
    nameEnUS: `Traffic package ${index + 1}`, currencyCode: "CNY", trafficGb: 100,
    amountMinor: 1250, enabled: true, stockQuantity: 100, occupiedQuantity: 30,
    remainingQuantity: 70, productNos: ["prd_test_server"], publishedAt: 1788470400000,
  }));
  const state = { rows, writes: [] as Record<string, unknown>[], failNext: false, lookupFailure: false, missingProductName: false, pages: [] as number[] };
  await page.route(/\/admin\/resource\/traffic-package-products(?:\?|$)/u, async route => {
    if (state.lookupFailure) {
      await route.fulfill({ status: 503, json: { code: 50, message: "商品选项暂时不可用" } });
      return;
    }
    await route.fulfill({ json: { code: 0, data: { items: state.missingProductName ? [] : [{ productNo: "prd_test_server", nameZhCN: "回归云服务器", nameEnUS: "Test cloud server" }] } } });
  });
  await page.route(/\/admin\/resource\/traffic-packages(?:\/[^?]+)?(?:\?|$)/u, async route => {
    const request = route.request();
    if (request.method() === "GET") {
      const url = new URL(request.url());
      const current = Number(url.searchParams.get("page") || 1);
      const size = Number(url.searchParams.get("pageSize") || 20);
      const keyword = url.searchParams.get("keyword") || "";
      state.pages.push(current);
      const matching = rows.filter(row => row.nameZhCN.includes(keyword));
      await route.fulfill({ json: { code: 0, data: { items: matching.slice((current - 1) * size, current * size), total: matching.length } } });
      return;
    }
    const input = request.postDataJSON() as Record<string, unknown>;
    state.writes.push(input);
    if (state.failNext) {
      state.failNext = false;
      await route.fulfill({ status: 409, json: { code: 50, message: "流量套餐已变更，请重新确认" } });
      return;
    }
    const number = decodeURIComponent(new URL(request.url()).pathname.split("/").at(-1)!);
    const previous = request.method() === "PUT" ? rows.find(row => row.packageNo === number) : undefined;
    const saved = {
      ...input, packageNo: previous?.packageNo || `tpk_ui_new_${rows.length}`, version: (previous?.version || 0) + 1,
      occupiedQuantity: previous?.occupiedQuantity || 0,
      remainingQuantity: input.stockQuantity == null ? null : Math.max(0, Number(input.stockQuantity) - (previous?.occupiedQuantity || 0)),
      publishedAt: 1788470400000,
    } as unknown as TrafficPackage;
    if (previous) rows.splice(rows.indexOf(previous), 1, saved); else rows.unshift(saved);
    await route.fulfill({ json: { code: 0, data: { package: saved } } });
  });
  return state;
}
