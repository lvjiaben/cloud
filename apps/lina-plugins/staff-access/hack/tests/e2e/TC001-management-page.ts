import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ensureSourcePluginEnabled } from "../../../../../../hack/tests/fixtures/plugin";

import { StaffAccessPage } from "../pages/StaffAccessPage";

test.describe("TC-1 员工权限页面", () => {
  test("TC-1a: 中文页面能读取资格列表且不显示原始接口错误", async ({
    adminPage,
  }) => {
    await ensureSourcePluginEnabled(adminPage, "staff-access");
    const staffAccess = new StaffAccessPage(adminPage);
    await staffAccess.goto();

    await expect(
      staffAccess.pageRoot.getByText("新增资格", { exact: true }),
    ).toBeVisible();
    await expect(staffAccess.total).toHaveText(/^\d+$/u);
    await expect(adminPage.getByTestId("staff-access-error")).toHaveCount(0);
    const navigation = adminPage.getByRole("menu");
    await expect(
      navigation.getByText("员工权限", { exact: true }),
    ).toBeVisible();
    await expect(
      navigation.getByText("Staff Permissions", { exact: true }),
    ).toHaveCount(0);
    await expect(
      adminPage.getByText("插件页面未找到", { exact: true }),
    ).toHaveCount(0);
    await expect(
      adminPage.locator('[data-plugin-frontend-root="staff-access"]'),
    ).toHaveCount(0);
    await expect(
      adminPage.getByTestId("layout-user-dropdown-trigger"),
    ).toBeVisible();
    await expect(
      adminPage.getByText("处理客户工单", { exact: true }),
    ).toBeHidden();
    await staffAccess.pageRoot
      .getByRole("button", { name: "新增资格", exact: true })
      .click();
    const dialog = adminPage.getByRole("dialog", { name: "新增资格" });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText("最近 5 分钟内完成过完整登录");
    await expect(dialog).not.toContainText("idc.support.");
    const dialogBottom = await dialog.evaluate(
      (element) => element.getBoundingClientRect().bottom,
    );
    expect(dialogBottom).toBeLessThanOrEqual(
      await adminPage.evaluate(() => window.innerHeight),
    );
    await dialog.getByRole("button", { name: /取\s*消/u }).click();
    await expect(dialog).toBeHidden();
    await staffAccess.screenshot("staff-access-management-loaded");
  });

  test("TC-1b: 英文和手机宽度下文案完整且页面不横向溢出", async ({
    adminPage,
    mainLayout,
  }) => {
    await ensureSourcePluginEnabled(adminPage, "staff-access");
    await mainLayout.switchLanguage("English");
    await adminPage.setViewportSize({ height: 844, width: 390 });

    const staffAccess = new StaffAccessPage(adminPage);
    await staffAccess.goto();
    await expect(
      staffAccess.pageRoot.getByText("Grant access", { exact: true }),
    ).toBeVisible();
    await expect(staffAccess.pageRoot).toContainText("Staff Permissions");
    await expect(staffAccess.pageRoot).not.toContainText("plugin.staff-access");
    await expect(staffAccess.pageRoot).not.toContainText(
      /员工|资格|业务职责|适用范围/u,
    );
    const overflow = await adminPage.evaluate(
      () =>
        document.documentElement.scrollWidth -
        document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(1);
    await staffAccess.screenshot("staff-access-en-us-mobile");
  });
});
