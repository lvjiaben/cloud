import type { Locator, Page } from "@playwright/test";

import { expect } from "@playwright/test";

import { workspacePath } from "../fixtures/config";
import { waitForRouteReady } from "../support/ui";
import { MainLayout } from "./MainLayout";

export class DashboardPage {
  constructor(private page: Page) {}

  get analyticsPage(): Locator {
    return this.page.getByTestId("dashboard-analytics-page");
  }

  get analyticsChartInstances(): Locator {
    return this.analyticsPage.locator("[_echarts_instance_]:visible");
  }

  get workspacePage(): Locator {
    return this.page.getByTestId("dashboard-workspace-page");
  }

  get workspaceDescription(): Locator {
    return this.page.getByTestId("dashboard-workspace-description");
  }

  get workspaceProjects(): Locator {
    return this.page.getByTestId("dashboard-workspace-projects");
  }

  get workspaceQuickNav(): Locator {
    return this.page.getByTestId("dashboard-workspace-quick-nav");
  }

  get workspaceTodos(): Locator {
    return this.page.getByTestId("dashboard-workspace-todos");
  }

  get workspaceTrends(): Locator {
    return this.page.getByTestId("dashboard-workspace-trends");
  }

  analyticsMetric(title: string): Locator {
    return this.page
      .getByTestId("dashboard-analytics-overview")
      .getByText(title, { exact: true });
  }

  analyticsTab(label: string): Locator {
    return this.page
      .getByTestId("dashboard-analytics-tabs")
      .getByText(label, { exact: true });
  }

  analyticsCardTitle(title: string): Locator {
    return this.page.getByRole("heading", { name: title }).first();
  }

  workspaceQuickNavItem(title: string): Locator {
    return this.workspaceQuickNav.getByText(title, { exact: true }).first();
  }

  async gotoAnalytics() {
    await this.page.goto(workspacePath("/dashboard/analytics"));
    await waitForRouteReady(this.page, 30_000);

    // A platform administrator first lands on tenant management when
    // multi-tenancy is enabled. Continue through the real sidebar flow so
    // this page object validates that the dashboard remains reachable rather
    // than assuming that a cold deep link bypasses the platform landing rule.
    if (!(await this.analyticsPage.isVisible().catch(() => false))) {
      const mainLayout = new MainLayout(this.page);
      await mainLayout.expandSidebarGroup(/Dashboard|工作台/u);
      const analyticsMenu = mainLayout.sidebarMenuItem(/Analytics|分析页/u);
      await expect(analyticsMenu).toBeVisible();
      await analyticsMenu.click();
      await expect(this.page).toHaveURL(/\/dashboard\/analytics(?:[/?#]|$)/u);
    }

    await this.analyticsPage.waitFor({ state: "visible" });
  }

  async waitForAnalyticsChartInstances(minimumCount = 4) {
    await expect
      .poll(async () => this.analyticsChartInstances.count())
      .toBeGreaterThanOrEqual(minimumCount);
    await expect
      .poll(async () => (await this.getAnalyticsChartInstanceIds()).length)
      .toBeGreaterThanOrEqual(minimumCount);
  }

  async getAnalyticsChartInstanceIds() {
    return this.analyticsChartInstances.evaluateAll((nodes) =>
      nodes
        .map((node) => node.getAttribute("_echarts_instance_"))
        .filter((id): id is string => Boolean(id)),
    );
  }

  async waitForAnalyticsChartsPainted(minimumCount = 4) {
    await expect
      .poll(async () => {
        const paintInfo = await this.getAnalyticsChartCanvasPaintInfo();
        return (
          paintInfo.length >= minimumCount &&
          paintInfo.every(
            ({ canvasCount, paintedCanvasCount }) =>
              canvasCount > 0 && paintedCanvasCount > 0,
          )
        );
      })
      .toBeTruthy();

    return this.getAnalyticsChartCanvasPaintInfo();
  }

  async getAnalyticsChartCanvasPaintInfo() {
    return this.analyticsChartInstances.evaluateAll((nodes) =>
      nodes.map((node) => {
        const canvases = Array.from(node.querySelectorAll("canvas"));
        const paintedCanvasCount = canvases.filter((canvas) => {
          if (canvas.width === 0 || canvas.height === 0) {
            return false;
          }

          const context = canvas.getContext("2d", {
            willReadFrequently: true,
          });
          if (!context) {
            return false;
          }

          const pixels = context.getImageData(
            0,
            0,
            canvas.width,
            canvas.height,
          ).data;
          const firstRed = pixels[0];
          const firstGreen = pixels[1];
          const firstBlue = pixels[2];
          const firstAlpha = pixels[3];

          for (let index = 4; index < pixels.length; index += 4) {
            if (
              pixels[index] !== firstRed ||
              pixels[index + 1] !== firstGreen ||
              pixels[index + 2] !== firstBlue ||
              pixels[index + 3] !== firstAlpha
            ) {
              return true;
            }
          }

          return false;
        }).length;

        return {
          canvasCount: canvases.length,
          instanceId: node.getAttribute("_echarts_instance_") ?? "",
          paintedCanvasCount,
        };
      }),
    );
  }

  async captureAnalyticsScreenshot(screenshotPath: string) {
    await this.analyticsPage.waitFor({ state: "visible" });
    await expect(this.page.getByText(/加载菜单中/u)).toBeHidden();
    await expect(
      this.analyticsPage.getByTestId("dashboard-analytics-overview"),
    ).toBeVisible();
    await expect(
      this.analyticsPage.getByTestId("dashboard-analytics-error"),
    ).toHaveCount(0);
    await expect(
      this.analyticsPage.getByTestId("dashboard-analytics-loading"),
    ).toHaveCount(0);
    await this.waitForAnalyticsChartInstances(4);
    await this.page.screenshot({
      animations: "disabled",
      fullPage: true,
      path: screenshotPath,
    });
  }

  async gotoWorkspace() {
    await this.page.goto(workspacePath("/dashboard/workspace"));
    await this.page.waitForLoadState("networkidle");
    await this.workspacePage.waitFor({ state: "visible" });
  }

  async clickWorkspaceQuickNav(title: string) {
    await this.workspaceQuickNavItem(title).click();
    await this.page.waitForLoadState("networkidle");
  }
}
