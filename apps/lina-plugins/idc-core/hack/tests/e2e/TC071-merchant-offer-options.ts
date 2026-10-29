// Unified merchant option editing: typed fields and custom information share
// one editor and their own prices, with drafts retained across field changes.
import { test, expect } from "../support/merchant-auth";
import {
  installCatalogManagementMock,
  mockProductDetail,
} from "../support/catalog-mock";
import { MainLayout } from "@host-tests/pages/MainLayout";
import { MerchantOfferPage } from "../pages/MerchantOfferPage";

test.describe("TC-71 统一配置与价格编辑", () => {
  test("TC-71a: 选配、名称、范围与自定义字段同处维护，错误步长保留输入，保存后可继续编辑", async ({
    adminPage,
  }) => {
    test.setTimeout(60000);
    await installCatalogManagementMock(adminPage);
    const detail: any = mockProductDetail("product-manual-service");
    detail.version.status = "draft";
    detail.fulfillmentBinding = {
      interfaceInstanceNo: "offer-fixture",
      interfaceDisplayName: "只读测试上游",
      evidenceVerified: true,
      interfaceNewBusinessState: "accepting",
    };
    const fields = [
      {
        key: "cpu",
        type: "integer",
        label: { zhCN: "处理器核心数", enUS: "CPU cores" },
        minimum: "1",
        maximum: "128",
      },
      {
        key: "memory",
        type: "integer",
        label: { zhCN: "内存", enUS: "Memory" },
        minimum: "128",
        maximum: "65536",
      },
    ];
    const configuration: any = {
      version: 1,
      merchantValues: [],
      customerConfigurableFields: fields,
      customerFieldSelections: [],
      productDeclaration: { version: "v1", fields: [] },
      customerDeclaration: { version: "v1", fields: [] },
      productResolvedCandidates: {},
      customerResolvedCandidates: {},
    };
    const writes: any[] = [];
    await adminPage.route(
      "**/admin/catalog/products/product-manual-service",
      (r) => r.fulfill({ json: { code: 0, data: detail } }),
    );
    await adminPage.route(
      "**/admin/catalog/product-versions/*/configuration",
      (r) => {
        if (r.request().method() === "PUT") {
          const input = r.request().postDataJSON();
          writes.push(input);
          configuration.customerFieldSelections = input.customerFieldSelections;
          configuration.version++;
          configuration.customerDeclaration.fields =
            input.customerFieldSelections.map(
              (s: any) =>
                s.localField || {
                  ...fields.find((f) => f.key === s.fieldKey),
                  minimum: s.minimum,
                  maximum: s.maximum,
                  label: { zhCN: s.displayNameZhCN, enUS: s.displayNameEnUS },
                },
            );
        }
        return r.fulfill({ json: { code: 0, data: { configuration } } });
      },
    );
    await adminPage.route("**/configuration-price-candidates", (r) =>
      r.fulfill({ json: { code: 0, data: { candidates: [] } } }),
    );
    const view = new MerchantOfferPage(adminPage);
    await view.goto();
    await view.openProductDetail("product-manual-service");
    await view.selectEditSection("购买配置");
    await expect(
      adminPage.getByTestId("catalog-offer-custom-options"),
    ).toHaveCount(0);
    await expect(adminPage.getByTestId("catalog-package-defaults")).toHaveCount(
      0,
    );
    await expect(view.offers).toContainText("固定规格保留一个选项即可");
    await view.addField("处理器核心数");
    await view.input("最小可售数量").fill("2");
    await view.input("最大可售数量").fill("8");
    await view.input("数量步长").fill("2");
    await view.control("下拉选择");
    await view.setChildren("2\n4\n8");
    await view.addField("内存");
    await view.enable();
    await view.chooseField("cpu");
    await view.expectChildren("2\n4\n8");
    await view.setChildren("2\n3\n8");
    await view.save();
    await expect(view.offers).toContainText(
      "步长须为正整数，每个子项与最小值之差须为步长的整数倍",
    );
    expect(writes).toHaveLength(0);
    await view.screenshot("unified-option-invalid-step");
    await view.setChildren("2\n4\n8");
    await view.save();
    await expect.poll(() => writes.length).toBe(1);
    expect(writes[0].customerFieldSelections).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          fieldKey: "cpu",
          minimum: "2",
          maximum: "8",
          quantityStep: "2",
          displayControl: "select",
          numericChoices: ["2", "4", "8"],
          numericPricing: "quantity",
        }),
      ]),
    );
    await expect(
      adminPage.getByTestId("catalog-customer-options-save"),
    ).toBeEnabled();
    await expect(view.offers).not.toContainText("配置已被更新");
    await adminPage.getByTestId("catalog-local-option-add").click();
    await view.offers
      .getByRole("checkbox", { name: "分组、排序与多语言设置", exact: true })
      .check();
    await view.input("配置名称（中文）").fill("服务用途");
    await view.input("配置名称（英文）").fill("Service purpose");
    await view.setChildren("web|网站|Website\napp|应用|Application");
    await view.save();
    await expect.poll(() => writes.length).toBe(2);
    const custom = writes[1].customerFieldSelections.find(
      (s: any) => s.localField,
    );
    expect(custom).toMatchObject({
      displayNameZhCN: "服务用途",
      localField: {
        type: "single_select",
        staticCandidates: [
          { value: "web", label: { zhCN: "网站", enUS: "Website" } },
          { value: "app", label: { zhCN: "应用", enUS: "Application" } },
        ],
      },
    });
    expect(custom.productFieldKey).toBeUndefined();
    await expect(
      adminPage.getByTestId("catalog-customer-options-save"),
    ).toBeEnabled();
    await view.expectChildren("web|网站|Website\napp|应用|Application");
    await expect(view.field(custom.fieldKey)).toContainText("服务用途");
    await expect(
      adminPage.getByTestId("catalog-offer-field-prices"),
    ).toContainText("这个选项的附加费用");
    await expect(view.offers).not.toContainText("plugin.idc-core.");
    await view.screenshot("unified-custom-option-and-price");
  });
