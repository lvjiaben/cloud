import { expect, test } from "../../fixtures/auth";
import { DashboardPage } from "../../pages/DashboardPage";

test.describe("TC001 默认分析页", () => {
  test("TC001a: 分析页展示真实运营聚合与业务入口", async ({ adminPage }) => {
    const dashboardPage = new DashboardPage(adminPage);
    await dashboardPage.gotoAnalytics();

    await expect(
      dashboardPage.analyticsPage.getByTestId("dashboard-analytics-error"),
    ).toHaveCount(0);
    await expect(
      dashboardPage.analyticsPage.getByTestId("operations-workbench"),
    ).toHaveCount(0);
    await expect(dashboardPage.analyticsMetric("待处理订单")).toBeVisible();
    await expect(dashboardPage.analyticsMetric("待开通服务")).toBeVisible();
    await expect(dashboardPage.analyticsMetric("待处理工单")).toBeVisible();
    await expect(dashboardPage.analyticsMetric("任务告警")).toBeVisible();
    await expect(dashboardPage.analyticsTab("原币种经营金额")).toBeVisible();
    await dashboardPage.waitForAnalyticsChartInstances(4);
  });

  test("TC001b: 零数据时使用真实空态而不是演示数字", async ({ adminPage }) => {
    const dashboardPage = new DashboardPage(adminPage);
    await dashboardPage.gotoAnalytics();

    await expect(dashboardPage.analyticsPage).not.toContainText("120,000");
    await expect(dashboardPage.analyticsPage).not.toContainText("500,000");
    await expect(
      dashboardPage.analyticsPage.getByTestId("dashboard-analytics-overview"),
    ).toBeVisible();
    await expect(
      dashboardPage.analyticsPage.getByTestId("dashboard-analytics-tabs"),
    ).toBeVisible();
  });
});
