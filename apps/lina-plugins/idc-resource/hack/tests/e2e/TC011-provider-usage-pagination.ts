import { readFileSync } from "node:fs";
import type { Page } from "../../../../../../hack/tests/fixtures/auth";
import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ProviderUsagePage } from "../pages/ProviderUsagePage";

/** All business responses are bounded fixtures; no upstream or billing mutation. */
async function installDirectory(page: Page) {
  const state = { failure: 0, requests: [] as URLSearchParams[] };
  const flat = JSON.parse(readFileSync(new URL("../../../manifest/i18n/zh-CN/plugin.json", import.meta.url), "utf8")) as Record<string, string>;
  const messages: Record<string, any> = {};
  for (const [key, value] of Object.entries(flat)) {
    let branch = messages;
    const parts = key.split(".");
    for (const part of parts.slice(0, -1)) branch = branch[part] ??= {};
    branch[parts.at(-1)!] = value;
  }
  await page.route("**/api/v1/i18n/runtime/messages**", route => route.fulfill({ json: { code: 0, data: { messages } } }));
  await page.route("**/api/v1/user/info", async route => {
    const response = await route.fetch(); const payload = await response.json();
    payload.data.permissions = [...(payload.data.permissions ?? []), "idc:admin:resource:configuration:list", "idc:admin:resource:configuration:write"];
    await route.fulfill({ json: payload });
  });
  await page.route("**/api/v1/menus/all", route => route.fulfill({ json: { code: 0, data: { list: [{ name: "IDCOperations", path: "/idc", component: "BasicLayout", meta: { title: "IDC 运营" }, children: [{ name: "IDCProviderConfiguration", path: "/idc/resource/provider-configuration", component: "system/plugin/dynamic-page", meta: { title: "plugin.idc-resource.providerConfiguration.title" } }] }] } } }));
  await page.route(/\/x\/idc-resource\/api\/v1\/admin\/resource\/provider-configurations(?:\?.*)?$/, async route => {
    const params = new URL(route.request().url()).searchParams;
    state.requests.push(params);
    if (state.failure) { await route.fulfill({ status: state.failure, json: { code: state.failure, message: "测试读取失败" } }); return; }
    const channelPage = Number(params.get("channelPage") || 1), channelSize = Number(params.get("channelPageSize") || 20);
    const poolPage = Number(params.get("poolPage") || 1), poolSize = Number(params.get("poolPageSize") || 20);
    const channels = Array.from({ length: 121 }, (_, index) => ({
      channelNo: `CHN_usage_${index + 1}`, displayName: `接口 ${index + 1}`, endpoint: `https://supplier${index + 1}.example.invalid`,
      providerPluginId: "test-connector", status: "disabled", secretFields: [], version: 1,
      interfaceConfiguration: { version: "v1", fields: [] },
      usage: { activeInstances: index + 3, totalInstances: index + 5, configuredProducts: index % 4 },
    }));
    const pools = Array.from({ length: 521 }, (_, index) => ({ poolNo: `POL_usage_${index + 1}`, channelNo: "CHN_usage_121", channelName: "接口 121", displayName: `资源池 ${index + 1}`, status: "disabled", resourceKind: "virtual_server", routingPriority: 10, routingWeight: 100, version: 1 }));
    await route.fulfill({ json: { code: 0, data: { channelTotal: channels.length, poolTotal: pools.length, channels: channels.slice((channelPage - 1) * channelSize, channelPage * channelSize), pools: pools.slice((poolPage - 1) * poolSize, poolPage * poolSize), providerCandidates: [] } } });
  });
  return state;
}

test.describe("TC-11 供应商接口使用统计和真实分页", () => {
  test("TC-11a: 当前使用统计、详情重复打开、独立页码及每页切换", async ({ adminPage }) => {
    const state = await installDirectory(adminPage);
    const view = new ProviderUsagePage(adminPage);
    await view.goto();
    await expect(view.usageRow("接口 1")).toContainText("3 / 5");
    await expect(view.pager).toContainText("121");
    for (let index = 0; index < 2; index++) {
      await view.details("接口 1");
      await expect(view.drawer).toContainText("3 / 5");
      await expect(view.drawer).toContainText("当前生效版本");
      await view.stableScreenshot(`provider-usage-drawer-${index}`);
      await view.closeDetails();
    }
    await view.numberPage(3);
    await expect(view.usageRow("接口 41")).toBeVisible();
    await view.pageSize(50);
    await expect.poll(() => state.requests.at(-1)?.get("channelPageSize")).toBe("50");
    expect(state.requests.at(-1)?.get("channelPage")).toBe("1");
    await expect(view.usageRow("接口 1")).toBeVisible();
    await view.pools();
    await expect(view.pager).toContainText("521");
    await expect(view.usageRow("资源池 1")).toContainText("接口 121");
    await view.numberPage(3);
    await expect(view.usageRow("资源池 41")).toBeVisible();
    expect(state.requests.at(-1)?.get("channelPageSize")).toBe("50");
    await view.stableScreenshot("provider-usage-pool-pagination");
  });
  test("TC-11b: 暂时失败保留已确认页，权限拒绝清空统计和详情", async ({ adminPage }) => {
    const state = await installDirectory(adminPage);
    const view = new ProviderUsagePage(adminPage);
    await view.goto();
    await expect(view.usageRow("接口 1")).toBeVisible();
    state.failure = 503;
    await view.nextPage();
    await expect.poll(() => view.activePageText()).toBe("1");
    await expect(view.usageRow("接口 1")).toContainText("3 / 5");
    await view.details("接口 1");
    state.failure = 403;
    await view.refreshWhileDetailsOpen();
    await expect(view.drawer).toBeHidden();
    await expect(view.usageRow("接口 1")).toHaveCount(0);
    await expect(view.pager).toContainText("0");
    state.failure = 0;
    await view.refresh();
    await expect(view.usageRow("接口 1")).toContainText("3 / 5");
    await view.stableScreenshot("provider-usage-read-denial-recovery");
  });
});