test("TC-71b: 配置重名时按字段标识分别保存价格，不按名称串到另一个配置", async ({
  adminPage,
}) => {
  test.setTimeout(60000);
  await installCatalogManagementMock(adminPage);
  const detail: any = mockProductDetail("product-manual-service");
  detail.version.status = "draft";
  detail.fulfillmentBinding = {
    interfaceInstanceNo: "same-label-interface",
    interfaceDisplayName: "只读测试上游",
    evidenceVerified: true,
    interfaceNewBusinessState: "accepting",
  };
  detail.billingRules = [
    {
      billingRuleNo: "billing-month",
      ruleCode: "monthly",
      periodUnit: "month",
      periodCount: 1,
      chargeType: "recurring",
      quantityMode: "fixed",
      pricePhase: "both",
      status: "active",
      minimumPurchaseCycles: 1,
    },
  ];
  detail.prices = [
    {
      priceNo: "base-price",
      billingRuleNo: "billing-month",
      currencyCode: "CNY",
      status: "active",
      amountMinor: "100",
      setupFeeMinor: "0",
      renewalAmountMinor: "100",
      minimumQuantity: 0,
      maximumQuantity: null,
      configurationPriceDimensionNo: "",
    },
  ];
  const fields = ["cpu", "memory"].map((key) => ({
    key,
    type: "integer",
    label: { zhCN: "规格", enUS: "Specification" },
    minimum: "1",
    maximum: "128",
  }));
  const configuration: any = {
    version: 1,
    merchantValues: [],
    productDeclaration: { version: "v1", fields: [] },
    customerDeclaration: { version: "v1", fields },
    customerConfigurableFields: fields,
    customerFieldSelections: fields.map((field) => ({
      fieldKey: field.key,
      minimum: "1",
      maximum: "128",
    })),
    productResolvedCandidates: {},
    customerResolvedCandidates: {},
    merchantPriceTargets: [],
  };
  const writes: any[] = [];
  await adminPage.route(
    "**/admin/catalog/products/product-manual-service",
    (r) => r.fulfill({ json: { code: 0, data: detail } }),
  );
  await adminPage.route(
    "**/admin/catalog/product-versions/*/configuration",
    (r) => {
      if (r.request().method() === "PUT") {
        const input = r.request().postDataJSON();
        writes.push(input);
        configuration.customerFieldSelections = input.customerFieldSelections;
        configuration.version++;
        for (const price of input.priceRows || []) {
          const dimensionNo = `dimension-${price.fieldKey}`;
          configuration.merchantPriceTargets = configuration.merchantPriceTargets.filter(
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
          detail.prices.push(...price.tiers.map((tier: any, index: number) => ({
            ...tier,
            priceNo: `saved-${configuration.version}-${price.fieldKey}-${index}`,
            billingRuleNo: price.billingRuleNo,
            currencyCode: price.currencyCode,
            status: price.active ? "active" : "inactive",
            configurationPriceDimensionNo: dimensionNo,
          })));
        }
      }
      return r.fulfill({ json: { code: 0, data: { configuration } } });
    },
  );
  const view = new MerchantOfferPage(adminPage);
  await view.goto();
  await view.openProductDetail("product-manual-service");
  await view.selectEditSection("购买配置");
  for (const [key, amount] of [
    ["cpu", "2.00"],
    ["memory", "3.00"],
  ]) {
    await view.chooseField(key!);
    const prices = adminPage.getByTestId("catalog-offer-field-prices");
    await prices.getByTestId("catalog-offer-price-amount").fill(amount!);
    await prices.getByTestId("catalog-offer-price-renewal").fill(amount!);
    expect(writes).toHaveLength(0);
  }
  await view.save();
  await expect.poll(() => writes.length).toBe(1);
  expect(
    writes[0].priceRows.map((row: any) => [
      row.fieldKey,
      row.quantity,
      row.tiers[0].amountMinor,
    ]),
  ).toEqual([
    ["cpu", true, "200"],
    ["memory", true, "300"],
  ]);
  await expect(
    adminPage.getByTestId("catalog-quantity-price-save"),
  ).toHaveCount(0);
  await expect(adminPage.getByTestId("catalog-customer-options-save")).toBeEnabled();
  await view.chooseField("cpu");
  const prices = adminPage.getByTestId("catalog-offer-field-prices");
  await expect(prices.getByTestId("catalog-offer-price-amount")).toHaveValue("2.00");
  await expect(prices.getByTestId("catalog-offer-price-renewal")).toHaveValue("2.00");
  await prices.getByRole("checkbox", { name: "启用新购价系数试算", exact: true }).check();
  await prices.getByTestId("catalog-offer-price-coefficient").fill("2");
  await prices.getByRole("button", { name: "应用试算", exact: true }).click();
  await expect(prices.getByTestId("catalog-offer-price-amount")).toHaveValue("4.00");
  await expect(prices.getByTestId("catalog-offer-price-renewal")).toHaveValue("2.00");
  await view.save();
  await expect.poll(() => writes.length).toBe(2);
  expect(writes[1].priceRows).toHaveLength(1);
  expect(writes[1].priceRows[0]).toMatchObject({
    fieldKey: "cpu",
    previousPriceNo: "saved-2-cpu-0",
    tiers: [{ amountMinor: "400", renewalAmountMinor: "200", setupFeeMinor: "0" }],
  });
  await expect(adminPage.getByTestId("catalog-customer-options-save")).toBeEnabled();
  await view.screenshot("same-name-options-price-identity");
});

test("TC-71c: 开通规则保留在同一工作区，保存规则不丢失尚未保存的购买规格", async ({
  adminPage,
}) => {
  test.setTimeout(60000);
  await installCatalogManagementMock(adminPage);
  const detail: any = mockProductDetail("product-manual-service");
  detail.version.status = "draft";
  detail.fulfillmentBinding = {
    interfaceInstanceNo: "rules-interface",
    interfaceDisplayName: "只读测试上游",
    evidenceVerified: true,
    interfaceNewBusinessState: "accepting",
  };
  const fields = [
    {
      key: "cpu",
      type: "integer",
      required: true,
      label: { zhCN: "处理器核心数", enUS: "CPU cores" },
      minimum: "1",
      maximum: "128",
    },
    {
      key: "hostname_prefix",
      type: "single_line",
      label: { zhCN: "主机名前缀", enUS: "Hostname prefix" },
      maximumLength: 10,
    },
    {
      key: "password_length",
      type: "integer",
      label: { zhCN: "自动生成密码长度", enUS: "Generated password length" },
      minimum: "6",
      maximum: "20",
    },
  ];
  fields.push(...["port", "system_disk_io_limit", "data_disk_io_limit"].map(key => ({ key, type: "single_line", label: { zhCN: key, enUS: key }, maximumLength: 100 })));
  const configuration: any = {
    version: 1,
    merchantValues: [
      { fieldKey: "cpu", integerValue: "2" },
      { fieldKey: "hostname_prefix", textValue: "old" },
      { fieldKey: "password_length", integerValue: "12" },
    ],
    productDeclaration: { version: "v1", fields },
    customerDeclaration: { version: "v1", fields: [] },
    customerConfigurableFields: fields.filter(field => ["port", "system_disk_io_limit", "data_disk_io_limit"].includes(field.key)),
    productChoiceFields: ["port", "system_disk_io_limit", "data_disk_io_limit"],
    customerFieldSelections: [],
    productResolvedCandidates: {},
    customerResolvedCandidates: {},
  };
  const writes: any[] = [];
  await adminPage.route(
    "**/admin/catalog/products/product-manual-service",
    (r) => r.fulfill({ json: { code: 0, data: detail } }),
  );
  await adminPage.route(
    "**/admin/catalog/product-versions/*/configuration",
    (r) => {
      if (r.request().method() === "PUT") {
        const input = r.request().postDataJSON();
        writes.push(input);
        configuration.merchantValues = input.merchantValues;
        configuration.version++;
      }
      return r.fulfill({ json: { code: 0, data: { configuration } } });
    },
  );
  await adminPage.route("**/configuration/evaluation", (r) => {
    const input = r.request().postDataJSON();
    return r.fulfill({
      json: {
        code: 0,
        data: {
          version: configuration.version,
          evaluation: {
            valid: true,
            values: input.merchantValues,
            issues: [],
            fieldStates: Object.fromEntries(
              fields.map((field) => [
                field.key,
                { visible: true, required: !!field.required, disabled: false },
              ]),
            ),
            candidates: {},
          },
        },
      },
    });
  });
  const view = new MerchantOfferPage(adminPage);
  await view.goto();
  await view.openProductDetail("product-manual-service");
  await view.selectEditSection("购买配置");
  const cpu = adminPage.getByTestId("catalog-option-child-value");
  await expect(cpu).toHaveValue("2");
  await cpu.fill("4");
  await expect(
    adminPage.getByTestId("catalog-option-tab-local_product_password_length"),
  ).toHaveCount(0);
  const rules = adminPage.getByTestId("catalog-provisioning-rules");
  await view.selectEditSection("交付设置");
  await expect(rules).toBeVisible();
  await expect(rules).not.toContainText("system_disk_io_limit");
  await expect(rules).not.toContainText("data_disk_io_limit");
  await expect(rules).not.toContainText("port");
  await expect(rules).toContainText("登录信息与开通规则");
  await view.screenshot("product-login-rules-separated");
  const form = adminPage.getByTestId("catalog-product-parameters-form");
  await expect(form).toContainText("自动生成密码长度");
  await form
    .getByRole("textbox", { name: "主机名前缀", exact: true })
    .fill("demo");
  const save = adminPage.getByTestId("catalog-product-parameters-save");
  await expect(save).toBeEnabled();
  await expect(save).toHaveCSS("border-radius", "6px");
  await save.click();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0]).toMatchObject({
    expectedVersion: 1,
    customerFieldSelections: [],
    merchantValues: expect.arrayContaining([
      { fieldKey: "cpu", integerValue: "2" },
      { fieldKey: "hostname_prefix", textValue: "demo" },
      { fieldKey: "password_length", integerValue: "12" },
    ]),
  });
  await view.selectEditSection("购买配置");
  await expect(cpu).toHaveValue("4");
  await expect(view.offers).not.toContainText("配置已被更新");
  await view.screenshot("deployment-rules-preserve-purchase-draft");
  await view.closeProductDetail();
  const layout = new MainLayout(adminPage);
  await layout.switchLanguage("English");
  await layout.prepareThemeModeWithoutAnimation("dark");
  await view.openProductDetail("product-manual-service");
  await view.selectEditSection("交付设置");
  await expect(rules).toContainText("Login details and provisioning rules");
  await view.screenshot("product-login-rules-dark-english");
  await view.selectEditSection("购买配置");
  await adminPage.setViewportSize({ width: 768, height: 900 });
  await expect(adminPage.getByTestId("catalog-offer-preview")).toBeVisible();
  expect(await adminPage.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  await view.screenshot("product-options-narrow-dark");
});

