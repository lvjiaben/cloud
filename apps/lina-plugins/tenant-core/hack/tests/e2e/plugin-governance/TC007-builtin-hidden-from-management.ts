import { PluginPage } from "@host-tests/pages/PluginPage";

import { test } from "../../support/tenant-core";

const tenantCorePluginID = "tenant-core";

test.describe("TC-7 tenant-core 内置治理", () => {
  test("TC-7a: tenant-core 不出现在插件管理列表", async ({ adminPage }) => {
    const pluginPage = new PluginPage(adminPage);
    await pluginPage.gotoManage();
    await pluginPage.searchAndExpectPluginMissing(tenantCorePluginID);
  });
});
