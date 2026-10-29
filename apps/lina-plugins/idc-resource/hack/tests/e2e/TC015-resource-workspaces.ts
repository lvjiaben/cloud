import { expect, test as baseTest } from "../../../../idc-core/hack/tests/support/resource-navigation-auth";
import { ResourceWorkspacePage } from "../pages/ResourceWorkspacePage";
import { ProviderTaskPage } from "../pages/ProviderTaskPage";
import { installResourceOverviewMock } from "../support/resource-overview-mock";
import { installProviderHealthMock } from "../support/provider-health-mock";
import { installProviderTaskMock, providerTaskFixture } from "../support/provider-task-mock";
import { installProviderCapacityMock } from "../support/provider-capacity-mock";
import { installResourceWorkspaceMenus } from "../support/resource-workspace-mock";

const test = baseTest.extend({ adminPage: async ({ authenticatedPage }, use) => { await use(authenticatedPage); } });

test.describe("TC-15 内嵌服务器与供应商运维", () => {
  test("TC-15a: 独立路由退役，供应商配置内按需加载四项运维工具", async ({ adminPage }) => {
    const overview = await installResourceOverviewMock(adminPage);
    const health = await installProviderHealthMock(adminPage);
    const tasks = await installProviderTaskMock(adminPage);
    const capacity = await installProviderCapacityMock(adminPage);
    const menus = await installResourceWorkspaceMenus(adminPage, [
      "idc:admin:resource:overview:list", "idc:admin:resource:health:list",
      "idc:admin:resource:task:list", "idc:admin:resource:provider-capacity:list",
    ]);
    expect(menus).toHaveLength(2);
    expect(menus.some(menu => menu.path === "/idc/resource/provider-configuration")).toBe(true);
    const workspace = new ResourceWorkspacePage(adminPage);
    await workspace.open("providers");
    await expect(adminPage.getByTestId("resource-overview-page")).toContainText("12850.00 CNY");
    expect(overview.requests).toBe(1);
    expect(health.listRequests).toHaveLength(0);
    expect(tasks.listRequests).toHaveLength(0);
    expect(capacity.listRequests).toHaveLength(0);
    await expect(adminPage.getByRole("menu").getByText("服务器与资源", { exact: true })).toBeVisible();
    if (!await adminPage.getByRole("menu").getByText("供应商运维", { exact: true }).isVisible())
      await adminPage.getByRole("menu").getByText("服务器与资源", { exact: true }).click();
    await expect(adminPage.getByRole("menu").getByText("供应商运维", { exact: true })).toBeVisible();
    expect((menus[0]?.meta as { hideInMenu?: boolean }).hideInMenu).toBe(false);
    await expect(adminPage.getByRole("menuitem", { name: /SSL|魔方云|资源运营总览|上游任务核对/ })).toHaveCount(0);
    await workspace.screenshot("resource-workspace-overview");
    await workspace.selectTab("连接状态");
    await expect(adminPage.getByTestId("provider-health-page")).toContainText("已暂停新操作");
    expect(health.listRequests).toHaveLength(1);
    await workspace.selectTab("操作记录");
    await expect(new ProviderTaskPage(adminPage).row(providerTaskFixture.taskNo)).toContainText("结果未知");
    expect(tasks.listRequests).toHaveLength(1);
    await workspace.selectTab("资源更新");
    await expect(adminPage.getByTestId("provider-capacity-operations-page")).toContainText("428");
    expect(capacity.listRequests).toHaveLength(1);
    await workspace.screenshot("resource-workspace-capacity");
  });

  test("TC-15b: 仅连接查看权限仍可到达工作台，不读取其他内容或显示写入口", async ({ adminPage }) => {
    const overview = await installResourceOverviewMock(adminPage);
    const health = await installProviderHealthMock(adminPage, { canUpdatePolicy: false });
    await installResourceWorkspaceMenus(adminPage, ["idc:admin:resource:health:list"]);
    const workspace = new ResourceWorkspacePage(adminPage);
    await workspace.open("providers");
    await expect(workspace.tab("连接状态")).toBeVisible();
    await expect(adminPage.getByTestId("provider-operations-page").getByRole("tab")).toHaveCount(1);
    await expect(adminPage.getByRole("tab", { name: "供应商接口", exact: true })).toHaveCount(0);
    await expect(adminPage.getByTestId("provider-health-page")).toContainText("魔方云生产渠道");
    await expect(adminPage.getByTestId("provider-health-policy-channel_mofang_prod")).toHaveCount(0);
    expect(health.listRequests).toHaveLength(1);
    expect(overview.requests).toBe(0);
    await workspace.screenshot("resource-workspace-health-readonly");
  });

  test("TC-15c: 概况失败可刷新恢复，英文窄屏有可读页签", async ({ adminPage, mainLayout }) => {
    const overview = await installResourceOverviewMock(adminPage, { failFirst: true });
    await installResourceWorkspaceMenus(adminPage, ["idc:admin:resource:overview:list"]);
    const workspace = new ResourceWorkspacePage(adminPage);
    await workspace.open("providers");
    await expect(adminPage.getByTestId("resource-overview-page")).toContainText("概况加载失败，请刷新重试。");
    await workspace.screenshot("resource-workspace-overview-failed");
    await workspace.refreshOverview();
    await expect(adminPage.getByTestId("resource-overview-page")).toContainText("12850.00 CNY");
    expect(overview.requests).toBe(2);
    await mainLayout.switchLanguage("English");
    await adminPage.setViewportSize({ width: 768, height: 900 });
    await expect(workspace.tab("Overview")).toBeVisible();
    await expect(adminPage.getByTestId("provider-operations-page")).toContainText("Overview");
    await expect(adminPage.getByTestId("provider-operations-page")).not.toContainText(/plugin\.idc-resource|menu\.plugin:/);
    await workspace.screenshot("resource-workspace-english-narrow");
  });

  // TC-15d/e retired: independent server routes were removed; product renewals are covered by TC-39g.
  test("TC-15f: 操作记录表格撑满、筛选同行和分页完整可达，概况并排且窄屏不溢出", async ({ adminPage, mainLayout }) => {
    await installResourceOverviewMock(adminPage);
    await installProviderTaskMock(adminPage);
    await installResourceWorkspaceMenus(adminPage, ["idc:admin:resource:overview:list", "idc:admin:resource:task:list"]);
    const workspace = new ResourceWorkspacePage(adminPage);
    await workspace.open("providers");
    await mainLayout.switchLanguage("简体中文");
    const cards = adminPage.locator('.overview-directory');
    await expect.poll(async () => {
      const left = await cards.nth(0).boundingBox(); const right = await cards.nth(1).boundingBox();
      return Boolean(left && right && Math.abs(left.y - right.y) < 2 && right.x >= left.x + left.width);
    }).toBe(true);
    await workspace.screenshot("resource-overview-aligned");
    await workspace.selectTab("操作记录");
    await expect(adminPage.getByTestId("provider-task-page")).toContainText("结果未知");
    await workspace.expectGridFits("provider-task-page");
    await workspace.expectSearchAligned();
    await workspace.screenshot("resource-task-aligned");
    await mainLayout.switchLanguage("English");
    await expect(adminPage.getByTestId("provider-task-page")).toContainText("Task status");
    await adminPage.getByText(/加载菜单中|Loading menu/iu).waitFor({ state: "hidden" });
    await expect(workspace.tab("Operations")).toHaveAttribute("aria-selected", "true");
    await adminPage.setViewportSize({ width: 768, height: 1000 });
    await workspace.expectGridFits("provider-task-page");
    await workspace.screenshot("resource-task-english-narrow-aligned");
    await expect(adminPage.getByTestId("provider-task-page")).toBeVisible();
  });

});
