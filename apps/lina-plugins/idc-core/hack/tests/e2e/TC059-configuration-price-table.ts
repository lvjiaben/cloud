import { test, expect } from "@host-tests/fixtures/auth";
import {
  installCatalogManagementMock,
  mockProductDetail,
} from "../support/catalog-mock";
import { CatalogWorkspacePage } from "../pages/CatalogWorkspacePage";

async function mockPriceConfiguration(
  page: Parameters<typeof installCatalogManagementMock>[0],
  numericPricing: "quantity" | "per_choice" = "per_choice",
) {
  const field = {
    key: "cpu",
    type: "integer",
    label: { zhCN: "处理器", enUS: "CPU" },
    minimum: "2",
    maximum: "2",
  };
  await page.route("**/admin/catalog/product-versions/*/configuration", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          configuration: {
            version: 1,
            merchantValues: [],
            customerConfigurableFields: [field],
            customerFieldSelections: [
              {
                fieldKey: "cpu",
                displayControl: "select",
                numericChoices: ["2"],
                numericPricing,
                minimum: "2",
                maximum: "2",
              },
            ],
            productDeclaration: { version: "v1", fields: [] },
            customerDeclaration: { version: "v1", fields: [field] },
            productResolvedCandidates: {},
            customerResolvedCandidates: {},
          },
        },
      },
    }),
  );
}

