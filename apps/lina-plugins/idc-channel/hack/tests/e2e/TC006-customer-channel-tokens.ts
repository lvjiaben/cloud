// Customer platform browser acceptance: real plugin page and host menu, with isolated API responses.
import type { Route } from "@playwright/test";
import { expect, test } from "@host-tests/fixtures/auth";
import { mockCustomerAPIs } from "../../../../../lina-vben/apps/web-customer/tests/support/customer-api-mock";
import { ChannelCustomerTokenPage } from "../pages/ChannelCustomerTokenPage";

const api = "/x/idc-channel/api/v1/account/open-platform";
test.use({
  baseURL: process.env.E2E_CUSTOMER_BASE_URL?.trim() || "http://127.0.0.1:6032",
});
async function fulfill(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify({
      code: status === 200 ? 0 : 403,
      data,
      message: status === 200 ? "success" : "not authorized",
    }),
  });
}
async function surface(route: Route) {
  await fulfill(route, {
    list: [
      {
        id: "idc-channel",
        installed: 1,
        enabled: 1,
        version: "v0.1.2",
        generation: 1,
        statusKey: "sys_plugin.status:idc-channel",
        runtimeState: "normal",
        type: "source",
        customerSurfaces: [
          {
            pluginId: "idc-channel",
            pluginType: "source",
            key: "open-platform",
            version: "v1",
            title: "plugin.idc-channel.workspace.platform.title",
            icon: "lucide:code",
            slot: "customer.products",
            routePath: "/account/plugins/idc-channel/open-platform",
            entry: "customer/channel-tokens.vue",
            status: "active",
            bridgeCapabilities: [],
          },
        ],
      },
    ],
  });
}

test.describe("TC006 customer open platform", () => {
  test("TC-6a: 授权菜单、接入信息、文档、重置及完整分页记录", async ({
    page,
  }) => {
    let version = 42;
    let resetCount = 0;
    await mockCustomerAPIs(page);
    await page.route("**/api/v1/plugins/dynamic**", surface);
    await page.route(`**${api}**`, async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname.endsWith("/secrets")) {
        expect(route.request().postDataJSON()).toEqual({ tokenId: 42 });
        resetCount++;
        version = 43;
        await fulfill(route, {
          appId: "app_customer_fixture",
          secret: "one-time-secret-fixture",
        });
        return;
      }
      if (url.pathname.endsWith("/calls")) {
        const current = Number(url.searchParams.get("page") || 1);
        const size = Number(url.searchParams.get("pageSize") || 10);
        await fulfill(route, {
          list: Array.from(
            { length: Math.min(size, Math.max(0, 23 - (current - 1) * size)) },
            (_, i) => ({
              id: 23 - ((current - 1) * size + i),
              method: "GET",
              operation: "catalog.read",
              succeeded: i % 2 === 0,
              durationMillis: 8,
              createdAt: 1791792000000,
            }),
          ),
          total: 23,
        });
        return;
      }
      await fulfill(route, {
        authorized: true,
        appId: "app_customer_fixture",
        displayName: "Customer API",
        status: "approved",
        tokenId: version,
        canReset: true,
        scopes: ["catalog.read"],
        sourceCidrs: ["127.0.0.0/8"],
        expiresAt: 1798761600000,
      });
    });
    const platform = new ChannelCustomerTokenPage(page);
    await platform.login();
    await platform.goto();
    await expect(
      page.getByRole("menuitem", { name: "开放平台", exact: true }).last(),
    ).toBeVisible();
    await expect(platform.root.getByTestId("platform-app-id")).toHaveText(
      "app_customer_fixture",
    );
    await expect(platform.root).toContainText("来源限制");
    await expect(platform.root).toContainText("127.0.0.0/8");
    await platform.screenshot("open-platform-info");
    await page.getByRole("tab", { name: "接口文档", exact: true }).click();
    await expect(platform.root).toContainText("/channel/orders");
    await platform.screenshot("open-platform-docs");
    await page.getByRole("tab", { name: "调用记录", exact: true }).click();
    await expect(platform.root.locator(".vxe-body--row")).toHaveCount(10);
    await expect(platform.root).toContainText("共 23 条");
    await expect(platform.root).toContainText("失败");
    await platform.screenshot("open-platform-calls");
    await page.getByRole("tab", { name: "接入信息", exact: true }).click();
    await platform.root.getByTestId("platform-reset").click();
    await expect(
      page.getByText("重置密钥后，旧密钥立即失效。", { exact: true }),
    ).toBeVisible();
    await platform.screenshot("open-platform-reset-confirm");
    await page.getByRole("button", { name: /^确\s*认$/ }).click();
    await expect(page.getByTestId("platform-new-secret")).toHaveText(
      "one-time-secret-fixture",
    );
    expect(resetCount).toBe(1);
    await platform.screenshot("open-platform-reset-once");
    await page
      .getByRole("button", { name: "我已保存，关闭", exact: true })
      .click();
    await expect(page.getByTestId("platform-new-secret")).not.toBeVisible();
    await page.reload();
    await expect(platform.root).not.toContainText("one-time-secret-fixture");
  });
  test("TC-6b: 未授权隐藏菜单，停用保留历史且不能重置", async ({ page }) => {
    let authorized = false;
    await mockCustomerAPIs(page);
    await page.route("**/api/v1/plugins/dynamic**", surface);
    await page.route(`**${api}**`, (route) =>
      fulfill(route, {
        authorized,
        appId: authorized ? "app_paused" : "",
        status: "suspended",
        canReset: false,
        tokenId: 1,
        scopes: ["catalog.read"],
        sourceCidrs: [],
      }),
    );
    const platform = new ChannelCustomerTokenPage(page);
    await platform.login();
    await platform.goto();
    await expect(platform.root).toContainText("管理员还未为你开放接口");
    await expect(
      page.getByRole("menuitem", { name: "开放平台", exact: true }),
    ).toHaveCount(0);
    await platform.screenshot("open-platform-unbound");
    authorized = true;
    await page.reload();
    await expect(platform.root.getByTestId("platform-app-id")).toHaveText(
      "app_paused",
    );
    await expect(
      page.getByRole("menuitem", { name: "开放平台", exact: true }).last(),
    ).toBeVisible();
    await expect(platform.root.getByTestId("platform-reset")).toBeDisabled();
    await expect(platform.root).toContainText("已暂停");
    await platform.screenshot("open-platform-paused");
  });
});
