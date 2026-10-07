import { readFileSync } from "node:fs";
import type { Page } from "../../../../../../hack/tests/fixtures/auth";
import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { IsolatedMountPage } from "../../../../vben-adapter/hack/tests/pages/IsolatedMountPage";
import { ProviderUsagePage } from "../pages/ProviderUsagePage";
import { ProviderCatalogPage } from "../pages/ProviderCatalogPage";

const locales = Object.fromEntries(["zh-CN", "en-US"].map(locale => [locale,
  JSON.parse(readFileSync(new URL(`../../../manifest/i18n/${locale}/plugin.json`, import.meta.url), "utf8")) as Record<string, string>,
]));
const version = readFileSync(new URL("../../../plugin.yaml", import.meta.url), "utf8").match(/^version: (\S+)$/mu)?.[1];

/** Bounded fixture responses exercise the UI; they never reach a supplier or write business data. */
async function installCatalog(page: Page) {
  const state = { listFailure: false, detailFailure: false, pages: [] as URLSearchParams[], details: [] as string[], writes: 0 };
  const products = Array.from({ length: 121 }, (_, index) => ({ reference: String(index + 1), name: `上游商品 ${index + 1}`, description: "上游销售说明" }));
  await page.route("**/x/idc-resource/api/v1/admin/resource/provider-configurations**", async route => {
    const url = new URL(route.request().url());
    if (route.request().method() !== "GET") {
      ++state.writes;
      await route.fulfill({ status: 405, json: { code: 405, message: "测试仅允许读取" } });
      return;
    }
    if (url.pathname.endsWith("/catalog-products")) {
      state.pages.push(url.searchParams);
      if (state.listFailure) { await route.fulfill({ status: 503, json: { code: 503, message: "测试读取失败" } }); return; }
      const pageNo = Number(url.searchParams.get("page") || 1), pageSize = Number(url.searchParams.get("pageSize") || 20);
      const matched = products.filter(product => product.name.includes(url.searchParams.get("keyword") || ""));
      await route.fulfill({ json: { code: 0, data: { items: matched.slice((pageNo - 1) * pageSize, pageNo * pageSize), total: matched.length } } });
      return;
    }
    if (url.pathname.endsWith("/catalog-product")) {
      const reference = url.searchParams.get("reference") || "";
      state.details.push(reference);
      if (state.detailFailure) { await route.fulfill({ status: 503, json: { code: 503, message: "测试价格读取失败" } }); return; }
      const product = products.find(item => item.reference === reference);
      const price = { currencyCode: "CNY", period: "month", periodCount: 1, amount: "92233720368547758.07", setupFee: "0.005" };
      await route.fulfill({ json: { code: 0, data: { item: { product, basePrices: [price], options: [{ reference: "hidden-group", name: "内存", choices: [{ reference: "hidden-choice", name: "8 GB", prices: [{ ...price, amount: "12.345" }] }] }], priceScheduleComplete: false } } } });
      return;
    }
    await route.fulfill({ json: { code: 0, data: { channelTotal: 1, poolTotal: 0, pools: [], providerCandidates: [], channels: [{ channelNo: "CHN_test", interfaceInstanceNo: "pfi_0123456789abcdef0123456789abcdef", displayName: "接口 1", endpoint: "https://supplier.example.invalid", providerPluginId: "test", catalogAvailable: true, status: "enabled", secretFields: [], interfaceConfiguration: { version: "v1", fields: [] }, version: 1, usage: { activeInstances: 3, totalInstances: 5, configuredProducts: 2 } }] } } });
  });
  return state;
}

async function installHostPage(page: Page) {
  const messages: Record<string, any> = {};
  for (const [key, value] of Object.entries(locales["zh-CN"]!)) {
    let branch = messages;
    const parts = key.split(".");
    for (const part of parts.slice(0, -1)) branch = branch[part] ??= {};
    branch[parts.at(-1)!] = value;
  }
  await page.route("**/api/v1/i18n/runtime/messages**", route => route.fulfill({ json: { code: 0, data: { messages } } }));
  await page.route("**/api/v1/user/info", async route => {
    const response = await route.fetch(), payload = await response.json();
    payload.data.permissions = [...(payload.data.permissions ?? []), "idc:admin:resource:configuration:list", "idc:admin:resource:configuration:write"];
    await route.fulfill({ json: payload });
  });
  await page.route("**/api/v1/menus/all", route => route.fulfill({ json: { code: 0, data: { list: [{ name: "IDCOperations", path: "/idc", component: "BasicLayout", meta: { title: "IDC 运营" }, children: [{ name: "IDCProviderConfiguration", path: "/idc/resource/provider-configuration", component: "system/plugin/dynamic-page", meta: { title: "plugin.idc-resource.providerConfiguration.title" } }] }] } } }));
}

