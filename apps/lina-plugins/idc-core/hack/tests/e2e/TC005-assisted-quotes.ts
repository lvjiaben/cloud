import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { AssistedQuotesPage } from "../pages/AssistedQuotesPage";
import {
  assistedRequestNo,
  installAssistedQuoteMock,
} from "../support/assisted-quote-mock";

test.describe("TC-5 后台协助报价", () => {
  test("TC-5a: 有界目录使用真实总数数字分页且详情保留精确大金额", async ({ adminPage }) => {
    const state = await installAssistedQuoteMock(adminPage);
    const assisted = new AssistedQuotesPage(adminPage);
    await assisted.goto();

    await expect(assisted.pageRoot).toContainText("协助报价");
    await expect
      .poll(async () =>
        assisted.pageRoot.locator(".vxe-grid").first().evaluate((element) =>
          element.getBoundingClientRect().height,
        ),
      )
      .toBeGreaterThan(100);
    await expect(assisted.row(assistedRequestNo)).toContainText(
      "协助报价客户 · 13800138005 · #10005",
    );
    await expect(assisted.row(assistedRequestNo)).not.toContainText("CUS_ASSISTED_0001");

    await assisted.openDetail(assistedRequestNo);
    const drawer = adminPage.getByRole("dialog");
    await expect(drawer).toContainText("高内存云服务器");
    await expect(drawer).toContainText("CNY 0.00");
    await expect(drawer).toContainText("延长预付款期限 · 2 天");
    await expect(drawer).not.toContainText(/CUS_ASSISTED_0001|1001|2002|evidenceDigest|证据摘要值|extended_prepay|pending_approval/);
    await assisted.screenshot("idc-assisted-quote-detail");
    await adminPage.keyboard.press("Escape");
    await expect(drawer).toBeHidden();

    await assisted.pageRoot.getByRole("combobox", { name: "客户" }).click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("协助报价客户 · 13800138005 · #10005", { exact: true })
      .click();
    await assisted.pageRoot.getByRole("button", { name: /搜\s*索/u }).click();
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get("customerNo")).toBe("CUS_ASSISTED_0001");
    const requestCountBeforeNextPage = state.listRequests.length;
    await expect(assisted.pageRoot.locator(".vxe-pager--total")).toContainText("11");
    await expect(assisted.pageRoot.locator(".vxe-pager--sizes")).toBeVisible();
    await assisted.pageRoot.locator(".vxe-pager--next-btn").click();
    await expect.poll(() => state.listRequests.length).toBeGreaterThan(requestCountBeforeNextPage);
    expect(state.listRequests.at(-1)?.searchParams.get("cursor")).toBeNull();
    expect(state.listRequests.at(-1)?.searchParams.get("page")).toBe("2");
    expect(state.listRequests.at(-1)?.searchParams.get("limit")).toBe("10");
    await expect(assisted.pageRoot.locator(".vxe-pager--num-btn.is--active")).toHaveText("2");
    await expect(assisted.row("AQR_20260811_0011")).toBeVisible();
  });

  test("TC-5b: 审批证据只由服务端生成且提交体不接受危险状态", async ({ adminPage }) => {
    const state = await installAssistedQuoteMock(adminPage);
    const assisted = new AssistedQuotesPage(adminPage);
    await assisted.goto();
    await assisted.openDetail(assistedRequestNo);

    await adminPage.getByTestId("assisted-quote-approve").click();
    await adminPage.getByTestId("assisted-decision-reason").fill("Independent approval completed");
    await adminPage
      .locator("button:visible")
      .filter({ hasText: /批\s*准/u })
      .last()
      .click();
    await expect.poll(() => state.decisionBodies.length).toBe(1);
    const body = state.decisionBodies[0] ?? {};
    expect(Object.keys(body).sort()).toEqual([
      "decisionReason",
      "idempotencyKey",
    ]);
    expect(body).not.toHaveProperty("evidenceDigest");
    expect(body).not.toHaveProperty("paid");
    expect(body).not.toHaveProperty("statusCode");
    await expect(adminPage.getByTestId("assisted-quote-checkout")).toBeVisible();
  });

  test("TC-5c: 正常结算只创建待付款订单", async ({ adminPage }) => {
    const state = await installAssistedQuoteMock(adminPage);
    const assisted = new AssistedQuotesPage(adminPage);
    await assisted.goto();

    await assisted.openDetail(assistedRequestNo);
    await adminPage.getByTestId("assisted-quote-approve").click();
    await adminPage.getByTestId("assisted-decision-reason").fill("Independent approval completed");
    await adminPage
      .locator("button:visible")
      .filter({ hasText: /批\s*准/u })
      .last()
      .click();
    await adminPage.getByTestId("assisted-quote-checkout").click();
    const confirm = adminPage.locator(".ant-modal-confirm");
    await expect(confirm).toContainText("待付款订单");
    await expect(confirm).toContainText("不会把订单标为已付款");
    await confirm.locator(".ant-btn-primary").click();
    await expect.poll(() => state.checkoutBodies.length).toBe(1);
    expect(Object.keys(state.checkoutBodies[0] ?? {})).toEqual(["idempotencyKey"]);
    const detailDrawer = adminPage.getByRole("dialog", {
      name: "协助报价详情",
    });
    await expect(detailDrawer).toContainText("ORD_ASSISTED_0001 · 人工审核");
    await expect(adminPage.getByTestId("assisted-quote-checkout")).toHaveCount(0);
    await expect(detailDrawer).not.toContainText(/已付款|运行中|已开通/);
    await expect(confirm).toBeHidden();
    await assisted.screenshot("idc-assisted-order-gate");
  });
});
