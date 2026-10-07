import { test, expect } from "@host-tests/fixtures/auth";
import {
  createAdminApiContext,
  findPlugin,
  listPlugins,
} from "@host-tests/fixtures/plugin";
import { waitForRouteReady } from "@host-tests/support/ui";

const builtinPlugins = ["content-notice", "org-core"] as const;

test.describe("TC-7 内置业务插件启动对账", () => {
  test("TC007a: 通知公告和组织管理始终启用、隐藏且刷新后业务页可用", async ({
    adminPage,
  }) => {
    const adminApi = await createAdminApiContext();
    try {
      const managed = await listPlugins(adminApi);
      for (const pluginID of builtinPlugins) {
        expect(managed.some((item) => item.id === pluginID)).toBe(false);
        expect(await findPlugin(adminApi, pluginID)).toMatchObject({
          distribution: "builtin",
          enabled: 1,
          installed: 1,
          newBusinessState: "accepting",
        });
      }

      await adminPage.goto("/system/dept", { waitUntil: "domcontentloaded" });
      await waitForRouteReady(adminPage, 15_000);
      await expect(
        adminPage.getByText("部门管理", { exact: true }).first(),
      ).toBeVisible();
      await adminPage.reload({ waitUntil: "domcontentloaded" });
      await waitForRouteReady(adminPage, 15_000);
      await expect(
        adminPage.getByText("部门管理", { exact: true }).first(),
      ).toBeVisible();

      await adminPage.goto("/system/notice", { waitUntil: "domcontentloaded" });
      await waitForRouteReady(adminPage, 15_000);
      await expect(
        adminPage.getByTestId("notice-management-page"),
      ).toBeVisible();
    } finally {
      await adminApi.dispose();
    }
  });
});
