// 专用本地数据库中的已付款订单准备，禁止用于主环境或真实支付。
import { createHash, randomUUID } from "node:crypto";
import type { APIRequestContext } from "@host-tests/support/playwright";
import { expect, request } from "@host-tests/support/playwright";
import { get } from "./isolated-commerce";
import type { Fixture } from "./isolated-commerce";
async function write(
  client: APIRequestContext,
  path: string,
  data: object,
  method = "POST",
) {
  const response = await client.fetch(path, { method, data, maxRedirects: 0 });
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(body.code, body.message).toBe(0);
  return body.data;
}
export async function createCustomer(
  f: Fixture,
  admin: APIRequestContext,
  key: string,
) {
  const channels = await get(
    admin,
    "/x/idc-resource/api/v1/admin/resource/provider-configurations",
  );
  expect(channels.channels.length).toBeGreaterThan(0);
  for (const channel of channels.channels)
    expect(channel.endpoint).toBe(f.mockOrigin + "/administrator");
  const payments = await get(
    admin,
    "/x/idc-payment/api/v1/admin/payment-channels",
  );
  expect(payments.channels).toHaveLength(1);
  expect(payments.channels[0]).toMatchObject({
    channelNo: "EPAY_NIGHTLY",
    endpointOrigin: f.mockOrigin + "/submit.php",
  });
  const password = randomUUID() + "aA1!";
  const email = key + "@example.test";
  const account = await write(
    admin,
    "/x/idc-core/api/v1/admin/customers/direct-accounts",
    {
      customerType: "individual",
      displayName: "隔离开通异常验收",
      countryCode: "CN",
      preferredLocale: "zh-CN",
      email,
      initialPassword: password,
      reason: "独立数据库与本地模拟上游，不涉及真实客户或资金",
      idempotencyKey: key + "-customer",
    },
  );
  const detail = await get(
    admin,
    "/x/idc-core/api/v1/admin/customers/" + account.customerNo,
  );
  await write(
    admin,
    "/x/idc-core/api/v1/admin/customers/" +
      account.customerNo +
      "/verification",
    {
      action: "approve",
      expectedVersion: detail.customer.version,
      idempotencyKey: key + "-verified",
    },
    "PUT",
  );
  const anon = await request.newContext({ baseURL: f.baseURL });
  try {
    const auth = await write(anon, "/api/v1/auth/login", {
      username: email,
      password,
      clientType: "web",
    });
    return request.newContext({
      baseURL: f.baseURL,
      extraHTTPHeaders: {
        Authorization: "Bearer " + auth.accessToken,
        "Accept-Language": "zh-CN",
      },
    });
  } finally {
    await anon.dispose();
  }
}
export async function paidOrder(
  f: Fixture,
  client: APIRequestContext,
  key: string,
  publicSlug = "nightly-fixed-cloud",
  options: {
    fieldValues?: Record<string, string>;
    expectedTotal?: string;
    imageLabelEnUS?: string;
  } = {},
) {
  const base = "/x/idc-core/api/v1/";
  let config: any;
  await expect
    .poll(
      async () => {
        config = await get(
          client,
          base +
            `public/catalog/products/${encodeURIComponent(publicSlug)}/purchase-configuration`,
        );
        return (
          config.fields?.some(
            (row: any) =>
              row.labelZhCN === "操作系统" &&
              row.options?.some(
                (option: any) =>
                  option.labelEnUS ===
                  (options.imageLabelEnUS || "Debian13-x86"),
              ),
          ) ?? false
        );
      },
      { timeout: 45000, intervals: [250, 1000] },
    )
    .toBe(true);
  const field = config.fields.find((row: any) => row.labelZhCN === "操作系统");
  const option = field.options.find(
    (row: any) => row.labelEnUS === (options.imageLabelEnUS || "Debian13-x86"),
  );
  const defaults = (config.values || []).filter(
    (value: any) => value.fieldToken !== field.fieldToken,
  );
  for (const current of config.fields) {
    const supplied = options.fieldValues?.[current.labelZhCN];
    const existing = defaults.findIndex(
      (value: any) => value.fieldToken === current.fieldToken,
    );
    if (
      current.fieldToken === field.fieldToken ||
      !current.visible ||
      current.disabled ||
      (existing >= 0 && supplied === undefined)
    )
      continue;
    const value: any = { fieldToken: current.fieldToken, type: current.type };
    if (current.type === "integer")
      value.integerValue =
        supplied ?? current.numericChoices?.[0] ?? current.minimumInteger;
    else if (current.type === "single_select" && current.options?.[0])
      value.optionTokens = [current.options[0].optionToken];
    else if (current.type === "boolean") value.booleanValue = false;
    else continue;
    if (existing >= 0) defaults[existing] = value;
    else defaults.push(value);
  }
  const values = [
    ...defaults,
    {
      fieldToken: field.fieldToken,
      type: field.type,
      optionTokens: [option.optionToken],
    },
  ];
  const purchase = {
    publicSlug,
    configurationRevision: config.configurationRevision,
    values,
  };
  return paidConfiguredOrder(
    f,
    client,
    key,
    purchase,
    options.expectedTotal || "1100",
  );
}

