import { test, expect, now } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-3 续费", () => {
  test("TC-3a 先核对报价、过期报价不确认、有效报价进入真实账单", async ({
    consolePage: page,
  }) => {
    let expired = true,
      quoteCalls = 0;
    const confirmations: unknown[] = [];
    await page.route("**/api/v1/account/billing/renewal-entitlements*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            entitlements: [
              {
                entitlementNo: "ENT-1",
                entitlementVersion: 1,
                productNameZhCN: "云服务器 · 基础型",
                entitlementExpiresAt: now + 86400000,
                entitlementStatusCode: "active",
                autoRenewEnabled: false,
                canConfigureAutomaticRenewal: true,
                currencyCode: "CNY",
              },
            ],
            nextCursor: "",
          },
        },
      }),
    );
    await page.route("**/entitlements/ENT-1/renewal-quotes", (r) => {
      quoteCalls++;
      return r.fulfill({
        json: {
          code: 0,
          data: {
            quote: {
              quoteNo: "QUOTE-1",
              totalAmountMinor: "12800",
              currencyCode: "CNY",
              expiresAt: expired ? 1 : Date.now() + 60000,
              projectedExpiresAt: now + 30 * 86400000,
            },
          },
        },
      });
    });
    await page.route("**/renewal-quotes/QUOTE-1/confirmations", (r) => {
      confirmations.push(r.request().postDataJSON());
      return r.fulfill({
        json: { code: 0, data: { operation: { invoiceNo: "INV-TEST-001" } } },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/billing/renewals");
    await view.button("续费").click();
    await view.button("查看续费金额").click();
    await expect(page.locator(".t-dialog")).toContainText("CNY 128.00");
    await view.button("确认续费并付款").click();
    await expect(page.getByText("报价已过期，请重新获取")).toBeVisible();
    expect(confirmations).toHaveLength(0);
    await view.screenshot("renewal-expired");
    expired = false;
    await view.button("查看续费金额").click();
    await view.button("确认续费并付款").click();
    await expect.poll(() => confirmations.length).toBe(1);
    expect(quoteCalls).toBe(2);
    await expect(page).toHaveURL(/invoices\/INV-TEST-001/);
    await expect(page.getByRole("heading", { name: "账单详情" })).toBeVisible();
    await view.screenshot("renewal-invoice");
  });
});
