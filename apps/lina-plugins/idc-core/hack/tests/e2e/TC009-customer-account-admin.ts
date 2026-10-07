import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { CustomerManagementPage } from "../pages/CustomerManagementPage";
import { installCustomerManagementMock } from "../support/customer-mock";

test.describe("TC009 客户开户与激活邀请", () => {
  test("TC-2e: 后台创建个人和企业客户且不提交密码", async ({ adminPage }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();

    await customers.openCustomerActivation();
    await expect(
      adminPage
        .getByTestId("customer-activation-country")
        .locator('[role="combobox"]'),
    ).toBeVisible();
    const requiredLabelLayout = await adminPage
      .getByRole("dialog")
      .locator(".form-is-required label")
      .first()
      .evaluate((label) => {
        const marker = label.querySelector(".text-destructive");
        const labelBox = label.getBoundingClientRect();
        return {
          alignItems: getComputedStyle(label).alignItems,
          display: getComputedStyle(label).display,
          flexWrap: getComputedStyle(label).flexWrap,
          markerText: marker?.textContent,
          height: labelBox.height,
        };
      });
    expect(["flex", "inline-flex"]).toContain(requiredLabelLayout.display);
    expect(requiredLabelLayout.alignItems).toBe("center");
    expect(requiredLabelLayout.flexWrap).toBe("nowrap");
    expect(requiredLabelLayout.markerText).toBe("*");
    expect(requiredLabelLayout.height).toBeLessThan(32);
    await customers.screenshot(
      "customer-activation-country-and-required-label",
    );
    await customers.fillCustomerActivation({
      customerType: "individual",
      destination: "buyer@example.com",
      displayName: "个人邀请客户",
      reason: "客户确认需要创建独立客户账号。",
    });
    await customers.submitCustomerActivation();
    await expect.poll(() => state.activationRequests.length).toBe(1);
    expect(state.activationRequests[0]).toMatchObject({
      customerType: "individual",
      email: "buyer@example.com",
      bindExistingIdentity: false,
    });
    expect(state.activationRequests[0]).not.toHaveProperty("password");

    await customers.openCustomerActivation();
    await customers.fillCustomerActivation({
      customerType: "enterprise",
      destinationKind: "sms",
      destination: "13800138000",
      displayName: "企业邀请客户",
      reason: "企业负责人确认使用短信接收激活邀请。",
    });
    await customers.submitCustomerActivation();
    await expect.poll(() => state.activationRequests.length).toBe(2);
    expect(state.activationRequests[1]).toMatchObject({
      customerType: "enterprise",
      phoneCountryCode: "+86",
      phone: "13800138000",
    });
    expect(state.activationRequests[1]).not.toHaveProperty("password");
    await customers.screenshot("customer-activation-individual-enterprise");
  });

  test("TC-2f: 绑定已有无受众身份并保留明确选择", async ({ adminPage }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openCustomerActivation();
    await customers.fillCustomerActivation({
      bindExisting: true,
      customerType: "individual",
      destination: "existing@example.com",
      displayName: "已有账号客户",
      reason: "客户确认绑定当前已有的独立登录账号。",
    });
    await customers.submitCustomerActivation();
    await expect.poll(() => state.activationRequests.length).toBe(1);
    expect(state.activationRequests[0]).toMatchObject({
      bindExistingIdentity: true,
      email: "existing@example.com",
    });
  });

  test("TC-2g: 员工身份冲突时页面拒绝且不关闭填写窗口", async ({
    adminPage,
  }) => {
    await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openCustomerActivation();
    await customers.fillCustomerActivation({
      bindExisting: true,
      customerType: "individual",
      destination: "staff@example.com",
      displayName: "员工冲突客户",
      reason: "验证员工身份不能直接转换为客户身份。",
    });
    await customers.submitCustomerActivation();
    await expect(
      adminPage.getByText("员工账号不能绑定为客户，请使用独立客户账号", {
        exact: true,
      }),
    ).toBeVisible();
    await expect(adminPage.getByRole("dialog")).toBeVisible();
  });

  test("TC-2h: 待激活客户重发时只产生一个新邀请请求", async ({ adminPage }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openCustomerActivation();
    await customers.fillCustomerActivation({
      customerType: "individual",
      destination: "buyer@example.com",
      displayName: "待重发客户",
      reason: "先创建待激活客户用于验证重发流程。",
    });
    await customers.submitCustomerActivation();
    await expect.poll(() => state.activationRequests.length).toBe(1);
    const customerNo = "CUS_00000000000000000000000000000001";
    await adminPage
      .getByTestId(`customer-activation-resend-${customerNo}`)
      .click();
    await adminPage
      .getByRole("button", { name: "确认重新发送", exact: true })
      .click();
    await expect.poll(() => state.resendRequests.length).toBe(1);
    expect(state.resendRequests[0]?.idempotencyKey).toMatch(
      /^customer-activation-resend-/u,
    );
  });
});
