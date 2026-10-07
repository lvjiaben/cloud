import type { APIResponse } from "@host-tests/support/playwright";

import { test, expect } from "@host-tests/fixtures/auth";
import {
  createAdminApiContext,
  findPlugin,
  listPlugins,
} from "@host-tests/fixtures/plugin";
import { waitForRouteReady } from "@host-tests/support/ui";

const pluginID = "content-notice";

async function expectBuiltinActionDenied(response: APIResponse) {
  const payload = (await response.json()) as {
    code?: number;
    errorCode?: string;
    message?: string;
    messageKey?: string;
  };
  expect(response.ok() && payload.code === 0).toBe(false);
  expect(
    `${payload.errorCode ?? ""} ${payload.messageKey ?? ""} ${payload.message ?? ""}`,
  ).toMatch(/builtin|内置/iu);
}

test.describe("TC-1 通知公告内置插件生命周期", () => {
  test("TC001a: 内置插件强制启用、管理列表隐藏且拒绝生命周期操作", async ({
    adminPage,
  }) => {
    const adminApi = await createAdminApiContext();
    try {
      const detail = await findPlugin(adminApi, pluginID);
      expect(detail).toMatchObject({
        distribution: "builtin",
        enabled: 1,
        installed: 1,
        newBusinessState: "accepting",
      });
      expect((await listPlugins(adminApi, { id: pluginID })).length).toBe(0);

      await expectBuiltinActionDenied(
        await adminApi.put(`plugins/${pluginID}/disable`),
      );
      await expectBuiltinActionDenied(
        await adminApi.put(`plugins/${pluginID}/new-business/stop`),
      );
      await expectBuiltinActionDenied(
        await adminApi.delete(`plugins/${pluginID}`),
      );

      expect(await findPlugin(adminApi, pluginID)).toMatchObject({
        enabled: 1,
        installed: 1,
        newBusinessState: "accepting",
      });

      await adminPage.goto("/system/notice", { waitUntil: "domcontentloaded" });
      await waitForRouteReady(adminPage, 15_000);
      await expect(
        adminPage.getByTestId("notice-management-page"),
      ).toBeVisible();
      await expect(
        adminPage.locator('[data-testid="plugin-sandbox-frame"]'),
      ).toHaveCount(0);
    } finally {
      await adminApi.dispose();
    }
  });
});