test("TC-71d: 导入镜像后可编辑可售列表、名称与顺序，组合性能值保留逗号", async ({
  adminPage,
}) => {
  test.setTimeout(60000);
  await installCatalogManagementMock(adminPage);
  const detail: any = mockProductDetail("product-manual-service");
  detail.version.status = "draft";
  detail.fulfillmentBinding = {
    interfaceInstanceNo: "image-fixture",
    interfaceDisplayName: "只读测试上游",
    evidenceVerified: true,
    interfaceNewBusinessState: "accepting",
  };
  const image = {
    key: "operating_system",
    type: "single_select",
    required: true,
    label: { zhCN: "操作系统", enUS: "Operating system" },
    staticCandidates: [],
  };
  const profile = {
    key: "system_disk_io_limit",
    type: "single_line",
    maximumLength: 43,
    label: { zhCN: "系统盘性能", enUS: "System disk performance" },
  };
  const configuration: any = {
    version: 1,
    merchantValues: [],
    customerConfigurableFields: [image, profile],
    customerFieldSelections: [],
    productDeclaration: { version: "v1", fields: [profile] },
    customerDeclaration: { version: "v1", fields: [image] },
    productResolvedCandidates: {},
    customerResolvedCandidates: {
      operating_system: [
        { value: "97", label: { zhCN: "Ubuntu", enUS: "Ubuntu" } },
        { value: "98", label: { zhCN: "Windows", enUS: "Windows" } },
      ],
    },
  };
  const memory = {
    key: "memory",
    type: "integer",
    required: true,
    minimum: "128",
    maximum: "65536",
    label: { zhCN: "内存（MiB）", enUS: "Memory MiB" },
  };
  const gib = {
    key: "memory_gib",
    type: "integer",
    minimum: "1",
    maximum: "64",
    label: { zhCN: "内存（GiB）", enUS: "Memory GiB" },
  };
  configuration.productDeclaration.fields.push(memory, {
    key: "hostname_prefix",
    type: "single_line",
    maximumLength: 10,
    label: { zhCN: "主机名前缀", enUS: "Hostname prefix" },
  });
  configuration.customerConfigurableFields.push(gib);
  configuration.customerDeclaration.fields.push(gib);
  configuration.customerProductFields = { memory_gib: "memory" };
  configuration.productChoiceFields = ["system_disk_io_limit"];
  configuration.merchantValues = [{ fieldKey: "memory", integerValue: "2000" }];
  configuration.customerFieldSelections = [
    { fieldKey: "memory_gib", minimum: "1", maximum: "4" },
  ];
  const images = configuration.customerResolvedCandidates.operating_system;
  configuration.customerResolvedCandidates.operating_system = [];
  let configurationReads = 0;
  const writes: any[] = [];
  await adminPage.route(
    "**/admin/catalog/products/product-manual-service",
    (r) => r.fulfill({ json: { code: 0, data: detail } }),
  );
  await adminPage.route(
    "**/admin/catalog/product-versions/*/configuration",
    (r) => {
      if (r.request().method() === "GET") {
        configurationReads++;
        if (configurationReads > 1)
          configuration.customerResolvedCandidates.operating_system = images;
      }
      if (r.request().method() === "PUT") {
        const input = r.request().postDataJSON();
        writes.push(input);
        configuration.customerFieldSelections = input.customerFieldSelections;
        configuration.version++;
        configuration.customerDeclaration.fields =
          input.customerFieldSelections.map(
            (s: any) =>
              s.localField || {
                ...(configuration.customerConfigurableFields.find(
                  (field: any) => field.key === s.fieldKey,
                ) || image),
                staticCandidates: (s.candidateValues || []).map(
                  (value: string) => ({
                    value,
                    label: s.candidateLabels[value],
                  }),
                ),
              },
          );
      }
      return r.fulfill({ json: { code: 0, data: { configuration } } });
    },
  );
  await adminPage.route("**/configuration-price-candidates", (r) =>
    r.fulfill({ json: { code: 0, data: { candidates: [] } } }),
  );
  const view = new MerchantOfferPage(adminPage);
  await view.goto();
  await view.openProductDetail("product-manual-service");
  await view.selectEditSection("购买配置");
  await expect(view.offers).toContainText("操作系统");
  await expect(view.offers).toContainText("不限定子项时使用接口全部可用选项");
  await expect(
    adminPage.getByTestId("catalog-option-tab-local_product_memory"),
  ).toHaveCount(0);
  await expect(
    adminPage.getByTestId("catalog-option-tab-memory_gib"),
  ).toBeVisible();
  await adminPage.getByTestId("catalog-option-import-choices").click();
  await view.expectChildren("97|Ubuntu|Ubuntu\n98|Windows|Windows");
  await view.setChildren(
    "98|Windows Server|Windows Server\n97|Ubuntu长效版|Ubuntu LTS",
  );
  await view.input("配置名称（中文）").fill("服务器系统");
  await view.addField("系统盘性能 · system_disk_io_limit");
  await view.setChildren(
    "0,0,0,0|不限性能|Unlimited\n500,500,2000,2000|高性能|High performance",
  );
  await adminPage
    .getByTestId("catalog-local-option-type")
    .getByRole("combobox")
    .press("ArrowDown");
  await adminPage
    .locator(".ant-select-dropdown:visible")
    .getByText("单行文本", { exact: true })
    .click();
  await adminPage.getByTestId("catalog-local-option-add").click();
  await view.input("配置名称（中文）").fill("服务器前缀");
  const binding = view.offers
    .locator('[data-testid$="_product_field"]')
    .getByRole("combobox");
  await binding.fill("hostname_prefix");
  await adminPage
    .locator(".ant-select-dropdown:visible")
    .getByText("主机名前缀 · hostname_prefix", { exact: true })
    .click();
  await view.chooseField("operating_system");
  await view.expectChildren(
    "98|Windows Server|Windows Server\n97|Ubuntu长效版|Ubuntu LTS",
  );
  await view.save();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].customerFieldSelections).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        fieldKey: "operating_system",
        displayNameZhCN: "服务器系统",
        candidateValues: ["98", "97"],
        candidateLabels: {
          "98": { zhCN: "Windows Server", enUS: "Windows Server" },
          "97": { zhCN: "Ubuntu长效版", enUS: "Ubuntu LTS" },
        },
      }),
      expect.objectContaining({
        productFieldKey: "hostname_prefix",
        localField: expect.objectContaining({
          type: "single_line",
          maximumLength: 10,
          required: true,
        }),
      }),
      expect.objectContaining({
        productFieldKey: "system_disk_io_limit",
        localField: expect.objectContaining({
          type: "single_select",
          staticCandidates: [
            {
              value: "0,0,0,0",
              label: { zhCN: "不限性能", enUS: "Unlimited" },
            },
            {
              value: "500,500,2000,2000",
              label: { zhCN: "高性能", enUS: "High performance" },
            },
          ],
        }),
      }),
    ]),
  );
  await view.expectChildren(
    "98|Windows Server|Windows Server\n97|Ubuntu长效版|Ubuntu LTS",
  );
  const textChoice = writes[0].customerFieldSelections.find(
    (selection: any) => selection.productFieldKey === "hostname_prefix",
  );
  await view.chooseField(textChoice.fieldKey);
  await expect(adminPage.getByTestId("catalog-offer-field-prices")).toHaveCount(
    0,
  );
  await view.chooseField("operating_system");
  await view.screenshot("merchant-images-and-performance-choices-saved");
});