test.describe("TC-59 配置规格与周期价格表", () => {
  test("TC-59a: 默认关闭自动计算，精确系数预览，原子保存失败保留与重试", async ({
    adminPage,
  }) => {
    test.setTimeout(45000);
    
    await installCatalogManagementMock(adminPage);
    await mockPriceConfiguration(adminPage);
    const detail: any = mockProductDetail("product-manual-service");
    detail.fulfillmentBinding = {
      interfaceDisplayName: "魔方云",
      interfaceInstanceNo: "fixture",
      evidenceVerified: true,
      interfaceNewBusinessState: "accepting",
    };
    detail.billingRules = [
      {
        billingRuleNo: "monthly",
        ruleCode: "monthly",
        chargeType: "recurring",
        periodUnit: "month",
        periodCount: 1,
        pricePhase: "both",
        quantityMode: "fixed",
        status: "active",
      },
      {
        billingRuleNo: "quarterly",
        ruleCode: "quarterly",
        chargeType: "recurring",
        periodUnit: "month",
        periodCount: 3,
        pricePhase: "both",
        quantityMode: "fixed",
        status: "active",
      },
    ];
    detail.prices = [
      {
        priceNo: "base",
        billingRuleNo: "monthly",
        currencyCode: "CNY",
        amountMinor: "100",
        setupFeeMinor: "0",
        renewalAmountMinor: "100",
        minimumQuantity: 0,
        maximumQuantity: null,
        status: "active",
        configurationPriceDimensionNo: "",
      },
    ];
    await adminPage.route(
      "**/admin/catalog/products/product-manual-service",
      (r) => r.fulfill({ json: { code: 0, data: detail } }),
    );
    const candidates = [
      {
        fieldToken: "cpu",
        candidateToken: "cpu2",
        quantity: false,
        pricingMode: "fixed",
        dimensionNo: "dimension2",
        fieldLabelZhCN: "处理器",
        fieldLabelEnUS: "CPU",
        candidateLabelZhCN: "2核",
        candidateLabelEnUS: "2 cores",
      },
    ];
    await adminPage.route("**/configuration-price-candidates", (r) =>
      r.fulfill({ json: { code: 0, data: { candidates } } }),
    );
    const batches: any[] = [];
    await adminPage.route("**/configuration-price-batches", async (r) => {
      batches.push(r.request().postDataJSON());
      if(batches.length>1) for(const row of batches.at(-1).rows)detail.prices.push({...row.tiers[0],billingRuleNo:row.billingRuleNo,currencyCode:row.currencyCode,priceNo:`saved-${row.billingRuleNo}`,status:'active',configurationPriceDimensionNo:'dimension2'});
      await r.fulfill({
        json:
          batches.length === 1
            ? { code: 500, message: "本次价格没有保存", data: null }
            : { code: 0, data: { saved: 2 } },
      });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto();
    await workspace.openProductDetail("product-manual-service");
    await workspace.selectEditSection("购买配置");
    const modal = adminPage.getByTestId("catalog-price-table-form");
    await expect(adminPage.getByTestId("catalog-configuration-price-table-section")).toContainText("配置价格表");
    await expect(modal).toContainText("新购价格");
    await expect(
      adminPage.getByTestId("catalog-price-table-auto"),
    ).toHaveAttribute("aria-checked", "false");
    await expect(
      adminPage.getByTestId("catalog-price-table-base"),
    ).toBeHidden();
    await adminPage.getByTestId("catalog-price-table-auto").click();
    await adminPage.getByTestId("catalog-price-table-base").fill("0.05");
    const coefficients = modal.getByRole("textbox", {
      name: "价格系数",
      exact: true,
    });
    await coefficients.nth(0).fill("0.1");
    await coefficients.nth(1).fill("3");
    await adminPage.getByTestId("catalog-price-table-preview").click();
    await expect(modal).toContainText("0.01");
    await expect(modal).toContainText("0.15");
    expect(batches).toHaveLength(0);
    await adminPage.getByTestId("catalog-price-table-base").fill("0.10");
    await adminPage.getByTestId("catalog-price-table-apply").click();
    await expect(
      modal
        .getByRole("textbox", { name: "新购价格", exact: true })
        .nth(0),
    ).toHaveValue("");
    await adminPage.getByTestId("catalog-price-table-base").fill("0.05");
    await adminPage.getByTestId("catalog-price-table-preview").click();
    await coefficients.nth(0).fill("0.2");
    await adminPage.getByTestId("catalog-price-table-apply").click();
    await expect(
      modal
        .getByRole("textbox", { name: "新购价格", exact: true })
        .nth(0),
    ).toHaveValue("");
    await coefficients.nth(0).fill("0.1");
    await adminPage.getByTestId("catalog-price-table-preview").click();
    await adminPage.getByTestId("catalog-price-table-apply").click();
    await modal
      .getByRole("textbox", { name: "续费价格", exact: true })
      .nth(0)
      .fill("0.01");
    await modal
      .getByRole("textbox", { name: "续费价格", exact: true })
      .nth(1)
      .fill("0.15");
    await expect(adminPage.getByRole('tab',{name:'产品配置',exact:true})).toHaveAttribute('aria-selected','true');
    await expect(adminPage.getByTestId('catalog-configuration-price-add')).toBeHidden();
    await adminPage.getByTestId('catalog-editor-tab-pricing').click();
    await expect(modal).toBeHidden();
    await expect(adminPage.getByTestId('catalog-detail-base-prices')).toBeVisible();
    await adminPage.getByTestId('catalog-editor-tab-configuration').click();
    await expect(modal.getByRole('textbox',{name:'新购价格',exact:true}).nth(0)).toHaveValue('0.01');
    await workspace.screenshot("configuration-price-table-preview");
    await adminPage.getByTestId("catalog-price-table-save").click();
    await expect(modal).toBeVisible();
    await expect(modal).toContainText("输入已保留");
    await expect(
      modal
        .getByRole("textbox", { name: "新购价格", exact: true })
        .nth(0),
    ).toHaveValue("0.01");
    await workspace.screenshot("configuration-price-table-error");
    await adminPage.getByTestId("catalog-price-table-save").click();
    await expect(modal).toBeVisible();
    expect(batches).toHaveLength(2);
    await expect(modal.getByRole('textbox',{name:'新购价格',exact:true}).nth(0)).toHaveValue('0.01');
    await expect(adminPage.getByRole('dialog')).toHaveCount(1);
    expect(batches[1].rows).toEqual([
      {
        candidateToken: "cpu2",
        previousPriceNo: "",
        billingRuleNo: "monthly",
        currencyCode: "CNY",
        tiers: [
          {
            minimumQuantity: 0,
            maximumQuantity: null,
            amountMinor: "1",
            setupFeeMinor: "0",
            renewalAmountMinor: "1",
          },
        ],
      },
      {
        candidateToken: "cpu2",
        previousPriceNo: "",
        billingRuleNo: "quarterly",
        currencyCode: "CNY",
        tiers: [
          {
            minimumQuantity: 0,
            maximumQuantity: null,
            amountMinor: "15",
            setupFeeMinor: "0",
            renewalAmountMinor: "15",
          },
        ],
      },
    ]);
  });
  test("TC-59b: 已有规格价格修改、立即重开与删除保持实际状态", async ({
    adminPage,
  }) => {
    test.setTimeout(45000);
    
    await installCatalogManagementMock(adminPage);
    await mockPriceConfiguration(adminPage);
    const detail: any = mockProductDetail("product-manual-service");
    detail.fulfillmentBinding = {
      interfaceDisplayName: "魔方云",
      interfaceInstanceNo: "fixture",
      evidenceVerified: true,
      interfaceNewBusinessState: "accepting",
    };
    detail.billingRules = [
      {
        billingRuleNo: "monthly",
        ruleCode: "monthly",
        chargeType: "recurring",
        periodUnit: "month",
        periodCount: 1,
        pricePhase: "both",
        quantityMode: "fixed",
        status: "active",
      },
    ];
    const price = {
      priceNo: "specification-price",
      billingRuleNo: "monthly",
      currencyCode: "CNY",
      amountMinor: "500",
      setupFeeMinor: "300",
      renewalAmountMinor: "600",
      minimumQuantity: 0,
      maximumQuantity: null,
      status: "active",
      configurationPriceDimensionNo: "dimension2",
      configurationPricingMode: "fixed",
      configurationFieldLabelZhCn: "处理器",
      configurationFieldLabelEnUs: "CPU",
      configurationCandidateLabelZhCn: "2核",
      configurationCandidateLabelEnUs: "2 cores",
    };
    detail.prices = [
      {
        ...price,
        priceNo: "base",
        configurationPriceDimensionNo: "",
        amountMinor: "100",
      },
      price,
    ];
    let failRefresh = false;
    await adminPage.route(
      "**/admin/catalog/products/product-manual-service",
      (r) => {
        if (failRefresh) {
          failRefresh = false;
          return r.fulfill({
            json: { code: 500, message: "刷新失败", data: null },
          });
        }
        return r.fulfill({ json: { code: 0, data: detail } });
      },
    );
    await adminPage.route("**/configuration-price-candidates", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            candidates: [
              {
                fieldToken: "cpu",
                candidateToken: "cpu2",
                quantity: false,
                pricingMode: "fixed",
                dimensionNo: "dimension2",
                fieldLabelZhCN: "处理器",
                fieldLabelEnUS: "CPU",
                candidateLabelZhCN: "2核",
                candidateLabelEnUS: "2 cores",
              },
            ],
          },
        },
      }),
    );
    const batches: any[] = [];
    await adminPage.route("**/configuration-price-batches", async (r) => {
      const body = r.request().postDataJSON();
      batches.push(body);
      failRefresh = true;
      Object.assign(price, {
        amountMinor: body.rows[0].tiers[0].amountMinor,
        priceNo: "edited-specification-price",
      });
      await r.fulfill({ json: { code: 0, data: { saved: 1 } } });
    });
    const deletes: string[] = [];
    await adminPage.route("**/price-schedules/*", async (r) => {
      expect(r.request().method()).toBe("DELETE");
      deletes.push(r.request().url().split("/").at(-1)!);
      detail.prices = detail.prices.filter(
        (p: any) => p.configurationPriceDimensionNo !== "dimension2",
      );
      await r.fulfill({ json: { code: 0, data: {} } });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto();
    await workspace.openProductDetail("product-manual-service");
    await workspace.selectEditSection("购买配置");
    const modal = adminPage.getByTestId("catalog-price-table-form");
    const amount = modal.getByRole("textbox", {
      name: "新购价格",
      exact: true,
    });
    await expect(amount).toHaveValue("5.00");
    await amount.fill("7");
    await adminPage.getByTestId("catalog-price-table-save").click();
    await expect(modal).toBeVisible();
    expect(batches).toHaveLength(1);
    expect(batches[0].rows[0]).toMatchObject({
      previousPriceNo: "specification-price",
      tiers: [
        { amountMinor: "700", setupFeeMinor: "300", renewalAmountMinor: "600" },
      ],
    });
    await adminPage.getByTestId("catalog-price-table-refresh").click();
    await expect(amount).toHaveValue("7.00");
    await modal.getByRole("button", { name: /删\s*除/ }).click();
    await adminPage
      .locator(".ant-popover:visible")
      .getByRole("button", { name: /确\s*定/ })
      .click();
    await expect(amount).toHaveValue("");
    expect(deletes).toEqual(["edited-specification-price"]);
    await workspace.screenshot("configuration-price-table-deleted");
  });

  test("TC-59c: 数量价格的计算工具默认关闭，变更参数后旧预览失效", async ({
    adminPage,
  }) => {
    test.setTimeout(45000);
    
    await installCatalogManagementMock(adminPage);
    await mockPriceConfiguration(adminPage, "quantity");
    const detail: any = mockProductDetail("product-manual-service");
    detail.fulfillmentBinding = {
      interfaceDisplayName: "魔方云",
      interfaceInstanceNo: "fixture",
      evidenceVerified: true,
      interfaceNewBusinessState: "accepting",
    };
    detail.billingRules = [
      {
        billingRuleNo: "monthly",
        ruleCode: "monthly",
        chargeType: "recurring",
        periodUnit: "month",
        periodCount: 1,
        pricePhase: "both",
        quantityMode: "fixed",
        status: "active",
      },
    ];
    detail.prices = [
      {
        priceNo: "base",
        billingRuleNo: "monthly",
        currencyCode: "CNY",
        amountMinor: "100",
        setupFeeMinor: "0",
        renewalAmountMinor: "100",
        minimumQuantity: 0,
        maximumQuantity: null,
        status: "active",
        configurationPriceDimensionNo: "",
      },
    ];
    await adminPage.route(
      "**/admin/catalog/products/product-manual-service",
      (r) => r.fulfill({ json: { code: 0, data: detail } }),
    );
    await adminPage.route("**/x/idc-core/api/v1/currencies", (r) =>
      r.fulfill({ json: { code: 0, data: { currencies: ["CNY"] } } }),
    );
    await adminPage.route("**/configuration-price-candidates", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            candidates: [
              {
                fieldToken: "cpu",
                candidateToken: "cpu-quantity",
                quantity: true,
                pricingMode: "per_unit",
                dimensionNo: "cpu-quantity-dimension",
                fieldLabelZhCN: "处理器",
                fieldLabelEnUS: "CPU",
                candidateLabelZhCN: "处理器",
                candidateLabelEnUS: "CPU",
              },
            ],
          },
        },
      }),
    );
    const writes: any[] = [];
    await adminPage.route("**/billing-rules/*/price-schedules", async (r) => {
      writes.push(r.request().postDataJSON());
      await r.fulfill({ json: { code: 0, data: {} } });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto();
    await workspace.openProductDetail("product-manual-service");
    await workspace.selectEditSection("购买配置");
    await workspace.openFieldPrice("cpu");
    const dialog = adminPage.getByRole("dialog").filter({
      has: adminPage.getByTestId("catalog-configuration-price-modal"),
    });
    await expect(adminPage.getByTestId("catalog-price-auto")).toHaveAttribute(
      "aria-checked",
      "false",
    );
    await adminPage.getByTestId("catalog-price-auto").click();
    await adminPage.getByTestId("catalog-price-calculation-base").fill("5");
    await adminPage
      .getByTestId("catalog-price-calculation-coefficient")
      .fill("3");
    await adminPage.getByTestId("catalog-price-auto-preview").click();
    await expect(adminPage.getByTestId("catalog-price-auto-result")).toHaveText(
      "15.00",
    );
    await adminPage
      .getByTestId("catalog-price-calculation-coefficient")
      .fill("4");
    await expect(
      adminPage.getByTestId("catalog-price-auto-apply"),
    ).toBeHidden();
    await adminPage.getByTestId("catalog-price-auto-preview").click();
    await expect(adminPage.getByTestId("catalog-price-auto-result")).toHaveText(
      "20.00",
    );
    await adminPage.getByTestId("catalog-price-auto-apply").click();
    await expect(
      adminPage.getByTestId("catalog-configuration-price-amount"),
    ).toHaveValue("20.00");
    await adminPage
      .getByTestId("catalog-configuration-price-renewalAmount")
      .fill("5");
    await workspace.confirmConfigurationPrice();
    await expect(dialog).toBeHidden();
    expect(writes[0].tiers[0]).toMatchObject({
      amountMinor: "2000",
      setupFeeMinor: "0",
      renewalAmountMinor: "500",
    });
  });
 test('TC-59d: 英文直接价格表加载失败可重试，其他页签修改后保留输入并禁止覆盖',async({adminPage,mainLayout})=>{
  await installCatalogManagementMock(adminPage);await mockPriceConfiguration(adminPage);
  const detail:any=mockProductDetail('product-manual-service');
  detail.fulfillmentBinding={interfaceInstanceNo:'fixture',interfaceDisplayName:'Fixture',evidenceVerified:true,interfaceNewBusinessState:'accepting'};
  detail.billingRules=[{billingRuleNo:'monthly',ruleCode:'monthly',chargeType:'recurring',periodUnit:'month',periodCount:1,pricePhase:'both',quantityMode:'fixed',status:'active',minimumPurchaseCycles:1}];
  detail.prices=[{priceNo:'base',billingRuleNo:'monthly',currencyCode:'CNY',amountMinor:'100',renewalAmountMinor:'100',setupFeeMinor:'0',minimumQuantity:0,maximumQuantity:null,status:'active',configurationPriceDimensionNo:''}];
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  let reads=0,writes=0;
  await adminPage.route('**/configuration-price-candidates',r=>{reads++;return reads===1?r.fulfill({status:503,json:{code:50,message:'Candidates temporarily unavailable'}}):r.fulfill({json:{code:0,data:{candidates:[{fieldToken:'cpu',candidateToken:'cpu2',quantity:false,pricingMode:'fixed',dimensionNo:'dimension2',fieldLabelZhCN:'处理器',fieldLabelEnUS:'CPU',candidateLabelZhCN:'2核',candidateLabelEnUS:'2 cores'}]}}});});
  await adminPage.route('**/configuration-price-batches',r=>{writes++;return r.fulfill({json:{code:0,data:{}}});});
  await adminPage.route('**/fulfillment-policy',r=>{detail.version.versionNo='version-other-setting-saved';return r.fulfill({json:{code:0,data:{policy:{strategyCode:'manual'}}}});});
  await mainLayout.switchLanguage('English');const view=new CatalogWorkspacePage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');await adminPage.getByTestId('catalog-editor-tab-configuration').click();
  const form=adminPage.getByTestId('catalog-price-table-form');await expect(form).toContainText('The price table could not load');await adminPage.getByTestId('catalog-price-table-refresh').click();
  await expect(form).toContainText('Specification');await expect(adminPage.getByTestId('catalog-price-table-save')).toHaveText('Save configuration prices');
  const amount=form.getByRole('textbox',{name:'Purchase',exact:true}).first();await amount.fill('9.50');
  await view.openFulfillmentPolicy();await view.fulfillmentPolicyConfirm().click();await adminPage.getByTestId('catalog-editor-tab-configuration').click();
  await expect(amount).toHaveValue('9.50');await expect(adminPage.getByTestId('catalog-price-table-save')).toBeDisabled();await expect(form).toContainText('Your input is retained');expect(writes).toBe(0);await adminPage.getByTestId('catalog-price-table-reload').scrollIntoViewIfNeeded();await view.screenshot('inline-configuration-prices-english-conflict');
  await adminPage.getByTestId('catalog-price-table-reload').click();await adminPage.locator('.ant-popover:visible').getByRole('button',{name:'OK',exact:true}).click();await expect(amount).toHaveValue('');await expect(adminPage.getByRole('dialog')).toHaveCount(1);
  await adminPage.setViewportSize({width:414,height:896});await expect.poll(()=>view.detailDrawer.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true).catch(async error=>{
    const layout=await view.detailDrawer.evaluate(root=>{
     const right=root.getBoundingClientRect().right;
     return {width:root.clientWidth,scroll:root.scrollWidth,overflow:[...root.querySelectorAll('*')].filter(el=>{
      const box=el.getBoundingClientRect();if(box.width===0 || box.right<=right+1)return false;
      for(let parent=el.parentElement;parent && parent!==root;parent=parent.parentElement)if(['auto','hidden','clip','scroll'].includes(getComputedStyle(parent).overflowX))return false;
      return true;
     }).slice(0,20).map(el=>({tag:el.tagName,classes:el.className,testId:el.getAttribute('data-testid'),width:el.getBoundingClientRect().width,right:el.getBoundingClientRect().right,text:el.textContent?.trim().slice(0,80)}))};
    });
    await test.info().attach('configuration-width-diagnostic',{body:Buffer.from(JSON.stringify(layout,null,2)),contentType:'application/json'});throw error;
  });await form.scrollIntoViewIfNeeded();await view.screenshot('inline-configuration-prices-english-mobile');
 });

});
