import { customerTest as test } from "../support/customer-admin-session";
import { expect } from "../../../../../../hack/tests/fixtures/auth";

import { CustomerAssistancePage } from "../pages/CustomerAssistancePage";
import { CustomerManagementPage } from "../pages/CustomerManagementPage";

test.setTimeout(60_000);

test.describe("TC-20 客户运营与完整模拟登录", () => {
  test("TC-20a: 客户详情 使用可读状态且运营弹窗可反复打开", async ({
    adminPage,
  }) => {
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openOverviewByPublicId("10000");

    await expect(customers.overviewDrawer).toContainText("账户余额");
    await expect(customers.overviewDrawer).toContainText("全部订单");
    for (const tabKey of ["services", "tickets", "tax-invoices"]) {
      await customers.overviewDrawer
        .getByTestId(`customer-360-tab-${tabKey}`)
        .click();
      await expect(customers.overviewDrawer).toContainText("进入管理");
    }
    await customers.screenshot("customer-360-business-surfaces");
    await expect(customers.overviewDrawer).not.toContainText(
      /NTE_|CUS_|demo_/u,
    );
    await customers.reopenStatusConfirm();
    await customers.openDetailTab("备注");
    await customers.reopenActivityDialog("添加备注");
    await expect(customers.overviewDrawer).not.toContainText("运营足迹");
    await customers.screenshot("customer-operations-dialog-reopen");
  });

  test("TC-20b: 模拟登录进入用户中心并保留完整业务页面", async ({
    adminPage,
  }) => {
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openOverviewByPublicId("10000");

    const firstExchange = adminPage.context().waitForEvent("response", {
      predicate: response => response.request().method() === "POST" &&
        new URL(response.url()).pathname.endsWith("/customer-assistance/access-verifications"),
    });
    const popup = await customers.startAssistance();
    const assistance = new CustomerAssistancePage(popup);
    await assistance.waitUntilReady();
    await expect(popup.getByTestId("customer-assistance-banner")).toContainText(
      "正在模拟登录",
    );
    await expect(popup.locator("body")).not.toContainText(/CUS_|NTE_|demo_/u);

    const ordersResponse = await assistance.gotoOrders();
    const oldAuthorization = ordersResponse.request().headers().authorization;
    expect(oldAuthorization).toBeTruthy();
    const firstVerification = await firstExchange;
    const firstSession = (await firstVerification.json()).data.sessionNo;
    expect(ordersResponse.ok()).toBe(true);
    const ordersPayload = await ordersResponse.json();
    expect(ordersPayload.code).toBe(0);
    const orders = ordersPayload.data.orders;
    expect(Array.isArray(orders)).toBe(true);
    await expect(popup.getByTestId("customer-commerce-page")).toContainText(
      "我的订单",
    );
    if (orders.length === 0) {
      await expect(popup.getByTestId("customer-commerce-page")).toContainText("暂无订单");
    } else {
      for (const order of orders) {
        await expect(popup.getByTestId("customer-commerce-page")).toContainText(order.orderNo);
      }
    }
    await expect(popup.getByTestId("customer-commerce-page")).not.toContainText(
      /\b(?:fulfilling|fulfilled)\b/u,
    );
    await assistance.screenshot("customer-assistance-orders");
    // The second launch revokes the first delegated token, which remains in
    // customer-origin storage. A fresh handoff must still succeed.
    const secondExchange = adminPage.context().waitForEvent("response", {
      predicate: response => response.request().method() === "POST" &&
        new URL(response.url()).pathname.endsWith("/customer-assistance/access-verifications"),
    });
    const secondPopup = await customers.startAssistance();
    const secondAssistance = new CustomerAssistancePage(secondPopup);
    await secondAssistance.waitUntilReady();
    await expect(secondPopup.getByTestId("customer-assistance-banner")).toContainText("正在模拟登录");
    await expect(secondPopup.locator("body")).not.toContainText("无效的Token");
    await expect(secondPopup.locator("body")).not.toContainText("登录认证过期");
    const secondVerification = await secondExchange;
    const secondSession = (await secondVerification.json()).data.sessionNo;
    expect(secondSession).not.toBe(firstSession);
    const oldRequest = await popup.request.get(ordersResponse.url(), {
      headers: { Authorization: oldAuthorization },
    });
    expect(oldRequest.status()).toBe(401);
    const usedHandoff = await secondPopup.request.post(secondVerification.url(), {
      data: secondVerification.request().postDataJSON(),
    });
    expect((await usedHandoff.json()).code).not.toBe(0);
    await expect.poll(() => secondPopup.evaluate(() =>
      sessionStorage.getItem("idc-core:pending-assistance"),
    )).toBeNull();
    await secondAssistance.screenshot("customer-assistance-second-login");
  });
});
