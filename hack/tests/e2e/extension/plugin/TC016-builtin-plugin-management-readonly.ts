import type { Page, Route } from "@playwright/test";

import { test, expect } from "../../../fixtures/auth";
import { createAdminApiContext, listPlugins } from "../../../fixtures/plugin";
import { PluginPage } from "../../../pages/PluginPage";

const builtinPluginID = "linapro-builtin-hidden-e2e";
const managedPluginID = "managed-visible-e2e";

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function managedPluginRow() {
  return {
    abnormalReason: "",
    authorizationRequired: 0,
    authorizationStatus: "not_required",
    autoEnableForNewTenants: false,
    autoEnableManaged: 0,
    description: "Managed plugin visible in plugin management.",
    discoveredVersion: "v0.1.0",
    distribution: "managed",
    effectiveVersion: "v0.1.0",
    enabled: 1,
    hasMockData: 0,
    id: managedPluginID,
    installMode: "global",
    installed: 1,
    installedAt: 1767240000000,
    name: "Managed Visible E2E",
    newBusinessState: "accepting",
    newBusinessStoppedAt: 0,
    runtimeState: "normal",
    scopeNature: "platform_only",
    statusKey: `sys_plugin.status:${managedPluginID}`,
    supportsMultiTenant: false,
    type: "dynamic",
    updatedAt: 1767240000000,
    upgradeAvailable: false,
    version: "v0.1.0",
  };
}

async function mockManagedOnlyList(page: Page) {
  const searches: string[] = [];
  const row = managedPluginRow();

  await page.route("**/api/v1/plugins**", async (route: Route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (
      request.method() === "GET" &&
      /\/api\/v1\/plugins$/u.test(url.pathname)
    ) {
      searches.push(url.search);
      const id = url.searchParams.get("id")?.trim();
      const list = !id || row.id.includes(id) ? [row] : [];
      await route.fulfill({ json: apiEnvelope({ list, total: list.length }) });
      return;
    }
    if (
      request.method() === "GET" &&
      url.pathname.endsWith("/plugins/dynamic")
    ) {
      await route.fulfill({ json: apiEnvelope({ list: [] }) });
      return;
    }
    await route.continue();
  });

  return searches;
}

test.describe("TC-16 内置插件从管理列表隐藏", () => {
  test("TC-16a: 页面只展示托管插件且不再发送兼容参数", async ({
    adminPage,
  }) => {
    const pageErrors: string[] = [];
    adminPage.on("pageerror", (error) => pageErrors.push(error.message));
    const searches = await mockManagedOnlyList(adminPage);

    const pluginPage = new PluginPage(adminPage);
    await pluginPage.gotoManage();
    await expect(pluginPage.pluginRow(managedPluginID)).toBeVisible();
    await expect(pluginPage.pluginRow(builtinPluginID)).toHaveCount(0);
    expect(searches.length).toBeGreaterThan(0);
    expect(searches.every((search) => !search.includes("includeBuiltin"))).toBe(
      true,
    );
    expect(pageErrors).toEqual([]);
  });

  test("TC-16b: 真实后端的管理列表只返回托管插件", async () => {
    const adminApi = await createAdminApiContext();
    try {
      const managedRows = await listPlugins(adminApi);
      expect(
        managedRows.length,
        "插件管理至少应存在一个托管插件",
      ).toBeGreaterThan(0);
      expect(managedRows.every((item) => item.distribution === "managed")).toBe(
        true,
      );
    } finally {
      await adminApi.dispose();
    }
  });
});
