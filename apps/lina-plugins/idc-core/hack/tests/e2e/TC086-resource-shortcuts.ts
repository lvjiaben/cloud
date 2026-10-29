import { test as base, expect } from "../support/resource-navigation-auth";
import { CustomerWorkspacePage } from "../pages/CustomerWorkspacePage";
import {
  installCustomerWorkspaceMock,
  installProductManagementLocales,
  workspaceCustomerNo,
} from "../support/customer-workspace-mock";
const test = base.extend({
  adminPage: async ({ authenticatedPage }, use) => {
    await use(authenticatedPage);
  },
});

test.describe("TC-86 资源菜单快捷导航", () => {
  test("TC-86a: 服务器快捷入口进入客户管理，选择客户直接打开产品，不建立全局目录", async ({
    adminPage,
  }) => {
    await installCustomerWorkspaceMock(adminPage);
    await installProductManagementLocales(adminPage);
    const requested: string[] = [];
    adminPage.on("request", (request) => {
      const path = new URL(request.url()).pathname;
      if (path.startsWith("/x/")) requested.push(path);
    });
    const view = new CustomerWorkspacePage(adminPage);
    await view.openResourceShortcut();
    await expect(adminPage).toHaveURL(
      /idc\/customers.*resourceShortcut=products/,
    );
    await expect(
      adminPage.getByTestId("resource-customer-shortcut-hint"),
    ).toHaveText("选择客户，打开产品管理服务器、证书及续费。");
    await view.openOverview(workspaceCustomerNo);
    await expect(
      view.overviewDrawer.getByRole("tab", { name: "产品", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(view.products).toContainText("云服务器");
    expect(
      requested.some((path) =>
        path.includes(`/customers/${workspaceCustomerNo}/products`),
      ),
    ).toBe(true);
    expect(
      requested.some((path) =>
        /\/admin\/servers$|\/admin\/server-services$/.test(path),
      ),
    ).toBe(false);
    await view.screenshot("resource-shortcut-customer-products");
  });
  test("TC-86b: 普通客户入口仍默认概览，无资源提示", async ({ adminPage }) => {
    await installCustomerWorkspaceMock(adminPage);
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await expect(
      adminPage.getByTestId("resource-customer-shortcut-hint"),
    ).toHaveCount(0);
    await view.openOverview(workspaceCustomerNo);
    await expect(
      view.overviewDrawer.getByRole("tab", { name: "产品", exact: true }),
    ).toHaveAttribute("aria-selected", "false");
    await expect(view.products).toHaveCount(0);
  });
  test("TC-86c: 实际菜单有两个快捷入口和续费待办，点击后进入原拥有方页面", async ({
    adminPage,
  }) => {
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await view.openResourcesMenu();
    const menu = adminPage.getByRole("menu");
    for (const name of ["服务器管理", "供应商运维", "附属资源续费待办"])
      await expect(menu.getByText(name, { exact: true })).toBeVisible();
    await view.screenshot("resource-shortcuts-live-menu");
    await view.clickResourceShortcut("服务器管理");
    await expect(adminPage).toHaveURL(
      /idc\/customers.*resourceShortcut=products/,
    );
    await expect(
      adminPage.getByTestId("resource-customer-shortcut-hint"),
    ).toBeVisible();
    await view.clickResourceShortcut("供应商运维");
    await expect(adminPage).toHaveURL(
      /idc\/resource\/provider-configuration.*resourceTab=operations/,
    );
    await expect(
      adminPage.getByRole("tab", { name: "供应商运维", exact: true }),
    ).toHaveAttribute("aria-selected", "true");
    await expect(
      adminPage.getByTestId("provider-operations-page"),
    ).toBeVisible();
    await view.screenshot("resource-shortcut-live-provider-operations");
    await adminPage.reload();
    await expect(
      adminPage.getByTestId("provider-operations-page"),
    ).toBeVisible();
  });
});
