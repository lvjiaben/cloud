import { test, expect, now } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
const server = {
  serverNo: "SVR-config",
  entitlementNo: "ENT_config",
  productNameZhCn: "应用服务器",
  runtimeState: "stopped",
  entitlementStatus: "active",
  addressesConfirmed: true,
  addresses: [],
  orderNo: "ORD-TEST-001",
  periodUnit: "month",
  periodCount: 1,
  startedAt: now,
  availableActions: [],
  commercialCapabilities: [
    { action: "resize", supported: true, canQuote: true, reasonCode: "" },
  ],
  operations: [],
  credentialAvailable: false,
};
const context = {
  entitlementNo: "ENT_config",
  configurationRevision: "rev-own",
  currencyCode: "CNY",
  purchaseCycles: 1,
  ready: true,
  billingTermToken: "term-current",
  fields: [
    {
      fieldToken: "f_cpu",
      type: "integer",
      labelZhCN: "处理器",
      labelEnUS: "CPU",
      descriptionZhCN: "选择处理器核数",
      visible: true,
      required: true,
      disabled: false,
      unit: "核",
      numericChoices: ["2", "4"],
    },
    {
      fieldToken: "f_memory",
      type: "integer",
      labelZhCN: "内存",
      labelEnUS: "Memory",
      descriptionZhCN: "",
      visible: true,
      required: true,
      disabled: false,
      unit: "MiB",
    },
  ],
  values: [
    { fieldToken: "f_cpu", type: "integer", integerValue: "2" },
    { fieldToken: "f_memory", type: "integer", integerValue: "8192" },
  ],
  issues: [],
};
test.describe("TC-22 服务器动态变配", () => {
  test("TC-22a 完整typed配置重新求值，冻结专用目标后再计算差价，失败不发购买", async ({
    consolePage: page,
  }) => {
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-config",
      (r) => r.fulfill({ json: { code: 0, data: server } }),
    );
    await page.route("**/native-servers/SVR-config/management-options", (r) =>
      r.fulfill({ json: { code: 0, data: { images: [], accessories: [] } } }),
    );
    const evaluated: any[] = [];
    let freeze: any,
      quoted: any,
      purchases = 0;
    await page.route(
      "**/native-servers/SVR-config/configuration-context*",
      (r) => {
        const url = new URL(r.request().url());
        let value = context;
        if (url.searchParams.has("values")) {
          expect(url.searchParams.get("configurationRevision")).toBe("rev-own");
          const values = JSON.parse(url.searchParams.get("values")!);
          evaluated.push(values);
          value = { ...context, values };
        }
        return r.fulfill({ json: { code: 0, data: value } });
      },
    );
    await page.route("**/native-servers/SVR-config/target-quotes", (r) => {
      freeze = r.request().postDataJSON();
      return r.fulfill({ json: { code: 0, data: { quoteNo: "QTE_target" } } });
    });
    await page.route("**/native-servers/SVR-config/commercial-quotes", (r) => {
      quoted = r.request().postDataJSON();
      return r.fulfill({
        json: {
          code: 0,
          data: {
            quoteNo: "nqt_resize",
            action: "resize",
            currencyCode: "CNY",
            totalAmountMinor: "2000",
            taxAmountMinor: "0",
            expiresAt: Date.now() + 60000,
            currentRecurringMinor: "8000",
            targetRecurringMinor: "12000",
            direction: "upgrade",
            periodUnit: "month",
            periodCount: 1,
          },
        },
      });
    });
    await page.route(
      "**/native-servers/SVR-config/commercial-confirmations",
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: {
              required: true,
              proofToken: "resize-proof",
              expiresAt: Date.now() + 60000,
            },
          },
        }),
    );
    await page.route(
      "**/native-servers/SVR-config/commercial-purchases",
      (r) => {
        purchases++;
        return r.fulfill({
          status: 503,
          json: { code: 503, message: "提交暂时失败，请重试" },
        });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-config");
    await view.button("调整配置").click();
    let dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("选择处理器核数");
    await expect(
      dialog.getByRole("button", { name: "获取变更报价", exact: true }),
    ).toBeDisabled();
    await dialog.locator(".t-select input").click();
    await page
      .locator(".t-select-option")
      .getByText("4 核", { exact: true })
      .click();
    await expect.poll(() => evaluated.length).toBe(1);
    expect(evaluated[0]).toEqual(
      expect.arrayContaining([
        { fieldToken: "f_cpu", type: "integer", integerValue: "4" },
        { fieldToken: "f_memory", type: "integer", integerValue: "8192" },
      ]),
    );
    await dialog
      .getByRole("button", { name: "获取变更报价", exact: true })
      .click();
    expect(freeze).toMatchObject({
      configurationRevision: "rev-own",
      billingTermToken: "term-current",
      values: evaluated[0],
    });
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "获取报价", exact: true })
      .click();
    dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("CNY 20.00");
    await expect(dialog).toContainText("CNY 120.00");
    expect(quoted).toMatchObject({
      action: "resize",
      targetQuoteNo: "QTE_target",
      parameters: {},
    });
    expect(quoted).not.toHaveProperty("amountMinor");
    expect(quoted).not.toHaveProperty("values");
    await view.screenshot("server-resize-price-confirm");
    await dialog
      .getByRole("button", { name: "确认价格并继续", exact: true })
      .click();
    expect(purchases).toBe(0);
    await dialog
      .getByRole("button", { name: "确认操作并提交", exact: true })
      .click();
    await expect(dialog).toContainText("提交暂时失败，请重试");
    expect(purchases).toBe(1);
  });
  test("TC-22b 配置读取失败提供重试，不展示假规格或价格", async ({
    consolePage: page,
  }) => {
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-config",
      (r) => r.fulfill({ json: { code: 0, data: server } }),
    );
    await page.route("**/native-servers/SVR-config/management-options", (r) =>
      r.fulfill({ json: { code: 0, data: { images: [], accessories: [] } } }),
    );
    await page.route(
      "**/native-servers/SVR-config/configuration-context*",
      (r) =>
        r.fulfill({
          status: 503,
          json: { code: 503, message: "配置暂时不可用，请重试" },
        }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-config");
    await view.button("调整配置").click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toContainText("配置暂时不可用，请重试");
    await expect(
      dialog.getByRole("button", { name: "获取变更报价", exact: true }),
    ).toHaveCount(0);
    await view.screenshot("server-resize-unavailable");
  });
});
