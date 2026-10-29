// Browser integration uses the explicit local fake-commerce database only.
// Shared core guards verify private fixture paths and reject live app ports.
import type { APIRequestContext } from "@host-tests/support/playwright";
import { expect, request } from "@host-tests/support/playwright";
import {
  fixture,
  get,
} from "../../../../idc-core/hack/tests/support/isolated-commerce";
import type { Fixture } from "../../../../idc-core/hack/tests/support/isolated-commerce";
export {
  api,
  descriptor,
  get,
  saved,
  serversPath,
} from "../../../../idc-core/hack/tests/support/isolated-commerce";
export interface WebFixture extends Fixture {
  websiteOrigin: string;
  customerOrigin: string;
  rendererOrigin: string;
}
export async function webFixture() {
  const f = (await fixture()) as WebFixture;
  const origins = [f.websiteOrigin, f.customerOrigin, f.rendererOrigin];
  expect(new Set(origins).size).toBe(3);
  for (const origin of origins) {
    const url = new URL(origin);
    expect(url.protocol).toBe("http:");
    expect(url.hostname).toBe("127.0.0.1");
    expect(["6030", "6031", "6032", "6680"]).not.toContain(url.port);
    expect(url.origin).toBe(origin);
  }
  return f;
}
export async function write(
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
export async function customer(
  f: WebFixture,
  admin: APIRequestContext,
  key: string,
  password: string,
) {
  const email = key + "@example.test";
  const account = await write(
    admin,
    "/x/idc-core/api/v1/admin/customers/direct-accounts",
    {
      customerType: "individual",
      displayName: "独立官网客户验收",
      countryCode: "CN",
      preferredLocale: "zh-CN",
      email,
      initialPassword: password,
      reason: "独立测试库与本地模拟上游，浏览器完整交易验收",
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
      idempotencyKey: key + "-approved",
    },
    "PUT",
  );
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
  return { email, password, customerNo: account.customerNo };
}
export async function mockSuccess(f: WebFixture) {
  const client = await request.newContext({
    baseURL: f.mockOrigin,
    ignoreHTTPSErrors: true,
  });
  try {
    for (const [path, data] of [
      ["/fixture/mode", { mode: "success" }],
      ["/fixture/task-delay", { seconds: 0 }],
    ] as const) {
      const response = await client.post(path, { data });
      expect(response.status()).toBe(200);
    }
  } finally {
    await client.dispose();
  }
}

// 每次在专用测试库复制独立商品，绝不修改基准套餐或真实商品。
export async function elasticProduct(admin: APIRequestContext, key: string) {
  const root = "/x/idc-core/api/v1/admin/catalog";
  const source = await get(
    admin,
    root + "/products/prd_8f2bae468581a39554c360b37e7dbf0c",
  );
  expect(source.product.publicSlug).toBe("nightly-fixed-cloud");
  const slug = "nightly-elastic-" + key;
  const copied = await write(
    admin,
    root + "/products/" + source.product.productNo + "/copies",
    {
      sourceVersionNo: source.version.versionNo,
      publicSlug: slug,
      nameZhCn: "V1隔离弹性云服务器",
      nameEnUs: "V1 isolated elastic cloud server",
    },
  );
  const no = (copied.product || copied).productNo;
  const detail = () => get(admin, root + "/products/" + no);
  async function draft() {
    let d = await detail();
    if (d.version.status !== "draft") {
      await write(admin, root + "/products/" + no + "/draft-versions", {
        sourceVersionNo: d.version.versionNo,
      });
      d = await detail();
    }
    return d;
  }
  let d = await draft();
  const configPath =
    root + "/product-versions/" + d.version.versionNo + "/configuration";
  const { configuration } = await get(admin, configPath);
  await write(
    admin,
    configPath,
    {
      expectedVersion: configuration.version,
      merchantValues: configuration.merchantValues,
      customerFieldSelections: [
        ...configuration.customerFieldSelections.filter(
          (field: { fieldKey: string }) => !["cpu", "memory_gib"].includes(field.fieldKey),
        ),
        {
          fieldKey: "cpu",
          displayControl: "input",
          numericPricing: "quantity",
          minimum: "1",
          maximum: "8",
          quantityStep: "1",
        },
        {
          fieldKey: "memory_gib",
          displayControl: "slider",
          numericPricing: "quantity",
          minimum: "1",
          maximum: "4",
          quantityStep: "1",
        },
      ],
    },
    "PUT",
  );
  d = await draft();
  await write(
    admin,
    root + "/product-versions/" + d.version.versionNo + "/inventory",
    {
      expectedVersion: d.inventory.version,
      trackingEnabled: true,
      sellableQuantity: 1000,
    },
    "PUT",
  );
  for (const [label, amount] of [
    ["处理器核心数", "100"],
    ["内存（GiB）", "50"],
  ]) {
    d = await draft();
    let versionPath = root + "/product-versions/" + d.version.versionNo;
    const { candidates } = await get(
      admin,
      versionPath + "/configuration-price-candidates",
    );
    const candidate = candidates.find(
      (row: any) => row.quantity && row.fieldLabelZhCN === label,
    );
    expect(candidate).toBeTruthy();
    const dimension = await write(
      admin,
      versionPath + "/configuration-price-dimensions",
      {
        candidateToken: candidate.candidateToken,
        pricingMode: "per_unit",
      },
    );
    d = await draft();
    versionPath = root + "/product-versions/" + d.version.versionNo;
    await write(
      admin,
      versionPath +
        "/billing-rules/" +
        d.billingRules[0].billingRuleNo +
        "/price-schedules",
      {
        configurationPriceDimensionNo: dimension.dimensionNo,
        currencyCode: "CNY",
        status: "active",
        tiers: [
          {
            minimumQuantity: 0,
            maximumQuantity: null,
            amountMinor: amount,
            setupFeeMinor: "0",
            renewalAmountMinor: amount,
          },
        ],
      },
    );
  }
  d = await detail();
  await write(
    admin,
    root + "/products/" + no + "/sale-state",
    {
      versionNo: d.product.currentVersionNo,
      expectedVersion: d.product.descriptionVersion,
      active: true,
    },
    "PUT",
  );
  return slug;
}
