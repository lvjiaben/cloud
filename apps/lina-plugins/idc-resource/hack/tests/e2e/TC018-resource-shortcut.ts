import {
  test as base,
  expect,
} from "../../../../idc-core/hack/tests/support/resource-navigation-auth";
import { ResourceWorkspacePage } from "../pages/ResourceWorkspacePage";
import { installResourceOverviewMock } from "../support/resource-overview-mock";
import { installProviderHealthMock } from "../support/provider-health-mock";
import { installResourceWorkspaceMenus } from "../support/resource-workspace-mock";
const test = base.extend({
  adminPage: async ({ authenticatedPage }, use) => {
    await use(authenticatedPage);
  },
});

test.describe("TC-18 供应商运维快捷入口", () => {
  test("TC-18a: 跳转原配置运维页签，只读取当前运维内容，英文窄屏正常", async ({
    adminPage,
    mainLayout,
  }) => {
    const overview = await installResourceOverviewMock(adminPage);
    await installResourceWorkspaceMenus(adminPage, [
      "idc:admin:resource:overview:list",
      "idc:admin:resource:configuration:list",
    ]);
    const hiddenReads: string[] = [];
    await adminPage.route(
      /\/x\/idc-resource\/api\/v1\/admin\/resource\/(?:provider-configurations|provider-candidates)/,
      (route) => {
        hiddenReads.push(route.request().url());
        return route.abort();
      },
    );
    const view = new ResourceWorkspacePage(adminPage);
    await view.openShortcut();
    await expect(adminPage).toHaveURL(
      /provider-configuration.*resourceTab=operations/,
    );
    await expect(
      adminPage.getByRole("tab", { name: "供应商运维", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(adminPage.getByTestId("resource-overview-page")).toContainText(
      "12850.00 CNY",
    );
    expect(overview.requests).toBe(1);
    expect(hiddenReads).toHaveLength(0);
    await expect(
      adminPage.getByRole("button", { name: /^刷\s*新$/u }),
    ).toHaveCount(1);
    await view.screenshot("provider-shortcut-overview");
    await mainLayout.switchLanguage("English");
    await adminPage.setViewportSize({ width: 768, height: 900 });
    await expect(
      adminPage.getByRole("tab", { name: "Supplier Operations", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(
      adminPage.getByTestId("provider-operations-page"),
    ).toContainText("Overview");
    await view.screenshot("provider-shortcut-english-narrow");
  });
  test("TC-18b: 未授权快捷菜单隐藏，原配置容器仍按连接权限可用", async ({
    adminPage,
  }) => {
    const overview = await installResourceOverviewMock(adminPage);
    const health = await installProviderHealthMock(adminPage, {
      canUpdatePolicy: false,
    });
    await installResourceWorkspaceMenus(adminPage, [
      "idc:admin:resource:health:list",
    ]);
    const view = new ResourceWorkspacePage(adminPage);
    await view.open("providers");
    await expect(
      adminPage.getByRole("menu").getByText("供应商运维", { exact: true }),
    ).toHaveCount(0);
    await expect(adminPage.getByTestId("provider-health-page")).toContainText(
      "魔方云生产渠道",
    );
    await expect(
      adminPage.getByRole("tab", { name: "供应商接口", exact: true }),
    ).toHaveCount(0);
    expect(overview.requests).toBe(0);
    expect(health.listRequests).toHaveLength(1);
    await view.screenshot("provider-shortcut-health-permission");
  });
});
