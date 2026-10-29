import type { Page } from '@host-tests/support/playwright';
import { test, expect } from '../support/merchant-auth';
import { installCatalogManagementMock, mockProductDetail } from '../support/catalog-mock';
import { CatalogPriceOverviewPage } from '../pages/CatalogPriceOverviewPage';

test.describe('TC-83 商品多币种价格总览与复制草稿', () => {
  async function setup(page:Page) {
    await installCatalogManagementMock(page);
    const detail:any=mockProductDetail('product-manual-service');
    detail.version.status='draft';
    const standard:any=mockProductDetail('product-ssl');
    detail.billingRules=[
      {...standard.billingRules[0],billingRuleNo:'billing-month',ruleCode:'monthly',periodUnit:'month'},
      {...standard.billingRules[0],billingRuleNo:'billing-year',ruleCode:'yearly',periodUnit:'year'},
      {...standard.billingRules[0],billingRuleNo:'billing-trial',ruleCode:'trial',periodUnit:'day',chargeType:'trial',pricePhase:'new_purchase'},
    ];
    detail.prices=[
      {priceNo:'cny-month',billingRuleNo:'billing-month',currencyCode:'CNY',amountMinor:'10000',renewalAmountMinor:'11000',setupFeeMinor:'500',minimumQuantity:0,maximumQuantity:null,status:'active'},
      {priceNo:'cny-year',billingRuleNo:'billing-year',currencyCode:'CNY',amountMinor:'120000',renewalAmountMinor:'130000',setupFeeMinor:'900',minimumQuantity:0,maximumQuantity:null,status:'inactive'},
      {priceNo:'usd-month',billingRuleNo:'billing-month',currencyCode:'USD',amountMinor:'1500',renewalAmountMinor:'1700',setupFeeMinor:'100',minimumQuantity:0,maximumQuantity:null,status:'active'},
      {priceNo:'usd-year',billingRuleNo:'billing-year',currencyCode:'USD',amountMinor:'18000',renewalAmountMinor:'19000',setupFeeMinor:'200',minimumQuantity:0,maximumQuantity:null,status:'active'},
      {priceNo:'cny-trial',billingRuleNo:'billing-trial',currencyCode:'CNY',amountMinor:'0',setupFeeMinor:'0',minimumQuantity:0,maximumQuantity:null,status:'active'},
    ];
    detail.taxPolicies=standard.taxPolicies;
    await page.route('**/x/idc-core/api/v1/currencies',route=>route.fulfill({json:{code:0,data:{currencies:['CNY','USD']}}}));
    await page.route('**/admin/catalog/products/product-manual-service',route=>route.fulfill({json:{code:0,data:detail}}));
    await page.route('**/public/customer-center/configuration',route=>route.fulfill({json:{code:0,data:{selectedPlugin:'customer-tdesign',config:{websiteurl:'https://shop.example',websiteconfigurepath:'/configure.html?product={product}',websitecatalogpath:'/products.html'}}}}));
    await page.route('**/admin/catalog/products/*/addons',route=>route.fulfill({json:{code:0,data:{addons:[]}}}));
    await page.route('**/admin/catalog/product-versions/*/configuration',route=>route.fulfill({json:{code:0,data:{configuration:{version:1,merchantValues:[],customerConfigurableFields:[],customerFieldSelections:[],productDeclaration:{version:'v1',fields:[]},customerDeclaration:{version:'v1',fields:[]},productResolvedCandidates:{},customerResolvedCandidates:{}}}}}));
    const ui=new CatalogPriceOverviewPage(page);await ui.goto();await ui.screenshot('price-overview-page');await ui.openProductDetail('product-manual-service');await ui.tab('pricing');
    return {ui,detail};
  }
  test('TC-83a: 同表核对多币种、复制预览不写入、取消无改动、失败保留和统一保存',async({adminPage})=>{
    const {ui,detail}=await setup(adminPage);const writes:any[]=[];
    await adminPage.route('**/configuration-price-batches',async route=>{
      const input=route.request().postDataJSON();writes.push(input);
      if(writes.length===1)return route.fulfill({json:{code:409,data:{},message:'价格已变化，请核对后重试'}});
      for(const row of input.rows){
        detail.prices=detail.prices.filter((price:any)=>!(price.billingRuleNo===row.billingRuleNo&&price.currencyCode===row.currencyCode));
        detail.prices.push(...row.tiers.map((tier:any)=>({...tier,priceNo:row.previousPriceNo,billingRuleNo:row.billingRuleNo,currencyCode:row.currencyCode,status:row.active?'active':'inactive'})));
      }
      await route.fulfill({json:{code:0,data:{saved:input.rows.length}}});
    });
    await expect(ui.priceTable.getByTestId('catalog-base-price-table-currency')).toContainText('全部币种');
    await expect(ui.priceTable).toContainText('首期价格');await expect(ui.priceTable).toContainText('续费价格');await expect(ui.priceTable).toContainText('开通费');
    await expect(ui.priceInput('amount','monthly','CNY')).toHaveValue('100.00');await expect(ui.priceInput('amount','monthly','USD')).toHaveValue('15.00');await expect(ui.priceInput('amount','yearly','CNY')).toHaveValue('1200.00');
    await ui.screenshot('price-overview-all-currencies');
    await ui.openPriceCopy();await expect(ui.priceCopyDialog).toContainText('复制价格');await expect(ui.priceCopy).toContainText('金额原样填入');
    await ui.chooseCopyTarget('CNY · 1 年');await expect(ui.priceCopy.getByTestId('catalog-base-price-copy-preview')).toContainText('1200.00 → 100.00');await ui.screenshot('price-overview-copy-preview');
    await ui.cancelPriceCopy();expect(writes).toHaveLength(0);await expect(ui.priceInput('amount','yearly','CNY')).toHaveValue('1200.00');
    await ui.openPriceCopy();await ui.chooseCopySource('USD · 1 月');
    const usdTargets=await ui.openCopyTargets();await expect(usdTargets).toContainText('USD · 1 年');await expect(usdTargets).not.toContainText('CNY');await expect(usdTargets).not.toContainText('试用');await ui.closeCopyChoices();
    await ui.chooseCopySource('CNY · 1 月');await ui.chooseCopyTarget('CNY · 1 年');await ui.fillPriceDraft();expect(writes).toHaveLength(0);
    await expect(ui.priceInput('amount','yearly','CNY')).toHaveValue('100.00');await expect(ui.priceInput('renewal','yearly','CNY')).toHaveValue('110.00');await expect(ui.priceInput('setup','yearly','CNY')).toHaveValue('5.00');await expect(ui.priceInput('amount','yearly','USD')).toHaveValue('180.00');
    await expect(ui.priceTable.getByTestId('catalog-base-price-pending')).toHaveText('1 项价格未保存');
    await ui.savePriceTable();await expect.poll(()=>writes.length).toBe(1);await expect(ui.priceInput('amount','yearly','CNY')).toHaveValue('100.00');await expect(ui.priceTable.getByText('价格保存或读取未完成，输入已保留。请读取最新价格后核对。',{exact:true})).toBeVisible();await ui.screenshot('price-overview-copy-save-failed');
    await ui.savePriceTable();await expect.poll(()=>writes.length).toBe(2);expect(writes[1]).toEqual({rows:[{targetKind:'base',active:false,previousPriceNo:'cny-year',billingRuleNo:'billing-year',currencyCode:'CNY',tiers:[{minimumQuantity:0,maximumQuantity:null,amountMinor:'10000',setupFeeMinor:'500',renewalAmountMinor:'11000'}]}]});
    await expect(ui.priceTable.getByTestId('catalog-base-price-pending')).toBeHidden();await expect(ui.priceInput('amount','yearly','CNY')).toHaveValue('100.00');await expect(ui.page.getByTestId('catalog-base-price-table-save')).toBeDisabled();await ui.screenshot('price-overview-copy-saved');
  });
  test('TC-83b: 无效金额不进入复制来源，保存定位对应币种周期且不写入',async({adminPage})=>{
    const {ui}=await setup(adminPage);let writes=0;await adminPage.route('**/configuration-price-batches',route=>{writes++;return route.fulfill({json:{code:0,data:{saved:1}}});});
    await ui.priceInput('amount','monthly','CNY').fill('invalid');await ui.openPriceCopy();
    await expect(await ui.openCopySources()).not.toContainText('CNY · 1 月');await ui.closeCopyChoices();await ui.cancelPriceCopy();
    await ui.savePriceTable();expect(writes).toBe(0);await expect(ui.priceTable.getByText('1 月 · CNY 的价格未填写完整或金额精度不正确。',{exact:true})).toBeVisible();await expect(ui.priceInput('amount','monthly','CNY')).toHaveValue('invalid');await ui.screenshot('price-overview-invalid-row');
  });
});
