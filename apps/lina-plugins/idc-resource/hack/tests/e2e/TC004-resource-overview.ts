import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ResourceOverviewPage } from "../pages/ResourceOverviewPage";
import { installResourceOverviewMock } from "../support/resource-overview-mock";

test.describe("TC-4 资源运营总览", () => {
  test("TC-4a: 中文页面同时展示资源池、容量时间、任务和供应商财务事实", async ({ adminPage }) => {
    await installResourceOverviewMock(adminPage);
    const overview = new ResourceOverviewPage(adminPage);
    await overview.goto();

    await expect(overview.root.getByText("资源运营总览", { exact: true })).toBeVisible();
    await expect(overview.root).toContainText("华东云服务器池");
    await expect(overview.root).toContainText("12850.00 CNY");
    await expect(overview.root).toContainText("27");
    await expect(overview.root).not.toContainText("provider-secret-ref");
    await expect(overview.root).not.toContainText(/pool_east_cloud|channel_mofang_prod/u);
    await overview.screenshot("resource-overview-zh-cn");
  });

  test("TC-4b: 切换英文后不泄露翻译键或供应商内部引用", async ({ adminPage, mainLayout }) => {
    await installResourceOverviewMock(adminPage);
    const overview = new ResourceOverviewPage(adminPage);
    await overview.goto();
    await mainLayout.switchLanguage("English");

    await expect(overview.root.getByText("Resource Operations Overview", { exact: true })).toBeVisible();
    await expect(overview.root).not.toContainText("plugin.idc-resource");
    await expect(overview.root).not.toContainText(/provider-(secret|expiry)-ref/u);
    await overview.screenshot("resource-overview-en-us");
  });

  test("TC-4c: 首次读取失败后可由员工手工刷新恢复", async ({ adminPage }) => {
    const state = await installResourceOverviewMock(adminPage, { failFirst: true });
    const overview = new ResourceOverviewPage(adminPage);
    await overview.goto();
    await expect(overview.root).toContainText("资源运营总览暂时无法加载");
    await overview.root.getByRole("button", { name: "刷新本地数据" }).click();
    await expect(overview.root).toContainText("华东云服务器池");
    expect(state.requests).toBe(2);
  });
});
