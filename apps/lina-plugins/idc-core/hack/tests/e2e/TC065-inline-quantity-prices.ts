// Current merchant offer editor: prices belong to one configuration save.
// These tests exercise retries, authority, external changes and exact amounts.
import type { Page } from "@host-tests/support/playwright";
import { test, expect } from "../support/merchant-auth";
import {
  installCatalogManagementMock,
  mockProductDetail,
} from "../support/catalog-mock";
import { installCatalogAutomationMock } from "../support/catalog-automation-mock";
import { MerchantOfferPage } from "../pages/MerchantOfferPage";

async function fixture(
  page: Page,
  options: { create?: boolean; inactive?: boolean; once?: boolean } = {},
) {
  await installCatalogManagementMock(page);
  const detail: any = mockProductDetail("product-manual-service");
  detail.version.status = "draft";
  detail.fulfillmentBinding = {
    interfaceInstanceNo: "quantity-fixture",
    interfaceDisplayName: "只读测试接口",
    evidenceVerified: true,
    interfaceNewBusinessState: "accepting",
  };
  detail.fulfillmentPolicy = { strategyCode: "manual" };
  detail.billingRules = [
    {
      billingRuleNo: "billing-quantity",
      ruleCode: "monthly",
      chargeType: options.once ? "one_time" : "recurring",
      periodUnit: options.once ? "once" : "month",
      periodCount: 1,
      quantityMode: "fixed",
      pricePhase: options.once ? "new_purchase" : "both",
      status: "active",
      minimumPurchaseCycles: 1,
    },
  ];
  const base: any = {
    priceNo: "base",
    billingRuleNo: "billing-quantity",
    currencyCode: "CNY",
    amountMinor: "100",
    setupFeeMinor: "1000",
    renewalAmountMinor: options.once ? null : "100",
    minimumQuantity: 0,
    maximumQuantity: null,
    configurationPriceDimensionNo: "",
    status: "active",
  };
  detail.prices = [
    base,
    ...(options.create
      ? []
      : [
          {
            ...base,
            priceNo: "quantity-cpu",
            configurationPriceDimensionNo: "dimension-cpu",
            amountMinor: "500",
            status: options.inactive ? "inactive" : "active",
          },
        ]),
  ];
  const field = {
    key: "cpu",
    type: "integer",
    label: { zhCN: "处理器核心数", enUS: "CPU cores" },
    minimum: "1",
    maximum: "128",
  };
  const configuration: any = {
    version: 1,
    merchantValues: [],
    productDeclaration: { version: "v1", fields: [] },
    customerDeclaration: { version: "v1", fields: [field] },
    customerConfigurableFields: [field],
    customerFieldSelections: [
      {
        fieldKey: "cpu",
        displayControl: "input",
        numericPricing: "quantity",
        minimum: "1",
        maximum: "128",
      },
    ],
    productResolvedCandidates: {},
    customerResolvedCandidates: {},
    merchantPriceTargets: options.create
      ? []
      : [
          {
            fieldKey: "cpu",
            candidateValue: "",
            quantity: true,
            dimensionNo: "dimension-cpu",
            pricingMode: "per_unit",
          },
        ],
  };
  const writes: any[] = [];
  let failures = 0,
    refreshFailures = 0,
    successes = 0,
    delay = 0,
    candidateReads = 0;
  await page.route(
    "**/admin/catalog/products/product-manual-service",
    (route) => route.fulfill({ json: { code: 0, data: detail } }),
  );
  await page.route(
    "**/admin/catalog/product-versions/*/configuration",
    async (route) => {
      if (route.request().method() === "GET") {
        if (successes && refreshFailures) {
          refreshFailures--;
          return route.fulfill({
            status: 503,
            json: { code: 50, message: "暂时无法读取最新配置" },
          });
        }
        return route.fulfill({ json: { code: 0, data: { configuration } } });
      }
      const input = route.request().postDataJSON();
      writes.push(input);
      if (delay) await new Promise((resolve) => setTimeout(resolve, delay));
      if (failures) {
        failures--;
        return route.fulfill({
          status: 409,
          json: {
            code: 52,
            errorCode: "IDC_CATALOG_MANAGEMENT_CONFLICT",
            message: "价格已变，保留输入后重试",
          },
        });
      }
      successes++;
      configuration.version++;
      configuration.customerFieldSelections = input.customerFieldSelections;
      for (const [index, price] of (input.priceRows || []).entries()) {
        const dimensionNo = "dimension-" + price.fieldKey;
        configuration.merchantPriceTargets =
          configuration.merchantPriceTargets.filter(
            (target: any) => target.fieldKey !== price.fieldKey,
          );
        configuration.merchantPriceTargets.push({
          fieldKey: price.fieldKey,
          candidateValue: price.candidateValue,
          quantity: price.quantity,
          dimensionNo,
          pricingMode: price.pricingMode,
        });
        detail.prices = detail.prices.filter(
          (row: any) => row.configurationPriceDimensionNo !== dimensionNo,
        );
        detail.prices.push(
          ...price.tiers.map((tier: any, tierIndex: number) => ({
            ...base,
            ...tier,
            priceNo: `saved-${successes}-${index}-${tierIndex}`,
            billingRuleNo: price.billingRuleNo,
            currencyCode: price.currencyCode,
            status: price.active ? "active" : "inactive",
            renewalAmountMinor: tier.renewalAmountMinor ?? null,
            configurationPriceDimensionNo: dimensionNo,
          })),
        );
      }
      return route.fulfill({ json: { code: 0, data: { configuration } } });
    },
  );
  await page.route("**/configuration-price-candidates", (route) => {
    candidateReads++;
    return route.abort();
  });
  await page.route("**/fulfillment-policy", (route) =>
    route.fulfill({
      json: { code: 0, data: { policy: { strategyCode: "manual" } } },
    }),
  );
  const view = new MerchantOfferPage(page);
  return {
    view,
    detail,
    configuration,
    writes,
    candidateReads: () => candidateReads,
    failWrite: () => failures++,
    failRefresh: () => refreshFailures++,
    delayWrite: () => (delay = 500),
    changeStoredPrice() {
      const price = detail.prices.find(
        (row: any) => row.configurationPriceDimensionNo,
      );
      price.priceNo = "external-price";
      price.amountMinor = "9900";
    },
  };
}
async function open(view: MerchantOfferPage) {
  await view.goto();
  await view.openProductDetail("product-manual-service");
  await view.selectEditSection("购买配置");
}
const priceArea = (page: Page) =>
  page.getByTestId("catalog-offer-field-prices");
