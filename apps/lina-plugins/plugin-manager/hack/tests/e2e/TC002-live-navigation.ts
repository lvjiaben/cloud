// Exercise the deployed menu and catalog, without injected routes or API mocks.
import { test, expect } from "../support/test";
import { LoginPage } from "../../../../../../hack/tests/pages/LoginPage";
import { config } from "../../../../../../hack/tests/fixtures/config";
import { PluginManagerPage } from "../pages/PluginManagerPage";

test.describe("TC-2 真实插件导航", () => {
  test("TC-2b: 只读验证指定插件的真实卸载结果", async ({ page }) => {
    const target = process.env.E2E_EXPECT_UNINSTALLED_PLUGIN;
    test.skip(!target, "需要显式指定本次操作后应为未安装状态的插件");
    const login = new LoginPage(page);
    await login.goto();
    await login.loginAndWaitForRedirect(config.adminUser, config.adminPass);
    const manager = new PluginManagerPage(page);
    const response = page.waitForResponse((result) => {
      const url = new URL(result.url());
      return (
        url.pathname === "/x/plugin-manager/api/v1/plugins" &&
        url.searchParams.get("category") === "payment"
      );
    });
    await manager.openFromSettings();
    const body = await (await response).json();
    expect(body.code).toBe(0);
    expect(
      body.data.list.find((row: { id: string }) => row.id === target)
        ?.installed,
    ).toBe(0);
    await expect(manager.identity(target!)).toBeVisible();
    await expect(
      manager.root.getByTestId(`plugin-status-${target}`),
    ).toHaveCount(0);
    await expect(
      manager.root.getByTestId(`plugin-uninstall-${target}`),
    ).toHaveCount(0);
    await expect(
      manager.root.getByTestId(`plugin-install-${target}`),
    ).toBeVisible();
    await manager.screenshot("live-payment-uninstalled");
    await page.reload();
    await expect(
      manager.root.getByTestId(`plugin-install-${target}`),
    ).toBeVisible();
  });
  test("TC-2c: 显式指定本地插件后真实卸载重装，业务自动开放", async ({
    page,
  }) => {
    const id = process.env.E2E_MANAGED_LIFECYCLE_PLUGIN;
    test.skip(!id, "仅在本地明确授权生命周期验收时运行");
    expect(["127.0.0.1", "localhost"]).toContain(
      new URL(config.baseURL).hostname,
    );
    const login = new LoginPage(page);
    await login.goto();
    await login.loginAndWaitForRedirect(config.adminUser, config.adminPass);
    const manager = new PluginManagerPage(page);
    await manager.openFromSettings();
    await expect(
      manager.root.getByTestId(`plugin-business-${id}`),
    ).toContainText("接收中");
    const removal = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === `/api/v1/plugins/${id}` &&
        response.request().method() === "DELETE",
    );
    await manager.action("uninstall", id!);
    await expect(manager.modal).toBeHidden();
    await expect(manager.popconfirm).toContainText("确认卸载");
    await expect(manager.popconfirm).not.toContainText("授权");
    await manager.screenshot("live-simple-uninstall-confirm");
    await manager.submit();
    expect((await (await removal).json()).code).toBe(0);
    await expect(
      manager.root.getByTestId(`plugin-install-${id}`),
    ).toBeVisible();
    const installation = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === `/api/v1/plugins/${id}/install` &&
        response.request().method() === "POST",
    );
    await manager.action("install", id!);
    await expect(manager.modal).toBeHidden();
    await manager.submit();
    expect((await (await installation).json()).code).toBe(0);
    await expect(
      manager.root.getByTestId(`plugin-status-${id}`),
    ).toHaveAttribute("aria-checked", "true");
    await expect(
      manager.root.getByTestId(`plugin-business-${id}`),
    ).toContainText("接收中");
    await page.reload();
    await expect(
      manager.root.getByTestId(`plugin-business-${id}`),
    ).toContainText("接收中");
    await manager.screenshot("live-reinstalled-business-open");
  });
  test("TC-2a: 登录后从系统设置进入并读取真实分类和服务器插件", async ({
    page,
  }) => {
    const login = new LoginPage(page);
    await login.goto();
    await login.loginAndWaitForRedirect(config.adminUser, config.adminPass);
    const manager = new PluginManagerPage(page);
    const categoriesResponse = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname ===
        "/x/plugin-manager/api/v1/categories",
    );
    await manager.openFromSettings();
    const categories = await (await categoriesResponse).json();
    expect(categories.code).toBe(0);
    expect(categories.data.list.length).toBeGreaterThanOrEqual(7);
    for (const category of categories.data.list) {
      await expect(
        manager.root.getByRole("tab", { name: category.name, exact: true }),
      ).toBeVisible();
      await expect(
        manager.root.getByTestId(`plugin-category-icon-${category.id}`),
      ).toBeVisible();
    }
    // This also fails if the old extension parent is restored: its child
    // routes are hidden by the deployed administration menu organizer.
    await expect(manager.navigationEntry).toBeVisible();
    await expect(page).toHaveURL(/\/extension\/plugin-manager/);
    const serversResponse = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return (
        url.pathname === "/x/plugin-manager/api/v1/plugins" &&
        url.searchParams.get("category") === "server"
      );
    });
    await manager.tab("服务器");
    const servers = await (await serversResponse).json();
    expect(servers.code).toBe(0);
    expect(servers.data.total).toBeGreaterThan(0);
    for (const row of servers.data.list) {
      await expect(manager.identity(row.id)).toContainText(row.name);
    }
    await expect(manager.root).toContainText(`共 ${servers.data.total} 条记录`);
    await manager.screenshot("live-navigation");
    await manager.openGuide();
    expect(
      await manager.guide.evaluate(
        (element) => element.getBoundingClientRect().width,
      ),
    ).toBeGreaterThan(800);
    expect(
      await manager.guide.evaluate(
        (element) => element.getBoundingClientRect().width,
      ),
    ).toBeLessThanOrEqual(920);
    await expect(manager.guide.getByRole("button")).toHaveCount(14);
    await manager.expandGuide("3. 编写 plugin.yaml");
    await expect(manager.guide.locator("pre")).toContainText("owner: idc-core");
    await expect(manager.guide).not.toContainText("pages.system.plugin.guide.");
    await manager.screenshot("live-developer-guide");
    await manager.closeGuide();
    await page.reload();
    await expect(manager.root).toBeVisible();
    await expect(manager.navigationEntry).toBeVisible();
  });
});
