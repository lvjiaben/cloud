import { readFileSync } from "node:fs";
import type { Page } from "../../../../../../hack/tests/fixtures/auth";
import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { IsolatedMountPage } from "../../../../vben-adapter/hack/tests/pages/IsolatedMountPage";
import { ProviderUsagePage } from "../pages/ProviderUsagePage";
import { ProviderCatalogPage } from "../pages/ProviderCatalogPage";
import { ProviderCatalogImportPage } from "../pages/ProviderCatalogImportPage";

const locales = Object.fromEntries(["zh-CN", "en-US"].map(locale => [locale,
  JSON.parse(readFileSync(new URL(`../../../manifest/i18n/${locale}/plugin.json`, import.meta.url), "utf8")) as Record<string, string>,
]));
const version = readFileSync(new URL("../../../plugin.yaml", import.meta.url), "utf8").match(/^version: (\S+)$/mu)?.[1];
const interfaceNo = "pfi_0123456789abcdef0123456789abcdef";
const categoryNo = "ctg_0123456789abcdef0123456789abcdef";
const product = { reference: "121", name: "上游商品 121", description: "上游销售说明" };
type ImportRequest = {
  requestKey: string; interfaceInstanceNo: string; reference: string; categoryNo: string;
  nameZhCn: string; nameEnUs: string; pricingMode: string; markupPercent?: string;
  prices?: Array<{ currencyCode: string; period: string; periodCount: number; amount: string; setupFee: string }>;
};

/** Every catalog write is intercepted: this verifies UI submission, not real supplier creation. */
async function installImport(page: Page) {
  const state = { failure: false, requests: [] as ImportRequest[], forbidden: [] as string[], categoryReads: 0, details: [] as string[] };
  await page.route("**/x/idc-resource/api/v1/admin/resource/provider-configurations**", async route => {
    const request = route.request(), url = new URL(request.url());
    if (request.method() !== "GET") {
      state.forbidden.push(`${request.method()} ${url.pathname}`);
      await route.fulfill({ status: 405, json: { code: 405 } }); return;
    }
    if (url.pathname.endsWith("/catalog-products")) {
      await route.fulfill({ json: { code: 0, data: { items: [product], total: 1 } } }); return;
    }
    if (url.pathname.endsWith("/catalog-product")) {
      state.details.push(url.searchParams.get("reference") ?? "");
      await route.fulfill({ json: { code: 0, data: { item: { product, basePrices: [{ currencyCode: "CNY", period: "month", periodCount: 1, amount: "100.005", setupFee: "0" }], options: [], priceScheduleComplete: false } } } }); return;
    }
    await route.fulfill({ json: { code: 0, data: { channelTotal: 1, poolTotal: 0, pools: [], providerCandidates: [], channels: [{ channelNo: "CHN_import", interfaceInstanceNo: interfaceNo, displayName: "接口 1", endpoint: "https://supplier.example.invalid", providerPluginId: "test", catalogAvailable: true, status: "enabled", secretFields: [], interfaceConfiguration: { version: "v1", fields: [] }, version: 1, usage: { activeInstances: 3, totalInstances: 5, configuredProducts: 2 } }] } } });
  });
  await page.route("**/x/idc-core/api/v1/admin/catalog/**", async route => {
    const request = route.request(), url = new URL(request.url());
    if (request.method() === "GET" && url.pathname.endsWith("/categories")) {
      state.categoryReads++;
      await route.fulfill({ json: { code: 0, data: { categories: [
        { categoryNo, parentCategoryNo: "", nameZhCn: "云服务器", nameEnUs: "Cloud servers", deleted: false },
        { categoryNo: "ctg_ffffffffffffffffffffffffffffffff", parentCategoryNo: "", nameZhCn: "已删除分类", nameEnUs: "Deleted category", deleted: true },
      ] } } }); return;
    }
    if (request.method() === "POST" && url.pathname.endsWith("/product-imports")) {
      state.requests.push(request.postDataJSON() as ImportRequest);
      if (state.failure) { await route.fulfill({ status: 503, json: { code: 503, message: "Test import unavailable" } }); return; }
      await route.fulfill({ json: { code: 0, data: {
        item: { productNo: "prd_0123456789abcdef0123456789abcdef", versionNo: "pvr_0123456789abcdef0123456789abcdef" },
        missingConfiguration: [{ fieldKey: "private_cpu_binding", nameZhCn: "处理器核心数", nameEnUs: "CPU core count" }, { fieldKey: "private_memory_binding", nameZhCn: "内存容量", nameEnUs: "Memory capacity" }],
        pricingIncomplete: state.requests.at(-1)?.pricingMode === "markup",
      } } }); return;
    }
    state.forbidden.push(`${request.method()} ${url.pathname}`);
    await route.fulfill({ status: 405, json: { code: 405, message: "Only draft import is allowed in this fixture" } });
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
    payload.data.permissions = [...(payload.data.permissions ?? []), "idc:admin:resource:configuration:list", "idc:admin:resource:configuration:write", "idc:admin:catalog:category:list", "idc:admin:catalog:product:create"];
    await route.fulfill({ json: payload });
  });
  await page.route("**/api/v1/menus/all", route => route.fulfill({ json: { code: 0, data: { list: [{ name: "IDCOperations", path: "/idc", component: "BasicLayout", meta: { title: "IDC 运营" }, children: [{ name: "IDCProviderConfiguration", path: "/idc/resource/provider-configuration", component: "system/plugin/dynamic-page", meta: { title: "plugin.idc-resource.providerConfiguration.title" } }] }] } } }));
}