test.describe("TC-13 上游商品只读价格与分页", () => {
  test("TC-13a: 独立入口使用完整分页、Vben 搜索、分项精确价格与双语", async ({ adminPage }) => {
    const state = await installCatalog(adminPage);
    const surface = new IsolatedMountPage(adminPage), view = new ProviderCatalogPage(adminPage);
    try {
      await surface.mount(`/x-assets/idc-resource/${version}/mount.js`, "/idc/resource/provider-configuration", "zh-CN", locales["zh-CN"]);
      await surface.click("详情");
      await view.open();
      await expect(view.row("上游商品 1")).toBeVisible();
      await expect(view.pager).toContainText("121");
      expect(state.details).toHaveLength(0);
      await surface.screenshot("provider-catalog-isolated-first-page");
      await view.numberPage(3);
      await expect(view.row("上游商品 41")).toBeVisible();
      expect(state.pages.at(-1)?.get("page")).toBe("3");
      await view.viewPrices("上游商品 41");
      await expect(view.prices).toContainText("92233720368547758.07");
      await expect(view.prices).toContainText("12.345");
      await expect(view.prices).toContainText("基础价格不含配置附加费");
      await expect(view.prices).toContainText("不能作为完整报价");
      await expect(view.prices).not.toContainText("hidden-group");
      await expect(view.prices).not.toContainText("hidden-choice");
      await surface.screenshot("provider-catalog-isolated-price-components");
      const requestsBeforeSearch = state.pages.length;
      expect((await view.search("121")).status()).toBe(200);
      await expect(view.row("上游商品 121")).toBeVisible();
      expect(state.pages).toHaveLength(requestsBeforeSearch + 1);
      expect(state.pages.at(-1)?.get("page")).toBe("1");
      expect(state.pages.at(-1)?.get("keyword")).toBe("121");
      await expect(view.row("上游商品 41")).toHaveCount(0);
      await expect(view.grid.locator(".vxe-pager--total")).toContainText("1");
      await view.viewPrices("上游商品 121");
      await surface.setLocale("en-US", locales["en-US"]);
      await expect(view.grid).toContainText("Product name");
      await expect(view.prices).toContainText("Option surcharges");
      await expect(view.root).not.toContainText("plugin.idc-resource");
      expect(state.writes).toBe(0);
      await surface.screenshot("provider-catalog-isolated-search-en");
    } finally { await surface.dispose(); }
  });

  test("TC-13b: 宿主接入入口清除读取失败后的旧商品和旧价格并可重试", async ({ adminPage }) => {
    const state = await installCatalog(adminPage);
    await installHostPage(adminPage);
    const parent = new ProviderUsagePage(adminPage), view = new ProviderCatalogPage(adminPage), screenshots = new IsolatedMountPage(adminPage);
    await parent.goto();
    await view.expectDirectoryPagerVisible();
    await parent.details("接口 1");
    await view.open();
    await expect(view.row("上游商品 1")).toBeVisible();
    await view.viewPrices("上游商品 1");
    await expect(view.prices).toContainText("92233720368547758.07");
    state.listFailure = true;
    const requestsBeforeFailure = state.pages.length;
    expect((await view.search("")).status()).toBe(503);
    await expect(view.root).toContainText("上游商品读取失败");
    expect(state.pages).toHaveLength(requestsBeforeFailure + 1);
    expect(state.pages.at(-1)?.get("page")).toBe("1");
    expect(state.pages.at(-1)?.get("keyword") ?? "").toBe("");
    await expect(view.row("上游商品 1")).toHaveCount(0);
    await expect(view.prices).toHaveCount(0);
    await screenshots.screenshot("provider-catalog-page-list-failed");
    state.listFailure = false;
    expect((await view.search("")).status()).toBe(200);
    await expect(view.row("上游商品 1")).toBeVisible();
    expect(state.pages).toHaveLength(requestsBeforeFailure + 2);
    state.detailFailure = true;
    await view.viewPrices("上游商品 1");
    await expect(view.root).toContainText("商品价格读取失败");
    await expect(view.prices).toHaveCount(0);
    state.detailFailure = false;
    await view.viewPrices("上游商品 1");
    await expect(view.prices).toContainText("12.345");
    await expect(view.root).not.toContainText("商品价格读取失败");
    await expect(adminPage.getByText("测试价格读取失败", { exact: true })).toBeHidden();
    expect(state.writes).toBe(0);
    await screenshots.screenshot("provider-catalog-page-recovered");
    await view.close();
  });
});
