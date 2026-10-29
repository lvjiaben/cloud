import { test, expect, invoice } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";

test("TC-16a 官网支付偏好只预选，刷新后需客户确认才能发起付款", async ({
  consolePage: page,
}) => {
  let payments = 0,
    preparations = 0;
  await page.route("**/account/payment-channels*", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          channels: [
            {
              channelNo: "ALI",
              displayName: "支付宝",
              modes: ["page"],
              currencyCodes: ["CNY"],
            },
            {
              channelNo: "APP",
              displayName: "应用支付",
              modes: ["app"],
              currencyCodes: ["CNY"],
            },
          ],
        },
      },
    }),
  );
  await page.route("**/account/payments", (r) => {
    payments++;
    return r.fulfill({ status: 503, json: { code: 503 } });
  });
  await page.route("**/payment-preparations", (r) => {
    preparations++;
    return r.fulfill({ status: 503, json: { code: 503 } });
  });
  const view = new CustomerPage(page);
  await view.open(
    "/account/billing/invoices/INV-TEST-001?pay=1&paymentChannel=ALI",
  );
  await expect(page.getByRole("dialog")).toContainText("支付账单");
  await expect(
    page.getByRole("textbox", { name: "支付方式", exact: true }),
  ).toHaveValue("支付宝");
  await expect(
    page.getByRole("button", { name: "前往支付", exact: true }),
  ).toBeEnabled();
  expect(payments).toBe(0);
  expect(preparations).toBe(0);
  await view.screenshot("website-payment-channel-preselected");
  await page.reload();
  await expect(
    page.getByRole("button", { name: "前往支付", exact: true }),
  ).toBeEnabled();
  expect(payments).toBe(0);
  expect(preparations).toBe(0);
});

test("TC-16b 失效或应用专用渠道回退选择，余额偏好不自动扣款", async ({
  consolePage: page,
}) => {
  let charges = 0;
  await page.route("**/account/payment-channels*", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          channels: [
            {
              channelNo: "APP",
              displayName: "应用支付",
              modes: ["app"],
              currencyCodes: ["CNY"],
            },
          ],
        },
      },
    }),
  );
  await page.route("**/balance-payments", (r) => {
    charges++;
    return r.fulfill({ status: 503, json: { code: 503 } });
  });
  const view = new CustomerPage(page);
  await view.open(
    "/account/billing/invoices/INV-TEST-001?pay=1&paymentChannel=APP&useBalance=1",
  );
  await expect(page.getByRole("dialog")).toContainText(
    "已优先选择账户余额，请确认后付款",
  );
  await expect(
    page.getByRole("button", { name: "前往支付", exact: true }),
  ).toBeDisabled();
  await expect(page.getByRole("dialog")).toContainText("暂未开通在线支付方式");
  expect(charges).toBe(0);
  await expect(page).toHaveURL(/\?pay=1&useBalance=1$/);
  await view.screenshot("website-balance-preference");
  await page.getByRole("button", { name: "使用余额付款", exact: true }).click();
  await expect(
    page.getByText("将从当前币种的可用余额扣除 CNY 128.00。"),
  ).toBeVisible();
  expect(charges).toBe(0);
  await view.screenshot("website-balance-explicit-confirm");
});

test("TC-16c 不可用余额及失效渠道参数被清理，不能变成付款授权", async ({
  consolePage: page,
}) => {
  let writes = 0;
  await page.route("**/api/v1/account/billing/invoices/INV-TEST-001*", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          detail: {
            invoice: { ...invoice, canPayFromBalance: false },
            items: [],
            nextItemCursor: "",
          },
        },
      },
    }),
  );
  await page.route("**/account/payment-channels*", (r) =>
    r.fulfill({ json: { code: 0, data: { channels: [] } } }),
  );
  for (const path of [
    "**/balance-payments",
    "**/account/payments",
    "**/payment-preparations",
  ])
    await page.route(path, (r) => {
      writes++;
      return r.fulfill({ status: 503, json: { code: 503 } });
    });
  const view = new CustomerPage(page);
  await view.open(
    "/account/billing/invoices/INV-TEST-001?pay=1&paymentChannel=REMOVED&useBalance=1",
  );
  await expect(page.getByRole("dialog")).toContainText("暂未开通在线支付方式");
  await expect(
    page.getByRole("button", { name: "使用余额付款", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "前往支付", exact: true }),
  ).toBeDisabled();
  await expect(page).toHaveURL(/\?pay=1$/);
  expect(writes).toBe(0);
  await view.screenshot("website-invalid-payment-preference");
});
