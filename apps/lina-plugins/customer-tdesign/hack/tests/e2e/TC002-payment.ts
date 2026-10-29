import { test, expect, invoice } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-2 支付结果", () => {
  test("TC-2a 余额付款确认后仅提交一次，并以重读账单为准", async ({
    consolePage: page,
  }) => {
    let paid = false;
    const requests: unknown[] = [];
    await page.route("**/api/v1/account/billing/invoices/INV-TEST-001*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            detail: {
              invoice: {
                ...invoice,
                statusCode: paid ? "paid" : "open",
                outstandingAmountMinor: paid ? "0" : "12800",
                paidAmountMinor: paid ? "12800" : "0",
              },
              items: [],
              nextItemCursor: "",
            },
          },
        },
      }),
    );
    await page.route("**/invoices/INV-TEST-001/balance-payments", async (r) => {
      requests.push(r.request().postDataJSON());
      paid = true;
      await r.fulfill({ json: { code: 0, data: {} } });
    });
    const view = new CustomerPage(page);
    await view.open("/account/billing/invoices/INV-TEST-001");
    await view.button("立即付款").click();
    await view.button("使用余额付款").click();
    await expect(
      page.getByText("将从当前币种的可用余额扣除 CNY 128.00。"),
    ).toBeVisible();
    await view.screenshot("payment-confirm");
    await page.getByRole("button", { name: "确认", exact: true }).click();
    await expect.poll(() => requests.length).toBe(1);
    await expect(
      view.main.getByText("已付款", { exact: true }).first(),
    ).toBeVisible();
    await expect(view.button("立即付款")).toHaveCount(0);
    await view.screenshot("payment-success");
  });
  test("TC-2b 浏览器回跳参数不能伪造成功", async ({ consolePage: page }) => {
    await page.route("**/account/payments/PAT-test", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            attemptNo: "PAT-test",
            statusCode: "processing",
            totalAmountMinor: "12800",
            currencyCode: "CNY",
            orderNo: "ORD-TEST-001",
          },
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open("/payment/return/PAT-test?success=true");
    await expect(page.getByText("付款结果以到账记录为准")).toBeVisible();
    await expect(page.getByText("付款已完成", { exact: true })).toHaveCount(0);
    await view.screenshot("payment-pending");
  });
});
