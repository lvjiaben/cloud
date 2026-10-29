import { test, expect } from "@host-tests/fixtures/auth";
import { LoginPage } from "@host-tests/pages/LoginPage";
import { resolve } from "node:path";
import { InvoiceOnlinePaymentPage } from "../pages/InvoiceOnlinePaymentPage";

for (const [index, mode] of (
  [
    "QR",
    "GET",
    "IDENTITY",
    "BLOCKED",
    "HIDDEN",
    "POST",
    "PAID",
    "BALANCE",
  ] as const
).entries()) {
  const title = {
    QR: "账单支付失败保留选择，重试显示二维码",
    GET: "账单支付失败保留选择，重试跳转渠道",
    IDENTITY: "未实名保留账单并引导认证，拒绝创建支付",
    BLOCKED: "订单不可付款时拒绝创建支付",
    HIDDEN: "支付模块不可用时隐藏付款入口",
    POST: "支付宝表单支付按渠道字段安全跳转",
    PAID: "账单已付款时刷新金额并停止在线付款",
    BALANCE: "余额已扣款但结清失败时刷新账单防止重复付款",
  }[mode];
  test(`TC-3${String.fromCharCode(97 + index)}: ${title}`, async ({ page }) => {
    const invoiceNo = "INV_checkout_ui",
      orderNo = "ORD_checkout_ui";
    let fail = true;
    let invoicePaid = false;
    let balanceCalls = 0;
    let channelReads = 0;
    const requests: any[] = [];
    await page.route("**/api/v1/auth/login", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            accessToken: "fixture-invoice-session",
            refreshToken: "fixture-invoice-refresh",
            tenants: [],
          },
        },
      }),
    );
    await page.route("**/api/v1/user/info", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            id: 702,
            username: "checkout-ui",
            realName: "结算测试客户",
            roles: ["user"],
            permissions: [],
            menus: [],
            homePath: "/account/billing/invoices",
          },
        },
      }),
    );
    await page.route("**/api/v1/plugins/dynamic", (route) =>
      route.fulfill({ json: { code: 0, data: { list: [] } } }),
    );
    await page.route("**/customer/external-menus", (route) =>
      route.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    await page.route(
      "**/x/idc-core/api/v1/account/product-directories*",
      (route) =>
        route.fulfill({
          json: {
            code: 0,
            data: { items: [], total: 0, page: 1, pageSize: 100 },
          },
        }),
    );
    await page.route(
      "**/x/idc-core/api/v1/account/initial-password/status",
      (route) =>
        route.fulfill({ json: { code: 0, data: { required: false } } }),
    );
    await page.route(
      "**/x/idc-core/api/v1/account/billing/invoices*",
      (route) =>
        route.fulfill({
          json: { code: 0, data: { invoices: [], nextCursor: "" } },
        }),
    );
    await page.route(
      "**/x/idc-core/api/v1/account/billing/invoices/" + invoiceNo + "*",
      (route) =>
        route.fulfill({
          json: {
            code: 0,
            data: {
              detail: {
                invoice: {
                  invoiceNo,
                  orderNo,
                  currencyCode: "CNY",
                  statusCode: invoicePaid ? "paid" : "pending_payment",
                  outstandingAmountMinor: invoicePaid ? "0" : "110000",
                  totalAmountMinor: "110000",
                  canPayFromBalance: mode === "BALANCE" && !invoicePaid,
                  paidAmountMinor: invoicePaid ? "110000" : "0",
                  refundedAmountMinor: "0",
                  waivedAmountMinor: "0",
                  itemCount: 1,
                },
                items: [],
              },
            },
          },
        }),
    );
    await page.route(
      "**/x/idc-core/api/v1/account/billing/invoices/" +
        invoiceNo +
        "/balance-payments",
      (route) => {
        balanceCalls++;
        invoicePaid = true;
        return route.fulfill({
          json: { code: 55, message: "当前订单不满足财务结清条件" },
        });
      },
    );
    await page.route(
      "**/x/idc-core/api/v1/account/billing/credit/overview",
      (route) =>
        route.fulfill({
          json: { code: 0, data: { overview: { accounts: [] } } },
        }),
    );
    await page.route(
      "**/x/idc-core/api/v1/account/commerce/orders/" +
        orderNo +
        "/payment-preparations",
      (route) => {
        if (mode === "PAID") invoicePaid = true;
        return route.fulfill({
          json: {
            code: 0,
            data: {
              preparation: {
                ready: mode === "QR" || mode === "GET" || mode === "POST",
                statusCode:
                  mode === "IDENTITY"
                    ? "awaiting_identity"
                    : mode === "BLOCKED"
                      ? "cancelled"
                      : mode === "PAID"
                        ? "closed"
                        : "ready",
                contextNo: "CTX_checkout_ui",
                invoiceNo,
                orderNo,
              },
            },
          },
        });
      },
    );
    await page.route(
      "**/x/idc-payment/api/v1/account/payment-channels*",
      (route) => {
        channelReads++;
        if (mode === "HIDDEN" || mode === "BALANCE")
          return route.fulfill({ status: 404, json: { code: 44 } });
        return route.fulfill({
          json: {
            code: 0,
            data: {
              channels: [
                {
                  channelNo: "CHN_checkout_ui",
                  displayName: mode === "QR" ? "微信支付" : "支付宝",
                  currencies: ["CNY"],
                  modes: [mode === "QR" ? "wechat_native" : "alipay_page"],
                },
              ],
            },
          },
        });
      },
    );
    await page.route("**/x/idc-payment/api/v1/account/payments", (route) => {
      requests.push(route.request().postDataJSON());
      if (fail)
        return route.fulfill({
          status: 503,
          json: { code: 50, message: "temporary" },
        });
      return route.fulfill({
        json: {
          code: 0,
          data: {
            attemptNo: "PAY_checkout_ui",
            redirectMethod: mode,
            redirectUrl:
              mode === "QR"
                ? "weixin://fixture-only-qr"
                : "https://payments.example.invalid/checkout",
            redirectFields:
              mode === "POST" ? { fixtureField: "fixture-value" } : {},
            statusCode: "pending",
            expiresAt: Date.now() + 60000,
          },
        },
      });
    });
    await page
      .context()
      .route("https://payments.example.invalid/checkout", (route) => {
        if (mode === "POST") {
          expect(route.request().method()).toBe("POST");
          expect(
            new URLSearchParams(route.request().postData() || "").get(
              "fixtureField",
            ),
          ).toBe("fixture-value");
        }
        return route.fulfill({
          contentType: "text/html; charset=utf-8",
          body: "<h1>支付渠道确认付款</h1>",
        });
      });
    await page.goto(
      `${process.env.CUSTOMER_E2E_BASE_URL || "http://127.0.0.1:6032"}/auth/login`,
    );
    await new LoginPage(page).login("checkout-ui", "fixture-only-password");
    const ui = new InvoiceOnlinePaymentPage(page);
    await ui.openInvoice(invoiceNo);
    if (mode === "BALANCE") {
      await page.getByTestId("finance-balance-pay").click();
      await page
        .locator(".ant-modal-confirm")
        .getByRole("button", { name: "确认支付", exact: true })
        .click();
      await expect(page.getByTestId("finance-balance-pay")).toHaveCount(0);
      await expect(
        page
          .getByText("账单已付款，无需重复支付。订单处理进度请查看订单。", {
            exact: true,
          })
          .first(),
      ).toBeVisible();
      expect(balanceCalls).toBe(1);
      expect(requests).toHaveLength(0);
      await expect(page.locator(".ant-modal-confirm")).toBeHidden();
      await ui.screenshot("BALANCE-confirmed");
      return;
    }
    if (mode === "PAID") {
      await expect(page.getByRole("dialog")).toContainText(
        "账单已付款，无需重复支付。订单处理进度请查看订单。",
      );
      await expect(page.getByTestId("invoice-payment-submit")).toHaveCount(0);
      await expect(page.getByTestId("invoice-online-pay")).toHaveCount(0);
      expect(requests).toHaveLength(0);
      await ui.screenshot("PAID-confirmed");
      return;
    }
    if (mode === "HIDDEN") {
      await expect.poll(() => channelReads).toBeGreaterThan(0);
      await expect(page.getByTestId("invoice-online-pay")).toHaveCount(0);
      await expect(page.getByRole("dialog")).toHaveCount(0);
      expect(requests).toHaveLength(0);
      return;
    }
    await expect(page.getByRole("dialog")).toContainText("选择支付方式");
    if (mode === "IDENTITY" || mode === "BLOCKED") {
      await expect(
        page.getByText(
          mode === "IDENTITY"
            ? "请先完成实名认证，再支付当前账单。"
            : "当前订单暂不能付款，请核对订单状态或联系商家。",
          { exact: true },
        ),
      ).toBeVisible();
      await expect(page.getByTestId("invoice-payment-submit")).toHaveCount(0);
      expect(requests).toHaveLength(0);
      await ui.screenshot(mode + "-blocked");
      return;
    }
    await expect(page.getByTestId("invoice-payment-channel")).toContainText(
      mode === "QR" ? "微信支付" : "支付宝",
    );
    await expect(page.getByTestId("invoice-payment-mode")).toContainText(
      mode === "QR" ? "微信扫码" : "支付宝电脑网站",
    );
    await ui.screenshot(mode + "-channel");
    await ui.pay();
    await expect(
      page.getByText("付款准备失败，订单和选择已保留，请重试。", {
        exact: true,
      }),
    ).toBeVisible();
    await expect(page.getByTestId("invoice-payment-channel")).toContainText(
      mode === "QR" ? "微信支付" : "支付宝",
    );
    await ui.screenshot(mode + "-failure");
    fail = false;
    await ui.pay();
    if (mode === "QR") {
      await expect(page.getByTestId("invoice-payment-qr")).toHaveAttribute(
        "src",
        /^data:image\/png;base64,/,
      );
      await ui.screenshot("QR-result");
    } else
      await expect(
        page.getByRole("heading", { name: "支付渠道确认付款" }),
      ).toBeVisible();
    expect(requests).toHaveLength(2);
    expect(requests[0]).toEqual(requests[1]);
    expect(requests[0]).toEqual({
      paymentContextNo: "CTX_checkout_ui",
      channelNo: "CHN_checkout_ui",
      modeCode: mode === "QR" ? "wechat_native" : "alipay_page",
      balanceAmountMinor: "0",
    });
  });
}
