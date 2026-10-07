import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { PaymentExceptionFundsPage } from "../pages/PaymentExceptionFundsPage";
import { installPaymentLifecycleMock } from "../support/payment-lifecycle-mock";
import { capturePaymentScreenshot } from "../support/screenshot";

test.describe("TC-2 迟到到账和异常款处置", () => {
  test("TC-2a: 易支付迟到到账只进入待归属资金并可精确分配", async ({
    adminPage,
  }) => {
    const { fundNo, state } = await installPaymentLifecycleMock(adminPage);
    const funds = new PaymentExceptionFundsPage(adminPage);
    await funds.goto();
    await expect(funds.root).toContainText("取消后的到账款不会复活旧订单");
    await expect(funds.root).not.toContainText(/plugin\.idc-payment\./u);
    await expect(funds.row(fundNo)).toContainText("CNY 9007199254740993.01");
    await capturePaymentScreenshot(
      adminPage,
      "payment-exception-fund-allocation",
    );
    await funds.allocate(fundNo, "ORD_new_unpaid_0001");
    expect(state.allocationBodies).toEqual([
      { targetOrderNo: "ORD_new_unpaid_0001" },
    ]);
    await expect(funds.row(fundNo)).toHaveCount(0);
  });

  test("TC-2b: 易支付未验证退款转人工，上传凭证后才完成", async ({
    adminPage,
  }) => {
    const { fundNo, state } = await installPaymentLifecycleMock(adminPage);
    const funds = new PaymentExceptionFundsPage(adminPage);
    await funds.goto();
    await funds.requestRefund(fundNo);
    await expect(funds.row(fundNo)).toContainText("等待人工原路退款凭证");
    await capturePaymentScreenshot(
      adminPage,
      "payment-exception-fund-manual-refund",
    );
    await funds.confirmManualRefund(fundNo, "%PDF-1.7\nrefund proof");
    expect(state.refundRequests).toBe(1);
    expect(state.confirmationBodies).toEqual([
      {
        expectedVersion: 2,
        proofDigest: "f".repeat(64),
        proofReference: "host-file:123",
      },
    ]);
    await expect(funds.row(fundNo)).toHaveCount(0);
  });

  test("TC-2c: 退款结果不明时冻结自动动作并等待核对", async ({
    adminPage,
    mainLayout,
  }) => {
    const { fundNo } = await installPaymentLifecycleMock(adminPage, {
      resultUnknown: true,
    });
    const funds = new PaymentExceptionFundsPage(adminPage);
    await funds.goto();
    const row = funds.row(fundNo);
    await expect(row).toContainText("退款结果待核对");
    await expect(row.getByRole("button")).toHaveCount(0);
    await capturePaymentScreenshot(
      adminPage,
      "payment-exception-fund-result-unknown",
    );
    await mainLayout.switchLanguage("English");
    await expect(funds.root).toContainText("Unassigned funds");
    await expect(funds.root).toContainText(
      "Refund result awaiting reconciliation",
    );
    await expect(funds.root).not.toContainText(/plugin\.idc-payment\./u);
  });
});
