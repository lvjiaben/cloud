import { test, expect, now } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
const server = {
  serverNo: "SVR-commercial",
  entitlementNo: "ENT_owned",
  productNameZhCn: "应用云服务器",
  runtimeState: "running",
  entitlementStatus: "active",
  addressesConfirmed: true,
  addresses: ["192.0.2.20"],
  orderNo: "ORD-TEST-001",
  periodUnit: "month",
  periodCount: 1,
  startedAt: now,
  expiresAt: now + 86400000,
  availableActions: [],
  commercialCapabilities: [
    "snapshot_create",
    "backup_create",
    "traffic_add",
    "ip_change",
  ].map((action) => ({
    action,
    supported: true,
    canQuote: true,
    reasonCode: "",
  })),
  operations: [],
  credentialAvailable: false,
};
const options = {
  images: [],
  accessories: [
    {
      objectNo: "ACO_disk",
      kind: "disk",
      name: "系统盘",
      state: "available",
      projection: {},
    },
  ],
  accessoriesExpiresAt: Date.now() + 600000,
};
test.describe("TC-20 服务器购买流程", () => {
  test("TC-20a 快照真实报价显示最终含税周期，提交失败保持同一请求，付费后才执行", async ({
    consolePage: page,
  }) => {
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial",
      (r) => r.fulfill({ json: { code: 0, data: server } }),
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial/management-options",
      (r) => r.fulfill({ json: { code: 0, data: options } }),
    );
    const quote = {
      quoteNo: "nqt_owned",
      action: "snapshot_create",
      currencyCode: "CNY",
      totalAmountMinor: "9007199254740993",
      taxAmountMinor: "3",
      expiresAt: Date.now() + 60000,
      freeCountPerServer: 1,
      existingObjectCount: 1,
      chargedQuantity: 1,
      periodUnit: "month",
      periodCount: 1,
    };
    let quoteRequest: any,
      confirms = 0,
      fail = true;
    const purchases: any[] = [];
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial/commercial-quotes",
      (r) => {
        quoteRequest = r.request().postDataJSON();
        return r.fulfill({ json: { code: 0, data: quote } });
      },
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial/commercial-confirmations",
      (r) => {
        confirms++;
        expect(r.request().postDataJSON().quoteNo).toBe("nqt_owned");
        return r.fulfill({
          json: { code: 0, data: { required: false, proofToken: "" } },
        });
      },
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial/commercial-purchases",
      (r) => {
        purchases.push(r.request().postDataJSON());
        return fail
          ? r.fulfill({
              status: 503,
              json: { code: 503, message: "提交暂时失败，请重试" },
            })
          : r.fulfill({
              json: {
                code: 0,
                data: {
                  operationNo: "BOP_owned",
                  quoteNo: "nqt_owned",
                  orderNo: "ORD-TEST-001",
                  invoiceNo: "INV-TEST-001",
                  status: "awaiting_financial_settlement",
                  currencyCode: "CNY",
                  totalAmountMinor: quote.totalAmountMinor,
                  lastErrorCode: "",
                },
              },
            });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-commercial");
    await page.getByRole("tab", { name: "快照与备份", exact: true }).click();
    await view.button("创建快照").click();
    const dialog = page.getByRole("dialog");
    await dialog.getByRole("button", { name: "获取报价", exact: true }).click();
    await expect(dialog).toContainText("请选择磁盘并填写名称");
    expect(quoteRequest).toBeUndefined();
    await dialog.getByPlaceholder("选择要保存的磁盘").click();
    await page
      .locator(".t-select-option")
      .getByText("系统盘", { exact: true })
      .click();
    await dialog.getByPlaceholder("例如：系统更新前").fill("更新前");
    await dialog.getByRole("button", { name: "获取报价", exact: true }).click();
    await expect(dialog).toContainText("CNY 90,071,992,547,409.93");
    await expect(dialog).toContainText("1 个月");
    expect(quoteRequest.parameters).toEqual({
      diskObjectNo: "ACO_disk",
      name: "更新前",
    });
    expect(quoteRequest).not.toHaveProperty("amountMinor");
    await view.screenshot("snapshot-authoritative-quote");
    await page.setViewportSize({ width: 390, height: 844 });
    await expect
      .poll(() =>
        page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      )
      .toBe(true);
    await view.screenshot("snapshot-authoritative-quote-mobile");
    await page.setViewportSize({ width: 1440, height: 960 });
    await dialog
      .getByRole("button", { name: "确认价格并继续", exact: true })
      .click();
    await expect(dialog).toContainText("提交暂时失败，请重试");
    fail = false;
    await dialog
      .getByRole("button", { name: "重试本次提交", exact: true })
      .click();
    await expect(dialog).toContainText("付款成功后才会执行");
    expect(purchases).toHaveLength(2);
    expect(purchases[1]).toEqual(purchases[0]);
    expect(confirms).toBe(1);
    await expect(
      dialog.getByRole("link", { name: "查看并支付账单", exact: true }),
    ).toHaveAttribute("href", "/account/billing/invoices/INV-TEST-001");
    await view.screenshot("snapshot-purchase-await-payment");
  });
  test("TC-20b 流量套餐按商家固定容量选购，库存不足不可选，报价过期不提交", async ({
    consolePage: page,
  }) => {
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial",
      (r) => r.fulfill({ json: { code: 0, data: server } }),
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial/management-options",
      (r) => r.fulfill({ json: { code: 0, data: options } }),
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial/traffic-packages",
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: {
              items: [
                {
                  packageNo: "pkg100",
                  nameZhCn: "临时流量",
                  trafficGb: 100,
                  amountMinor: "1000",
                  currencyCode: "CNY",
                  unlimitedStock: true,
                },
                {
                  packageNo: "pkg200",
                  nameZhCn: "售罄套餐",
                  trafficGb: 200,
                  amountMinor: "1500",
                  currencyCode: "CNY",
                  unlimitedStock: false,
                  remainingStock: 0,
                },
              ],
            },
          },
        }),
    );
    let request: any,
      confirmations = 0;
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial/commercial-quotes",
      (r) => {
        request = r.request().postDataJSON();
        return r.fulfill({
          json: {
            code: 0,
            data: {
              quoteNo: "nqt_traffic",
              action: "traffic_add",
              currencyCode: "CNY",
              totalAmountMinor: "1000",
              taxAmountMinor: "0",
              expiresAt: Date.now() + 1000,
              trafficGb: 100,
              periodUnit: "",
              periodCount: 0,
            },
          },
        });
      },
    );
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-commercial/commercial-confirmations",
      (r) => {
        confirmations++;
        return r.fulfill({ status: 500 });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-commercial");
    await page.getByRole("tab", { name: "网络与安全", exact: true }).click();
    await view.button("购买流量包").click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("暂未提供具体到期时间");
    await dialog.getByPlaceholder("选择流量套餐").click();
    await expect(
      page.locator(".t-select-option").filter({ hasText: "售罄套餐" }),
    ).toHaveClass(/t-is-disabled/);
    await page
      .getByText("临时流量 · 100 GB · CNY 10.00", { exact: true })
      .click();
    await dialog.getByRole("button", { name: "获取报价", exact: true }).click();
    await expect(dialog).toContainText("100 GB");
    expect(request).toMatchObject({
      action: "traffic_add",
      packageNo: "pkg100",
      parameters: {},
    });
    await expect(dialog).toContainText("不会自动开机或解除暂停");
    await page.waitForTimeout(1100);
    await dialog
      .getByRole("button", { name: "确认价格并继续", exact: true })
      .click();
    await expect(dialog).toContainText("报价已过期");
    expect(confirmations).toBe(0);
    await view.screenshot("traffic-quote-expired");
  });
});
