import { test, expect } from "@host-tests/fixtures/auth";
import {
  createAdminApiContext,
  findPlugin,
  listPlugins,
} from "@host-tests/fixtures/plugin";
import { PluginPage } from "@host-tests/pages/PluginPage";
import { queryPgScalar } from "@host-tests/support/postgres";

const pluginID = "content-notice";
const mockNoticeTitle = "系统升级通知";

test.describe("TC-4 内置通知公告示例数据", () => {
  test("TC-4a: 内置插件保留示例数据能力但不暴露安装入口", async ({
    adminPage,
  }) => {
    const adminApi = await createAdminApiContext();
    try {
      const detail = await findPlugin(adminApi, pluginID);
      expect(detail).toMatchObject({
        distribution: "builtin",
        enabled: 1,
        hasMockData: 1,
        installed: 1,
      });
      expect(await listPlugins(adminApi, { id: pluginID })).toEqual([]);
      expect(
        queryPgScalar(
          "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='plugin_content_notice';",
        ),
      ).toBe("1");
      expect(
        Number(queryPgScalar("SELECT COUNT(*) FROM plugin_content_notice;")),
      ).toBeGreaterThan(0);

      const pluginPage = new PluginPage(adminPage);
      await pluginPage.gotoManage();
      await pluginPage.filterByPluginId(pluginID);
      await expect(pluginPage.pluginRow(pluginID)).toHaveCount(0);

      await adminPage.goto("/system/notice", {
        waitUntil: "domcontentloaded",
      });
      await expect(
        adminPage.getByText(mockNoticeTitle, { exact: true }).first(),
      ).toBeVisible();
    } finally {
      await adminApi.dispose();
    }
  });
});
