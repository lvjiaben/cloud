import {
  ensureMultiTenantPluginEnabled,
  expect,
  test,
} from "../../support/tenant-core";
import {
  createAdminApiContext,
  expectBusinessError,
  getPlugin,
} from "@host-tests/support/api/job";
import { PluginPage } from "@host-tests/pages/PluginPage";

test.describe("TC-4 内建多租户插件生命周期保护", () => {
  test.use({ multiTenantMode: "tenant-core-enabled" });

  test("TC-4a: 内建插件不出现在管理列表且强制卸载也被拒绝", async ({
    adminPage,
    multiTenantMode,
  }) => {
    expect(multiTenantMode).toBe("tenant-core-enabled");

    const api = await createAdminApiContext();
    const pluginId = "tenant-core";
    try {
      await ensureMultiTenantPluginEnabled(api);

      const pluginPage = new PluginPage(adminPage);
      await pluginPage.gotoManage();
      await pluginPage.searchAndExpectPluginMissing(pluginId);

      const payload = await expectBusinessError(
        await api.delete(`plugins/${pluginId}?force=true`, {
          data: { force: true, purgeStorageData: 0 },
        }),
      );
      expect(payload.errorCode).toBe("PLUGIN_BUILTIN_MANAGEMENT_ACTION_DENIED");
      const pluginAfterAttempt = await getPlugin(api, pluginId);
      expect(pluginAfterAttempt.installed).toBe(1);
      expect(pluginAfterAttempt.enabled).toBe(1);
    } finally {
      await api.dispose();
    }
  });
});
