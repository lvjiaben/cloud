// UI-only generic management readiness, with isolated entry support.
// Local association status never grants a new provider or financial operation.
import { test, expect } from "../support/merchant-auth";
import { ProductFulfillmentPage } from "../pages/ProductFulfillmentPage";
import { installCatalogAutomationMock } from "../support/catalog-automation-mock";
import { installProductFulfillmentMock } from "../support/product-fulfillment-mock";
const permissions = [
  "idc:admin:product-fulfillment:task:list",
  "idc:admin:product-fulfillment:task:query",
  "idc:admin:product-fulfillment:recover",
  "idc:admin:billing:refund:review",
];
test.describe("TC-77 通用服务管理准备", () => {
  test("TC-77a: 已交付通用商品单独显示本地管理异常，不开放重新开通", async ({
    adminPage,
    mainLayout,
  }) => {
    await installCatalogAutomationMock(adminPage, { permissions });
    const state = await installProductFulfillmentMock(adminPage, {
      managementPreparation: {
        statusCode: "needs_attention",
        attemptCount: 10,
        failureCode: "owner_delivery_unavailable",
      },
    });
    const view = new ProductFulfillmentPage(adminPage);
    await view.goto();
    await view.openDetail("ORD-MOCK-1");
    const progress = view
      .detail()
      .getByTestId("fulfillment-management-preparation");
    await expect(progress).toContainText(
      "服务已交付，但管理入口关联需要管理员处理。请勿重新开通。",
    );
    await expect(progress).toContainText("本地关联尝试次数");
    await expect(progress).toContainText("10");
    await expect(progress).toContainText("管理插件尚未确认本地关联结果。");
    await expect(view.detail()).toContainText("已完成");
    await expect(
      view.detail().getByRole("button", { name: "重新开通", exact: true }),
    ).toHaveCount(0);
    await expect(
      view.detail().getByRole("button", { name: "申请退款", exact: true }),
    ).toHaveCount(0);
    expect(state.actionBodies).toHaveLength(0);
    await progress.scrollIntoViewIfNeeded();
    await view.screenshot("generic-management-attention");
    await adminPage.keyboard.press("Escape");
    await expect(view.detail()).toHaveCount(0);
    await mainLayout.switchLanguage("English");
    await view.pageRoot
      .getByRole("button", { name: /detail/iu })
      .first()
      .click();
    await expect(progress).toContainText(
      "The service is delivered, but an administrator must resolve its management access. Do not provision it again.",
    );
    await expect(progress).toContainText("Local linking attempts");
    await expect(progress).toContainText(
      "The management plugin has not confirmed the local link.",
    );
    await expect(progress).not.toContainText("plugin.idc-core.");
    await progress.scrollIntoViewIfNeeded();
    await view.screenshot("generic-management-attention-english");
  });
});

test("TC-77b: 本地管理关联失败直接重试，丢失响应复用操作且不重新开通", async ({
  adminPage,
}) => {
  await installCatalogAutomationMock(adminPage, { permissions });
  const state = await installProductFulfillmentMock(adminPage, {
    managementPreparation: {
      statusCode: "needs_attention",
      attemptCount: 10,
      failureCode: "owner_delivery_unavailable",
    },
    managementRepair: true,
    loseFirstActionResponse: true,
  });
  const view = new ProductFulfillmentPage(adminPage);
  await view.goto();
  await view.openDetail("ORD-MOCK-1");
  const progress = view
    .detail()
    .getByTestId("fulfillment-management-preparation");
  const button = progress.getByRole("button", {
    name: "重新关联管理入口",
    exact: true,
  });
  await button.click();
  await expect(progress).toContainText("暂时无法确认关联结果，请重试。");
  await expect(button).toBeEnabled();
  expect(state.actionBodies).toHaveLength(1);
  await button.click();
  await expect(progress).toContainText("服务已交付，正在关联管理入口。");
  await expect(button).toHaveCount(0);
  await expect(view.detail()).toContainText("已完成");
  await expect(
    view.detail().getByRole("button", { name: "重新开通", exact: true }),
  ).toHaveCount(0);
  await expect(
    view.detail().getByRole("button", { name: "申请退款", exact: true }),
  ).toHaveCount(0);
  expect(state.actionBodies).toHaveLength(2);
  expect(state.actionBodies[0]).toEqual(state.actionBodies[1]);
  expect(Object.keys(state.actionBodies[0]).sort()).toEqual([
    "expectedVersion",
    "idempotencyKey",
  ]);
  expect(state.actionBodies[0].expectedVersion).toBe(3);
  await progress.scrollIntoViewIfNeeded();
  await view.screenshot("native-management-repair-queued");
});

test("TC-77c: 只有查询权限时，本地修复资格不产生操作入口", async ({
  adminPage,
}) => {
  await installCatalogAutomationMock(adminPage, {
    permissions: permissions.filter(
      (code) => !code.endsWith(":recover") && !code.includes(":refund:"),
    ),
  });
  const state = await installProductFulfillmentMock(adminPage, {
    managementPreparation: { statusCode: "needs_attention", attemptCount: 10 },
    managementRepair: true,
  });
  const view = new ProductFulfillmentPage(adminPage);
  await view.goto();
  await view.openDetail("ORD-MOCK-1");
  await expect(
    view.detail().getByTestId("fulfillment-management-preparation"),
  ).toBeVisible();
  await expect(
    view
      .detail()
      .getByRole("button", { name: "重新关联管理入口", exact: true }),
  ).toHaveCount(0);
  expect(state.actionBodies).toHaveLength(0);
});
