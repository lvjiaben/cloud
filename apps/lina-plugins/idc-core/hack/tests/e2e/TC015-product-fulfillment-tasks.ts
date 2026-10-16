import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ProductFulfillmentPage } from "../pages/ProductFulfillmentPage";
import { installCatalogAutomationMock } from "../support/catalog-automation-mock";
import { installProductFulfillmentMock } from "../support/product-fulfillment-mock";

const permissions = [
  "idc:admin:product-fulfillment:task:list",
  "idc:admin:product-fulfillment:task:query",
  "idc:admin:product-fulfillment:review-approve",
  "idc:admin:product-fulfillment:review-reject",
  "idc:admin:product-fulfillment:takeover",
  "idc:admin:product-fulfillment:manual-conclude",
];

test.describe("TC-15 商品履约任务安全操作", () => {
  test("TC-15a: 六态列表和安全历史不显示内部供应商信息", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage);
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();

    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("待商家审核");
    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("证书");
    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("第 1 / 共 2 个");
    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("履约测试客户 · 139****8009 · #10009");
    await expect(fulfillment.row("ORD-MOCK-1")).not.toContainText("CUS_fulfillment_customer_0001");
    await fulfillment.pageRoot.getByRole("combobox", { name: "客户" }).click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("履约测试客户 · 139****8009 · #10009", { exact: true })
      .click();
    await fulfillment.pageRoot.getByRole("button", { name: /搜\s*索/u }).click();
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get("customerNo")).toBe("CUS_fulfillment_customer_0001");
    await fulfillment.openDetail("ORD-MOCK-1");
    const drawer = adminPage.locator('[role="dialog"]:visible').last();
    await expect(drawer).toContainText("资金结清");
    await expect(drawer).toContainText("已确认结清");
    await expect(drawer).toContainText("履约任务已建立");
    await expect(drawer).toContainText("第 1 / 共 2 个");
    await expect(drawer).toContainText("履约测试客户 · 139****8009 · #10009");
    await expect(drawer).not.toContainText("CUS_fulfillment_customer_0001");
    await expect(drawer).not.toContainText(
      /PFI-MOCK|ITEM-MOCK|plugin_id|provider_secret|supplier_payload|evidence_digest|\{"/iu,
    );
    await fulfillment.screenshot("product-fulfillment-safe-detail");
  });

  test("TC-15b: 响应丢失后重试批准动作复用同一个幂等键", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage, {
      loseFirstActionResponse: true,
    });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.openDetail("ORD-MOCK-1");
    const drawer = adminPage.locator('[role="dialog"]:visible').last();
    await drawer
      .getByRole("button", { name: /审核通过并开始履约|批准并开始履约/u })
      .click();
    const modal = adminPage
      .locator('[role="dialog"]')
      .filter({ has: adminPage.getByLabel("处理原因") });
    await modal.getByLabel("处理原因").click();
    await adminPage.locator(".ant-select-dropdown:visible").getByText("已核对", { exact: true }).click();
    await modal.getByRole("button", { name: /确\s*(认|定)/u }).click();
    await expect(
      adminPage.getByText("当前状态可能已经变化，本次操作未生效，请刷新后重试。"),
    ).toBeVisible();
    await expect(modal).toBeVisible();
    await modal.getByRole("button", { name: /确\s*(认|定)/u }).click();
    await expect(modal).toBeHidden();

    expect(state.actionBodies).toHaveLength(2);
    expect(state.actionBodies[0]?.idempotencyKey).toBeTruthy();
    expect(state.actionBodies[1]?.idempotencyKey).toBe(
      state.actionBodies[0]?.idempotencyKey,
    );
    expect(state.actionBodies[1]).toMatchObject({ note: "", reasonCode: "verified" });
    await fulfillment.screenshot("product-fulfillment-idempotent-retry");
  });

  test("TC-15c: 人工交付说明必填且响应丢失后重试保留结果", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage, { manual: true, loseFirstActionResponse: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.screenshot("manual-fulfillment-list");
    await fulfillment.openDetail("ORD-MOCK-1");
    await fulfillment.openManualAction();
    await expect(fulfillment.actionModal()).toContainText("登记人工履约已完成");
    await expect(fulfillment.actionModal()).toContainText("按实际结果登记完成。");
    await fulfillment.fillManualAction("过短");
    await fulfillment.submitManualAction();
    await expect(fulfillment.actionModal()).toBeVisible();
    await expect(fulfillment.actionModal()).toContainText("请填写10至512个字的处理说明。");
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("manual-fulfillment-validation");
    const note = "已完成域名注册，交付资料已发送给客户。";
    await fulfillment.actionModal().getByLabel("处理说明").fill(note);
    await fulfillment.screenshot("manual-fulfillment-form");
    await fulfillment.submitManualAction();
    await expect(adminPage.getByText("当前状态可能已经变化，本次操作未生效，请刷新后重试。")).toBeVisible();
    await expect(fulfillment.actionModal().getByLabel("处理说明")).toHaveValue(note);
    await fulfillment.screenshot("manual-fulfillment-retry");
    await fulfillment.submitManualAction();
    await expect(fulfillment.actionModal()).toBeHidden();
    expect(state.actionBodies).toHaveLength(2);
    expect(state.actionBodies[1]).toEqual(state.actionBodies[0]);
    expect(state.actionBodies[1]).toMatchObject({ expectedVersion: 3, outcomeCode: "completed", note, reasonCode: "manual_fulfillment_completed" });
    await expect(fulfillment.row("ORD-MOCK-1")).toContainText("已完成");
    await expect(fulfillment.detail()).toContainText("人工履约已登记完成");
    await expect(fulfillment.detail()).toContainText(note);
    await expect(fulfillment.detail().getByRole("button", { name: "登记已完成", exact: true })).toHaveCount(0);
    await fulfillment.screenshot("manual-fulfillment-completed");
  });

  test("TC-15d: 无法交付保留原因且不冒充成功", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage, { manual: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.openDetail("ORD-MOCK-1");
    await fulfillment.openManualAction(false);
    const note = "供应商域名库存不足，已通知客户办理退款。";
    await fulfillment.fillManualAction(note, false);
    await fulfillment.submitManualAction();
    await expect(fulfillment.actionModal()).toBeHidden();
    expect(state.actionBodies).toHaveLength(1);
    expect(state.actionBodies[0]).toMatchObject({ outcomeCode: "unable_to_complete", note, reasonCode: "manual_fulfillment_unable" });
    await expect(fulfillment.detail()).toContainText("人工履约已登记无法完成");
    await expect(fulfillment.detail()).toContainText(note);
    await fulfillment.screenshot("manual-fulfillment-unable");
  });

  test("TC-15e: 没有人工交付权限不显示写操作也不请求", async ({ adminPage }) => {
    await installCatalogAutomationMock(adminPage, { permissions: permissions.filter((permission) => !permission.endsWith(":manual-conclude")) });
    const state = await installProductFulfillmentMock(adminPage, { manual: true });
    const fulfillment = new ProductFulfillmentPage(adminPage);
    await fulfillment.goto();
    await fulfillment.openDetail("ORD-MOCK-1");
    await expect(fulfillment.detail().getByRole("button", { name: /登记已完成|登记无法完成/u })).toHaveCount(0);
    expect(state.actionBodies).toHaveLength(0);
    await fulfillment.screenshot("manual-fulfillment-readonly");
  });
});