test("TC-71e: 非服务器商品按接口声明编辑与保存，子项名称独立于接口值", async ({
  adminPage,
}) => {
  test.setTimeout(90000);
  await installCatalogManagementMock(adminPage);
  const detail: any = mockProductDetail("product-manual-service");
  detail.version.status = "draft";
  detail.fulfillmentBinding = {
    interfaceInstanceNo: "certificate-fixture",
    interfaceDisplayName: "证书签发测试接口",
    evidenceVerified: true,
    interfaceNewBusinessState: "accepting",
  };
  const field = {
    key: "validation_method",
    type: "single_select",
    required: true,
    label: { zhCN: "验证方式", enUS: "Validation method" },
    staticCandidates: [
      { value: "dns", label: { zhCN: "域名解析验证", enUS: "DNS validation" } },
      { value: "file", label: { zhCN: "文件验证", enUS: "File validation" } },
    ],
  };
  const configuration: any = {
    version: 1,
    merchantValues: [],
    productDeclaration: { version: "v1", fields: [] },
    customerDeclaration: { version: "v1", fields: [field] },
    customerConfigurableFields: [field],
    customerFieldSelections: [
      { fieldKey: field.key, candidateValues: ["dns", "file"] },
    ],
    productResolvedCandidates: {},
    customerResolvedCandidates: { validation_method: field.staticCandidates },
  };
  const writes: any[] = [];
  await adminPage.route(
    "**/admin/catalog/products/product-manual-service",
    (r) => r.fulfill({ json: { code: 0, data: detail } }),
  );
  await adminPage.route(
    "**/admin/catalog/product-versions/*/configuration",
    (r) => {
      if (r.request().method() === "PUT") {
        const input = r.request().postDataJSON();
        writes.push(input);
        configuration.customerFieldSelections = input.customerFieldSelections;
        configuration.customerDeclaration.fields =
          input.customerFieldSelections.map(
            (selection: any) => selection.localField || field,
          );
        configuration.version++;
      }
      return r.fulfill({ json: { code: 0, data: { configuration } } });
    },
  );
  await adminPage.route("**/configuration-price-candidates", (r) =>
    r.fulfill({ json: { code: 0, data: { candidates: [] } } }),
  );
  const view = new MerchantOfferPage(adminPage);
  await view.goto();
  await view.openProductDetail("product-manual-service");
  await view.selectEditSection("购买配置");
  await view.expectChildren("dns\nfile");
  await view.children.getByTestId("catalog-option-child-0").click();
  await view.children
    .getByTestId("catalog-option-child-zhCN")
    .fill("自动 DNS 验证 | 推荐");
  await view.children.getByTestId("catalog-option-child-down").click();
  await expect(
    view.children.getByTestId("catalog-option-child-1"),
  ).toContainText("自动 DNS 验证 | 推荐");
  await expect(adminPage.getByTestId("catalog-offer-preview")).toBeVisible();
  const preview = adminPage.getByTestId("catalog-offer-preview");
  await expect(preview).toContainText("客户选配预览");
  await preview.getByRole("combobox").press("ArrowDown");
  await adminPage
    .locator(".ant-select-dropdown:visible")
    .getByText("自动 DNS 验证 | 推荐", { exact: true })
    .click();
  await expect(preview).toContainText("自动 DNS 验证 | 推荐");
  expect(writes).toHaveLength(0);
  await view.save();
  await expect.poll(() => writes.length).toBe(1);
  expect(writes[0].customerFieldSelections[0]).toMatchObject({
    fieldKey: "validation_method",
    candidateValues: ["file", "dns"],
    candidateLabels: { dns: { zhCN: "自动 DNS 验证 | 推荐", enUS: "DNS validation" } },
  });
  expect(JSON.stringify(writes[0])).not.toContain('"cpu"');
  expect(JSON.stringify(writes[0])).not.toContain('"operating_system"');
  await expect(
    adminPage.getByTestId("catalog-customer-options-save"),
  ).toBeEnabled();
  await view.expectChildren("file\ndns");
  await expect(
    view.children.getByTestId("catalog-option-child-zhCN"),
  ).toHaveValue("自动 DNS 验证 | 推荐");
  await expect(view.offers).toContainText(
    "值和名称分别填写，客户默认选第一项。",
  );
  await expect(view.offers).not.toContainText("plugin.idc-core.");
  await view.screenshot("generic-certificate-option-rows-saved");
});