const amount = (page: Page, index = 0) =>
  priceArea(page).getByTestId("catalog-offer-price-amount").nth(index);
const renewal = (page: Page, index = 0) =>
  priceArea(page).getByTestId("catalog-offer-price-renewal").nth(index);
const save = (page: Page) => page.getByTestId("catalog-customer-options-save");
async function confirmed(page: Page) {
  await expect(save(page)).toBeEnabled();
  await expect(save(page)).not.toHaveClass(/ant-btn-loading/);
}

test.describe("TC-65 配置及价格统一保存", () => {
  test("TC-65a: 已有价格回填，失败保留输入，双击只提交一次，重试不新建重复收费项", async ({
    adminPage,
  }) => {
    const f = await fixture(adminPage);
    await open(f.view);
    await expect(amount(adminPage)).toHaveValue("5.00");
    await expect(
      priceArea(adminPage).getByTestId("catalog-offer-price-setup"),
    ).toHaveValue("10.00");
    await amount(adminPage).fill("6.25");
    f.failWrite();
    f.delayWrite();
    await save(adminPage).dblclick();
    await expect.poll(() => f.writes.length).toBe(1);
    await confirmed(adminPage);
    await expect(amount(adminPage)).toHaveValue("6.25");
    await f.view.screenshot("atomic-price-failed-keeps-draft");
    await save(adminPage).click();
    await expect.poll(() => f.writes.length).toBe(2);
    await confirmed(adminPage);
    expect(f.writes.map((write) => write.priceRows[0].previousPriceNo)).toEqual(
      ["quantity-cpu", "quantity-cpu"],
    );
    expect(f.writes[1].priceRows[0].tiers[0]).toMatchObject({
      amountMinor: "625",
      setupFeeMinor: "1000",
      renewalAmountMinor: "100",
    });
    await expect(amount(adminPage)).toHaveValue("6.25");
    expect(f.candidateReads()).toBe(0);
    await f.view.screenshot("atomic-price-confirmed");
  });
  test("TC-65b: 新数量阶梯与配置一次保存，金额超精度不提交，重试保留全部档位", async ({
    adminPage,
  }) => {
    const f = await fixture(adminPage, { create: true });
    await open(f.view);
    const mode = priceArea(adminPage).getByRole("combobox", {
      name: "计价方式",
      exact: true,
    });
    await mode.press("ArrowDown");
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("按区间整档收费", { exact: true })
      .click();
    await amount(adminPage).fill("1.001");
    await renewal(adminPage).fill("2");
    await save(adminPage).click();
    await expect(f.view.offers).toContainText("请填写有效的新购价");
    expect(f.writes).toHaveLength(0);
    await amount(adminPage).fill("2");
    await priceArea(adminPage)
      .getByTestId("catalog-offer-price-upper")
      .fill("100");
    await priceArea(adminPage)
      .getByRole("button", { name: "增加一档", exact: true })
      .click();
    await amount(adminPage, 1).fill("1");
    await renewal(adminPage, 1).fill("1");
    f.failWrite();
    await save(adminPage).click();
    await expect.poll(() => f.writes.length).toBe(1);
    await confirmed(adminPage);
    await expect(amount(adminPage, 1)).toHaveValue("1");
    await save(adminPage).click();
    await expect.poll(() => f.writes.length).toBe(2);
    await confirmed(adminPage);
    expect(f.writes[1].priceRows[0]).toMatchObject({
      pricingMode: "band_total",
      previousPriceNo: "",
      tiers: [
        {
          minimumQuantity: 0,
          maximumQuantity: 100,
          amountMinor: "200",
          renewalAmountMinor: "200",
        },
        {
          minimumQuantity: 100,
          maximumQuantity: null,
          amountMinor: "100",
          renewalAmountMinor: "100",
        },
      ],
    });
    await f.view.screenshot("atomic-price-quantity-bands");
  });
  test("TC-65c: 切换页签保留价格，外部改价阻止覆盖，明确重新读取后才清除草稿", async ({
    adminPage,
  }) => {
    const f = await fixture(adminPage);
    await open(f.view);
    await amount(adminPage).fill("8.50");
    await f.view.selectEditSection("基本资料");
    await f.view.selectEditSection("购买配置");
    await expect(amount(adminPage)).toHaveValue("8.50");
    await f.view.openFulfillmentPolicy();
    f.changeStoredPrice();
    await f.view.fulfillmentPolicyConfirm().click();
    await f.view.selectEditSection("购买配置");
    await expect(amount(adminPage)).toHaveValue("8.50");
    await expect(save(adminPage)).toBeDisabled();
    expect(f.writes).toHaveLength(0);
    await adminPage.getByTestId("catalog-customer-options-reload").click();
    await adminPage
      .locator(".ant-popover:visible")
      .getByRole("button", { name: /确\s*定/ })
      .click();
    await expect(amount(adminPage)).toHaveValue("99.00");
    await confirmed(adminPage);
    await amount(adminPage).fill("100");
    await save(adminPage).click();
    await expect.poll(() => f.writes.length).toBe(1);
    expect(f.writes[0].priceRows[0].previousPriceNo).toBe("external-price");
  });
  test("TC-65d: 保存成功但读取失败时不重复提交价格，可只重试读取恢复", async ({
    adminPage,
  }) => {
    const f = await fixture(adminPage);
    await open(f.view);
    await amount(adminPage).fill("7.50");
    f.failRefresh();
    await save(adminPage).click();
    await expect(
      adminPage.getByTestId("catalog-offer-saved-refresh-failed"),
    ).toContainText("已保存");
    await expect(save(adminPage)).toBeDisabled();
    expect(f.writes).toHaveLength(1);
    await adminPage.getByTestId("catalog-offer-saved-refresh-retry").click();
    await confirmed(adminPage);
    await expect(amount(adminPage)).toHaveValue("7.50");
    expect(f.writes).toHaveLength(1);
    await f.view.screenshot("atomic-price-read-recovery");
  });
  test("TC-65e: 修改停用价格仍保持停用，显式启用才生效", async ({
    adminPage,
  }) => {
    const f = await fixture(adminPage, { inactive: true });
    await open(f.view);
    const enabled = priceArea(adminPage).getByRole("switch", {
      name: "启用此价格",
      exact: true,
    });
    await expect(enabled).toHaveAttribute("aria-checked", "false");
    await amount(adminPage).fill("6");
    await save(adminPage).click();
    await expect.poll(() => f.writes.length).toBe(1);
    await confirmed(adminPage);
    expect(f.writes[0].priceRows[0].active).toBe(false);
    await enabled.click();
    await save(adminPage).click();
    await expect.poll(() => f.writes.length).toBe(2);
    expect(f.writes[1].priceRows[0].active).toBe(true);
  });
  test("TC-65f: 英文窄屏价格控件可读，水平滚动留在表格内", async ({
    adminPage,
    mainLayout,
  }) => {
    const f = await fixture(adminPage);
    await mainLayout.switchLanguage("English");
    await open(f.view);
    await expect(save(adminPage)).toHaveText("Save configuration and prices");
    await expect(priceArea(adminPage)).toContainText("Purchase price");
    await expect(priceArea(adminPage)).not.toContainText("plugin.idc-core.");
    await adminPage.setViewportSize({ width: 414, height: 896 });
    await save(adminPage).scrollIntoViewIfNeeded();
    await expect
      .poll(() =>
        f.view.detailDrawer.evaluate(
          (element) => element.scrollWidth <= element.clientWidth + 1,
        ),
      )
      .toBe(true);
    await f.view.screenshot("atomic-price-english-mobile");
  });
  test("TC-65g: 一次性周期不提交续费金额，保存保留原一次性费用", async ({
    adminPage,
  }) => {
    const f = await fixture(adminPage, { once: true });
    await open(f.view);
    await expect(renewal(adminPage)).toHaveCount(0);
    await amount(adminPage).fill("6.25");
    await save(adminPage).click();
    await expect.poll(() => f.writes.length).toBe(1);
    expect(f.writes[0].priceRows[0].tiers[0]).toMatchObject({
      amountMinor: "625",
      setupFeeMinor: "1000",
    });
    expect(
      f.writes[0].priceRows[0].tiers[0].renewalAmountMinor,
    ).toBeUndefined();
  });
  test("TC-65h: 本地价格立即显示，配置编辑不再请求独立价格候选", async ({
    adminPage,
  }) => {
    const f = await fixture(adminPage);
    await f.view.goto();
    const started = Date.now();
    await f.view.openProductDetail("product-manual-service");
    await f.view.selectEditSection("购买配置");
    await expect(amount(adminPage)).toHaveValue("5.00");
    expect(Date.now() - started).toBeLessThan(3000);
    expect(f.candidateReads()).toBe(0);
  });
  test("TC-65i: 缺少价格权限仍可维护配置，不读取或提交价格", async ({
    adminPage,
  }) => {
    const f = await fixture(adminPage);
    await installCatalogAutomationMock(adminPage, {
      permissions: [
        "idc:admin:catalog:product:list",
        "idc:admin:catalog:product:query",
        "idc:admin:catalog:product:update",
        "idc:admin:catalog:configuration:write",
      ],
    });
    await adminPage.reload();
    await open(f.view);
    await expect(priceArea(adminPage)).toHaveCount(0);
    await f.view.input("最小可售数量").fill("2");
    await save(adminPage).click();
    await expect.poll(() => f.writes.length).toBe(1);
    expect(f.writes[0].priceRows).toBeUndefined();
    expect(f.candidateReads()).toBe(0);
  });
  test("TC-65j: 大量通用子项仅渲染可见价格，滚动后保留编辑且可保存", async ({ adminPage }) => {
    const f = await fixture(adminPage, { create: true });
    const choices = Array.from({ length: 100 }, (_, index) => ({
      value: `choice-${index}`, label: { zhCN: `通用档位${index}`, enUS: `Profile ${index}` },
    }));
    const field = { key: "cpu", type: "single_select", label: { zhCN: "通用档位", enUS: "Generic profile" }, staticCandidates: choices };
    f.configuration.customerDeclaration.fields = [field];
    f.configuration.customerConfigurableFields = [field];
    f.configuration.customerFieldSelections = [{ fieldKey: "cpu", candidateValues: choices.map((choice) => choice.value) }];
    await open(f.view);
    await expect(amount(adminPage).first()).toBeVisible();
    expect(await priceArea(adminPage).getByTestId("catalog-offer-price-amount").count()).toBeLessThan(40);
    await amount(adminPage).first().fill("3");
    await renewal(adminPage).first().fill("3");
    await f.view.scrollPrices("end");
    await expect(priceArea(adminPage).getByText("通用档位99", { exact: true })).toBeVisible();
    expect(await priceArea(adminPage).getByTestId("catalog-offer-price-amount").count()).toBeLessThan(40);
    await f.view.scrollPrices("start");
    await expect(amount(adminPage).first()).toHaveValue("3");
    await save(adminPage).click();
    await expect.poll(() => f.writes.length).toBe(1);
    expect(f.writes[0].priceRows).toHaveLength(1);
    expect(f.writes[0].priceRows[0]).toMatchObject({ candidateValue: "choice-0", quantity: false, tiers: [{ amountMinor: "300", renewalAmountMinor: "300" }] });
    await f.view.screenshot("atomic-price-many-generic-choices");
  });
});
