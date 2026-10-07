import { expect, test } from "../../fixtures/auth";
import { DashboardPage } from "../../pages/DashboardPage";

test.describe("TC002 默认工作台", () => {
  test("TC002a: 工作台展示真实待办、任务和经营数据", async ({ adminPage }) => {
    const dashboardPage = new DashboardPage(adminPage);
    await dashboardPage.gotoWorkspace();

    await expect(
      dashboardPage.workspacePage.getByTestId("dashboard-workspace-error"),
    ).toHaveCount(0);
    await expect(
      dashboardPage.workspacePage.getByTestId("operations-workbench"),
    ).toHaveCount(0);
    await expect(dashboardPage.workspaceProjects).toContainText("今日业务重点");
    await expect(dashboardPage.workspaceQuickNav).toContainText("快捷入口");
    await expect(dashboardPage.workspaceTodos).toContainText("待办事项");
    await expect(dashboardPage.workspaceTrends).toContainText("最近异步任务");
    await expect(dashboardPage.workspacePage).not.toContainText("GoFrame");
    await expect(dashboardPage.workspacePage).not.toContainText("TypeScript");
    await expect(dashboardPage.workspacePage).not.toContainText("今日晴");
  });

  test("TC002b: 工作台快捷入口跳转到真实业务页面", async ({ adminPage }) => {
    const dashboardPage = new DashboardPage(adminPage);
    const quickNavCases: Array<[string, RegExp]> = [
      ["客户管理", /\/idc\/customers/u],
      ["商品管理", /\/idc\/catalog/u],
      ["订单管理", /\/idc\/orders/u],
      ["服务器与服务", /\/idc\/resource\/overview/u],
      ["工单中心", /\/idc\/support/u],
      ["财务管理", /\/idc\/finance/u],
    ];

    for (const [label, expectedUrl] of quickNavCases) {
      await dashboardPage.gotoWorkspace();
      await dashboardPage.workspaceQuickNavItem(label).click();
      await expect(adminPage).toHaveURL(expectedUrl);
    }
  });
});
