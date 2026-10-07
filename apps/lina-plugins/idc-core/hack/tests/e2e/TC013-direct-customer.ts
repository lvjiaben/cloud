import { test, expect } from "../../../../../../hack/tests/fixtures/auth";

import { CustomerManagementPage } from "../pages/CustomerManagementPage";
import { installCustomerManagementMock } from "../support/customer-mock";

test.describe("TC-13 后台直接创建可登录客户", () => {
  test("TC-13a: 直接创建不会发送邀请并立即显示为启用客户", async ({
    adminPage,
  }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openDirectCustomer();
    await customers.createDirectCustomer({
      displayName: "直接创建客户",
      email: "direct@example.test",
      password: "Initial-password-123",
      reason: "TC-13 directly activated account",
    });

    await expect(
      adminPage.getByText(
        "客户已直接创建并激活，请让客户登录后立即修改初始密码。",
        { exact: true },
      ),
    ).toBeVisible();
    expect(state.directRequests).toHaveLength(1);
    expect(state.activationRequests).toHaveLength(0);
    expect(state.directRequests[0]).toMatchObject({
      email: "direct@example.test",
      displayName: "直接创建客户",
    });
    expect(state.directRequests[0]).toHaveProperty("initialPassword");
    await expect(
      customers.pageRoot.getByText("直接创建客户", { exact: true }),
    ).toBeVisible();
    await customers.screenshot("customer-direct-active");
  });

  test("TC-13b: 必填星号与标签保持在同一行", async ({ adminPage }) => {
    await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openDirectCustomer();

    const label = adminPage
      .getByTestId("customer-direct-name")
      .locator(
        "xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' form-is-required ')][1]//label",
      );
    const labelStyle = await label.evaluate((element) => {
      const style = getComputedStyle(element);
      return { display: style.display, flexDirection: style.flexDirection };
    });
    expect(["flex", "inline-flex"]).toContain(labelStyle.display);
    expect(labelStyle.flexDirection).toBe("row");
    await customers.screenshot("customer-direct-form-alignment");
  });

  test("TC-13c: 缺少必填信息时不会发出直接创建请求", async ({ adminPage }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openDirectCustomer();

    const dialog = adminPage.getByRole("dialog");
    await dialog.getByRole("button", { name: /确\s*(?:定|认)/u }).click();

    await expect(dialog).toBeVisible();
    await expect(dialog.locator(".form-valid-error")).not.toHaveCount(0);
    expect(state.directRequests).toHaveLength(0);
    await customers.screenshot("customer-direct-required-validation");
  });

  test("TC-13d: 重置密码弹窗可提交临时密码并保留审计原因", async ({
    adminPage,
  }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    const customerNo = "CUS_0123456789abcdef0123456789abcdef";

    await customers.openPasswordReset(customerNo);
    await expect(
      adminPage.getByText(
        "新密码只作为临时密码。客户下次登录成功后必须设置自己的密码。",
        { exact: true },
      ),
    ).toBeVisible();
    await customers.resetCustomerPassword({
      password: "Temporary-password-123",
      reason: "TC-13 验证管理员重置客户临时密码流程。",
    });

    await expect.poll(() => state.passwordResetRequests.length).toBe(1);
    expect(state.passwordResetRequests[0]).toMatchObject({
      newPassword: "Temporary-password-123",
      reason: "TC-13 验证管理员重置客户临时密码流程。",
    });
    expect(state.passwordResetRequests[0]?.idempotencyKey).toMatch(
      /^customer-password-reset-/u,
    );
    await expect(
      adminPage.getByText("客户密码已重置。", { exact: true }),
    ).toBeVisible();
  });
});