test.describe("TC-14 上游商品显式导入未上架草稿", () => {
  test("TC-14a: 独立入口分类必填、加价百分比和同表单失败重试", async ({ adminPage }) => {
    const state = await installImport(adminPage);
    const surface = new IsolatedMountPage(adminPage), catalog = new ProviderCatalogPage(adminPage), importer = new ProviderCatalogImportPage(adminPage);
    try {
      await surface.mount(`/x-assets/idc-resource/${version}/mount.js`, "/idc/resource/provider-configuration", "zh-CN", locales["zh-CN"]);
      await surface.click("详情"); await catalog.open();
      await expect(catalog.row(product.name)).toBeVisible();
      await expect(importer.entry).toBeDisabled();
      expect(state.details).toEqual([]);
      await catalog.viewPrices(product.name);
      await expect(catalog.prices).toContainText("100.005");
      await importer.open();
      await expect(importer.input("provider-import-name-zh")).toHaveValue(product.name);
      await expect(importer.dialog).toContainText("只创建未上架草稿");
      await importer.markup("10");
      await importer.submit();
      await expect(importer.dialog).toContainText("请输入本地分类");
      expect(state.requests).toHaveLength(0);
      await surface.screenshot("provider-import-required-category");
      await importer.category("云服务器");
      state.failure = true;
      expect((await importer.submitAndWait()).status()).toBe(503);
      await expect(importer.dialog).toContainText("导入未完成");
      await expect(importer.input("provider-import-markup")).toHaveValue("10");
      await expect(importer.input("provider-import-name-zh")).toHaveValue(product.name);
      await expect(importer.dialog.getByTestId("provider-import-category")).toContainText("云服务器");
      expect(state.requests).toHaveLength(1);
      const failed = state.requests[0]!;
      expect(failed).toEqual({ requestKey: expect.any(String), interfaceInstanceNo: interfaceNo, reference: "121", categoryNo, nameZhCn: product.name, nameEnUs: product.name, pricingMode: "markup", markupPercent: "10" });
      expect(failed.requestKey).toMatch(/^[\da-f-]{36}$/u);
      await surface.screenshot("provider-import-failed-preserved");
      state.failure = false;
      expect((await importer.submitAndWait()).status()).toBe(200);
      await expect(importer.result).toContainText("商品草稿已创建，尚未上架");
      expect(state.requests).toHaveLength(2);
      expect(state.requests[1]).toEqual(failed);
      await expect(importer.result).toContainText("待补齐配置：处理器核心数、内存容量");
      await expect(importer.result).toContainText("草稿中的价格仍需补齐");
      await expect(importer.result).not.toContainText("private_");
      await expect(importer.result).not.toContainText("plugin.idc-resource");
      await expect(importer.confirm).toHaveCount(0);
      expect(state.forbidden).toEqual([]);
      await surface.screenshot("provider-import-draft-result-zh");
    } finally { await surface.dispose(); }
  });

  test("TC-14b: 宿主入口手填多条售价、不能移除最后一条且无自动发布", async ({ adminPage }) => {
    const state = await installImport(adminPage);
    await installHostPage(adminPage);
    const parent = new ProviderUsagePage(adminPage), catalog = new ProviderCatalogPage(adminPage), importer = new ProviderCatalogImportPage(adminPage), screenshots = new IsolatedMountPage(adminPage);
    await parent.goto(); await catalog.expectDirectoryPagerVisible(); await parent.details("接口 1"); await catalog.open();
    await expect(catalog.row(product.name)).toBeVisible();
    await catalog.viewPrices(product.name); await expect(catalog.prices).toContainText("100.005");
    await importer.open(); await importer.category("云服务器"); await importer.manual();
    await expect(importer.removePrice).toBeDisabled();
    await expect(importer.input("provider-import-markup")).toHaveCount(0);
    await importer.price(0, { currency: "CNY", periodLabel: "个月", count: "1", amount: "92233720368547758.07", setup: "0.005" });
    await importer.addPrice();
    await importer.price(1, { currency: "USD", periodLabel: "年", count: "1", amount: "12.345", setup: "0" });
    await importer.addPrice();
    await expect(importer.input("provider-import-currency-2")).toBeVisible();
    await importer.removeLastPrice();
    await expect(importer.input("provider-import-currency-2")).toHaveCount(0);
    await expect(importer.input("provider-import-amount-0")).toHaveValue("92233720368547758.07");
    await expect(importer.input("provider-import-amount-1")).toHaveValue("12.345");
    await screenshots.screenshot("provider-import-manual-prices");
    expect((await importer.submitAndWait()).status()).toBe(200);
    await expect(importer.result).toContainText("商品草稿已创建，尚未上架");
    expect(state.requests).toHaveLength(1);
    expect(state.requests[0]).toEqual({ requestKey: expect.any(String), interfaceInstanceNo: interfaceNo, reference: "121", categoryNo, nameZhCn: product.name, nameEnUs: product.name, pricingMode: "manual", prices: [
      { currencyCode: "CNY", period: "month", periodCount: 1, amount: "92233720368547758.07", setupFee: "0.005" },
      { currencyCode: "USD", period: "year", periodCount: 1, amount: "12.345", setupFee: "0" },
    ] });
    await expect(importer.result).not.toContainText("草稿中的价格仍需补齐");
    await expect(importer.result).toContainText("处理器核心数");
    expect(state.forbidden).toEqual([]);
    await screenshots.screenshot("provider-import-manual-draft-result");
    await importer.close();
  });

  test("TC-14c: 英文缺项提示、取消后重开使用新的提交标识", async ({ adminPage }) => {
    const state = await installImport(adminPage);
    const surface = new IsolatedMountPage(adminPage), catalog = new ProviderCatalogPage(adminPage), importer = new ProviderCatalogImportPage(adminPage);
    try {
      await surface.mount(`/x-assets/idc-resource/${version}/mount.js`, "/idc/resource/provider-configuration", "en-US", locales["en-US"]);
      await surface.click("Details"); await catalog.open();
      await expect(catalog.row(product.name)).toBeVisible();
      await catalog.viewPrices(product.name); await expect(catalog.prices).toContainText("100.005");
      await importer.open();
      await expect(importer.dialog).toContainText("Creates an unpublished draft only");
      await expect(importer.dialog).toContainText("Markup percentage");
      await importer.category("Cloud servers"); await importer.markup("10");
      state.failure = true;
      expect((await importer.submitAndWait()).status()).toBe(503);
      await expect(importer.dialog).toContainText("Import did not complete");
      const first = state.requests[0]!;
      await importer.close();
      await importer.open();
      await expect(importer.input("provider-import-markup")).toHaveValue("");
      await importer.category("Cloud servers"); await importer.markup("10");
      state.failure = false;
      expect((await importer.submitAndWait()).status()).toBe(200);
      await expect(importer.result).toContainText("The product draft was created and remains unpublished");
      expect(state.requests).toHaveLength(2);
      expect(state.requests[1]!.requestKey).not.toBe(first.requestKey);
      expect({ ...state.requests[1], requestKey: first.requestKey }).toEqual(first);
      expect(state.categoryReads).toBe(2);
      await expect(importer.result).toContainText("Configuration to complete");
      await expect(importer.result).toContainText("CPU core count");
      await expect(importer.result).toContainText("Memory capacity");
      await expect(importer.result).toContainText("Complete pricing in the draft");
      await expect(importer.result).not.toContainText("private_");
      await expect(importer.result).not.toContainText("处理器核心数");
      await expect(importer.result).not.toContainText("plugin.idc-resource");
      expect(state.forbidden).toEqual([]);
      await surface.screenshot("provider-import-draft-result-en");
    } finally { await surface.dispose(); }
  });
});
