import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

test.describe("TC018 系统配置真实页面体验", () => {
  test("TC018-1: 系统配置按业务分类展示唯一入口", async ({ adminPage }) => {
    await adminPage.goto("/admin/system/configuration-center", {
      waitUntil: "domcontentloaded",
    });

    const root = adminPage.getByTestId("system-configuration-center");
    await expect(root).toBeVisible();
    for (const tab of [
      "业务与品牌",
      "自动化接口",
      "认证与合同",
      "短信与通知",
      "支付方式",
      "数据迁移",
    ]) {
      await expect(root.getByRole("tab", { name: tab })).toBeVisible();
    }
    await expect(root).toContainText("官网与用户中心品牌");
    await root.getByRole("tab", { name: "支付方式" }).click();
    await expect(root).toContainText("支付渠道");
    await root.getByRole("tab", { name: "数据迁移" }).click();
    await expect(root).toContainText("迁移检查");
  });

  test("TC018-2: 工单与官网品牌配置使用紧凑方角页面", async ({ adminPage }) => {
    await adminPage.goto("/admin/idc/support/settings", {
      waitUntil: "domcontentloaded",
    });
    const support = adminPage.getByTestId("support-settings-page");
    await expect(support).toBeVisible();
    await expect(support.getByRole("alert")).toHaveCount(0);
    const supportStyle = await support
      .locator(".settings-tabs")
      .evaluate((node) => {
        const style = getComputedStyle(node);
        return { borderRadius: style.borderRadius, boxShadow: style.boxShadow };
      });
    expect(supportStyle).toEqual({ borderRadius: "0px", boxShadow: "none" });

    await adminPage.goto("/admin/idc/portal-brand", {
      waitUntil: "domcontentloaded",
    });
    const portal = adminPage.locator(
      '[data-plugin-frontend-root="idc-portal"]',
    );
    await expect(portal).toBeVisible();
    await expect(
      portal.getByRole("heading", { name: "官网品牌设置" }),
    ).toBeVisible();
    await expect(portal.locator(".portal-brand-error")).toHaveCount(0);
    const portalStyle = await portal.locator("section").evaluate((node) => {
      const style = getComputedStyle(node);
      const inputStyle = getComputedStyle(node.querySelector("input")!);
      const headingStyle = getComputedStyle(node.querySelector("h1")!);
      return {
        borderRadius: inputStyle.borderRadius,
        fontSize: Number.parseFloat(headingStyle.fontSize),
        maxWidth: style.maxWidth,
      };
    });
    expect(portalStyle.borderRadius).toBe("0px");
    expect(portalStyle.fontSize).toBeLessThanOrEqual(22);
    expect(portalStyle.maxWidth).toBe("none");
  });

  test("TC018-3: 四类动态支付插件都能进入统一配置表单", async ({
    adminPage,
  }) => {
    const pageErrors: string[] = [];
    adminPage.on("pageerror", (error) => pageErrors.push(error.message));
    await adminPage.goto("/admin/idc/payment-channels", {
      waitUntil: "domcontentloaded",
    });
    const add = adminPage.getByRole("button", { name: "新增支付方式" });
    await expect(add).toBeEnabled({ timeout: 15_000 });
    await add.click();
    await adminPage.waitForTimeout(250);
    expect(pageErrors, `支付配置页脚本错误: ${pageErrors.join(" | ")}`).toEqual(
      [],
    );

    const dialog = adminPage.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await dialog.locator(".ant-select").first().click();
    const dropdown = adminPage.locator(".ant-select-dropdown:visible");
    for (const provider of ["支付宝支付", "微信支付", "易支付", "银联支付"]) {
      await expect(dropdown.getByText(provider, { exact: true })).toBeVisible();
    }
  });

  test("TC018-4: 迁移检查只从系统配置进入且真实插件页可达", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/system/configuration-center", {
      waitUntil: "domcontentloaded",
    });
    const root = adminPage.getByTestId("system-configuration-center");
    await root.getByRole("tab", { name: "数据迁移" }).click();
    const entry = root.locator(".configuration-item", {
      hasText: "迁移检查",
    });
    await expect(entry).toBeVisible();
    await entry.getByRole("button", { name: "进入配置" }).click();
    await expect(adminPage).toHaveURL(/idc-zjmfmigrate\/migration-checks/u);
    await expect(
      adminPage
        .frameLocator('[data-testid="plugin-sandbox-frame"]')
        .getByTestId("migration-page"),
    ).toBeVisible({ timeout: 15_000 });
  });

  test("TC018-5: 用户中心外链菜单可独立配置且表格高度受控", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/idc/customer-external-menus", {
      waitUntil: "domcontentloaded",
    });

    await expect(
      adminPage.getByText("用户中心菜单", { exact: true }).first(),
    ).toBeVisible({ timeout: 15_000 });
    await expect(adminPage.locator("body")).not.toContainText("插件页面未找到");
    await expect(adminPage.locator("body")).not.toContainText(
      "插件页面请求不可用",
    );

    const grid = adminPage.locator(".external-menu-card .vxe-grid");
    await expect(grid).toBeVisible();
    const gridHeight = await grid.evaluate((node) =>
      Math.round(node.getBoundingClientRect().height),
    );
    expect(gridHeight).toBeGreaterThan(240);
    expect(gridHeight).toBeLessThan(adminPage.viewportSize()!.height);

    await adminPage.getByRole("button", { name: "新增菜单" }).click();
    const dialog = adminPage.getByRole("dialog");
    await expect(dialog).toBeVisible();
    for (const label of [
      "中文名称",
      "英文名称",
      "菜单图标",
      "跳转地址",
      "排序",
    ]) {
      await expect(dialog).toContainText(label);
    }
  });
});