test("TC-71f: 本地预览按通用关联保留合法子项，父项切换只重置失效项", async ({ adminPage }) => {
  test.setTimeout(60000);
  await installCatalogManagementMock(adminPage);
  const detail: any = mockProductDetail("product-manual-service");
  detail.version.status = "draft";
  const select = (key: string, name: string, items: Array<[string, string]>) => ({
    key, type: "single_select", required: true, label: { zhCN: name, enUS: key },
    staticCandidates: items.map(([value, label]) => ({ value, label: { zhCN: label, enUS: value } })),
  });
  const fields = [
    select("region", "地区", [["north", "北方地区"], ["south", "南方地区"]]),
    select("route", "线路", [["a", "北方线路"], ["shared", "共享线路"], ["b", "南方线路"]]),
    { key: "note", type: "single_line", label: { zhCN: "交付说明", enUS: "Delivery note" } },
  ];
  const configuration = {
    version: 1, merchantValues: [], productDeclaration: { version: "v1", fields: [] },
    customerDeclaration: { version: "v1", fields, conditions: [
      { when: { operator: "equal", field: "region", value: "north" }, actions: [{ action: "filter_candidates", field: "route", values: ["a", "shared"] }] },
      { when: { operator: "equal", field: "region", value: "south" }, actions: [{ action: "filter_candidates", field: "route", values: ["shared", "b"] }, { action: "show", field: "note" }, { action: "require", field: "note" }] },
    ] },
    customerConfigurableFields: fields, customerFieldSelections: fields.map((field, index) => ({ fieldKey: field.key,
      ...(index === 0 ? { groupZhCN: "地域", groupEnUS: "Location" } : {}) })),
    productResolvedCandidates: {}, customerResolvedCandidates: {},
  };
  const writes: unknown[] = [];
  await adminPage.route("**/admin/catalog/products/product-manual-service", r => r.fulfill({ json: { code: 0, data: detail } }));
  await adminPage.route("**/admin/catalog/product-versions/*/configuration", r => {
    if (r.request().method() !== "GET") writes.push(r.request().postDataJSON());
    return r.fulfill({ json: { code: 0, data: { configuration } } });
  });
  const view = new MerchantOfferPage(adminPage);
  await view.goto();
  await view.openProductDetail("product-manual-service");
  await view.selectEditSection("购买配置");
  await expect(adminPage.getByTestId("catalog-offer-preview")).toBeVisible();
  const preview = adminPage.getByTestId("catalog-offer-preview");
  const pick = async (key: string, label: string) => {
    await preview.getByTestId("catalog-option-preview-" + key).getByRole("combobox").press("ArrowDown");
    await adminPage.locator(".ant-select-dropdown:visible").getByText(label, { exact: true }).click();
  };
  await expect(preview).toContainText("客户选配预览");
  await expect(preview).toContainText("地域 / 地区");
  await expect(preview.getByTestId("catalog-option-preview-route")).toContainText("北方线路");
  await expect(preview.getByRole("textbox", { name: "交付说明" })).toHaveCount(0);
  await pick("route", "共享线路");
  await pick("region", "南方地区");
  await expect(preview.getByTestId("catalog-option-preview-route")).toContainText("共享线路");
  await expect(preview.getByRole("textbox", { name: "交付说明" })).toBeVisible();
  await pick("route", "南方线路");
  await pick("region", "北方地区");
  await expect(preview.getByTestId("catalog-option-preview-route")).toContainText("北方线路");
  await expect(preview.getByRole("textbox", { name: "交付说明" })).toHaveCount(0);
  await expect(preview).not.toContainText("plugin.idc-core.");
  expect(writes).toHaveLength(0);
  await view.screenshot("generic-preview-conditional-options-retained");
});
});