// The payment and freezing path is shared by server and non-server products.
export async function paidConfiguredOrder(
  f: Fixture,
  client: APIRequestContext,
  key: string,
  purchase: {
    publicSlug: string;
    configurationRevision: string;
    values: any[];
  },
  expectedTotal: string,
) {
  const base = "/x/idc-core/api/v1/";
  const { publicSlug, configurationRevision, values } = purchase;
  const quote = await write(
    client,
    base +
      `public/catalog/products/${encodeURIComponent(publicSlug)}/purchase-configuration/quote`,
    {
      configurationRevision,
      currencyCode: "CNY",
      values,
    },
  );
  expect(quote.terms[0].totalAmount).toBe(expectedTotal);
  const originalCart = (
    await write(client, base + "account/commerce/carts", {
      currencyCode: "CNY",
    })
  ).cart;
  const cart = (
    await write(client, base + "account/commerce/cart-items", {
      expectedCartVersion: originalCart.version,
      quantity: 1,
      ...purchase,
      billingTermToken: quote.terms[0].billingTermToken,
      currencyCode: "CNY",
      purchaseCycles: 1,
    })
  ).cart;
  const frozen = (
    await write(client, base + "account/commerce/quotes", {
      expectedCartVersion: cart.version,
      assetNumbers: [],
    })
  ).quote;
  expect(frozen.totalAmountMinor).toBe(expectedTotal);
  for (const item of frozen.items) {
    if (item.configurationRequired && !item.configurationFrozen)
      await write(
        client,
        base +
          "account/commerce/quote-items/" +
          item.quoteItemNo +
          "/purchase-configuration",
        purchase,
      );
  }
  const checkout = (
    await write(client, base + "account/commerce/orders", {
      quoteNo: frozen.quoteNo,
      idempotencyKey: key + "-order",
    })
  ).checkout;
  const prep = (
    await write(
      client,
      base +
        "account/commerce/orders/" +
        checkout.orderNo +
        "/payment-preparations",
      {},
    )
  ).preparation;
  expect(prep.ready).toBe(true);
  const payment = await write(
    client,
    "/x/idc-payment/api/v1/account/payments",
    {
      paymentContextNo: prep.contextNo,
      channelNo: "EPAY_NIGHTLY",
      modeCode: "epay_cashier",
      balanceAmountMinor: "0",
    },
  );
  expect(payment.redirectUrl).toBe(f.mockOrigin + "/submit.php");
  const valuesToSign: Record<string, string> = {
    pid: "1108",
    trade_no: key,
    out_trade_no: payment.attemptNo,
    type: "alipay",
    name: "Isolated failed cloud",
    money:
      (BigInt(expectedTotal) / 100n).toString() +
      "." +
      (BigInt(expectedTotal) % 100n).toString().padStart(2, "0"),
    trade_status: "TRADE_SUCCESS",
    sign_type: "MD5",
  };
  const canonical = Object.keys(valuesToSign)
    .filter((name) => !["sign", "sign_type"].includes(name))
    .sort()
    .map((name) => name + "=" + valuesToSign[name])
    .join("&");
  valuesToSign.sign = createHash("md5")
    .update(canonical + "nightly-epay-secret")
    .digest("hex");
  const path = "/x/idc-payment/api/v1/callbacks/payments/EPAY_NIGHTLY";
  expect(new URL(payment.redirectFields.notify_url).pathname).toBe(path);
  const callback = await client.get(
    path + "?" + new URLSearchParams(valuesToSign),
    { maxRedirects: 0 },
  );
  expect((await callback.text()).trim()).toBe("success");
  return { orderNo: checkout.orderNo, payment };
}
