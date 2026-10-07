import { test, expect } from "../../../../../../hack/tests/fixtures/auth";

import { CustomerAssistancePage } from "../pages/CustomerAssistancePage";
import { CustomerManagementPage } from "../pages/CustomerManagementPage";

test.setTimeout(60_000);

test.describe("TC-20 客户运营与完整模拟登录", () => {
  test("TC-20a: 客户 360 使用可读状态且运营弹窗可反复打开", async ({
    adminPage,
  }) => {
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openOverviewByPublicId("10000");

    await expect(customers.overviewDrawer).toContainText("账户余额");
    await expect(customers.overviewDrawer).toContainText("全部订单");
    for (const tabKey of ["services", "tickets", "contracts", "records"]) {
      await customers.overviewDrawer
        .getByTestId(`customer-360-tab-${tabKey}`)
        .click();
      await expect(customers.overviewDrawer).toContainText("进入管理");
    }
    await customers.screenshot("customer-360-business-surfaces");
    await expect(customers.overviewDrawer).not.toContainText(
      /NTE_|CUS_|demo_/u,
    );
    await customers.reopenStatusEditor();
    await customers.showActivity();
    for (const label of ["添加备注", "安排跟进", "提交黑名单"]) {
      await customers.reopenActivityDialog(label);
    }
    await customers.submitBlacklist();
    await customers.decideLatestBlacklist("批准", "已生效");
    await customers.decideLatestBlacklist("撤销", "已撤销");
    await customers.submitBlacklist();
    await customers.decideLatestBlacklist("驳回", "已驳回");
    await customers.screenshot("customer-operations-dialog-reopen");
  });

  test("TC-20b: 模拟登录进入用户中心并保留完整业务页面", async ({
    adminPage,
  }) => {
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openOverviewByPublicId("10000");

    const popup = await customers.startAssistance(
      "TC-20 验证客户完整业务页面与全程留痕",
    );
    const assistance = new CustomerAssistancePage(popup);
    await assistance.waitUntilReady();
    await expect(popup.getByTestId("customer-assistance-banner")).toContainText(
      "正在模拟登录",
    );
    await expect(popup.locator("body")).not.toContainText(/CUS_|NTE_|demo_/u);

    await assistance.gotoOrders();
    await expect(popup.getByTestId("customer-commerce-page")).toContainText(
      "我的订单",
    );
    await expect(popup.getByTestId("customer-commerce-page")).toContainText(
      "交付中",
    );
    await expect(popup.getByTestId("customer-commerce-page")).toContainText(
      "已交付",
    );
    await expect(popup.getByTestId("customer-commerce-page")).not.toContainText(
      /\b(?:fulfilling|fulfilled)\b/u,
    );
    await assistance.screenshot("customer-assistance-orders");
  });
});
