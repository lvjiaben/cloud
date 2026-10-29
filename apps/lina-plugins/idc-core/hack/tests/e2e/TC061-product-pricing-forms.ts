import type { Page } from '@host-tests/support/playwright';
import { test, expect } from '@host-tests/fixtures/auth';
import { installCatalogManagementMock, mockProductDetail } from '../support/catalog-mock';
import { installCatalogAutomationMock } from '../support/catalog-automation-mock';
import { CatalogWorkspacePage } from '../pages/CatalogWorkspacePage';

async function pricingFixture(page:Page) {
 await installCatalogManagementMock(page);
 const detail:any=mockProductDetail('product-manual-service');
 const rule=(code:string,unit:string)=>({billingRuleNo:`billing-${code}-1`,ruleCode:code,periodUnit:unit,periodCount:1,chargeType:unit==='once'?'one_time':'recurring',quantityMode:'fixed',pricePhase:unit==='once'?'new_purchase':'both',minimumPurchaseCycles:1,maximumRenewalCycles:null,trialQuantity:0,status:'active'});
 detail.billingRules=[rule('month','month'),rule('once','once')];
 detail.prices=[{priceNo:'cny-1',billingRuleNo:'billing-month-1',currencyCode:'CNY',amountMinor:'1100',renewalAmountMinor:'1000',setupFeeMinor:'50',minimumQuantity:0,maximumQuantity:null,status:'active',configurationPriceDimensionNo:''},{priceNo:'jpy-1',billingRuleNo:'billing-once-1',currencyCode:'JPY',amountMinor:'500',renewalAmountMinor:null,setupFeeMinor:'0',minimumQuantity:0,maximumQuantity:null,status:'active',configurationPriceDimensionNo:''}];
 await page.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
 let currencyReads=0;
 await page.route('**/x/idc-core/api/v1/currencies',r=>{currencyReads++;return r.fulfill({json:{code:0,data:{currencies:['CNY','JPY']}}});});
 const batches:Array<{url:string;body:any}>=[];const periods:any[]=[];let failures=0;let legacy=0;let loseResponse=false;
 function advance() {
  const suffix=String(Number(detail.version.versionNumber)+1);detail.version.versionNumber=Number(suffix);detail.version.versionNo=`version-manual-${suffix}`;
  const mapping=new Map<string,string>();
  for(const rule of detail.billingRules){const next=`billing-${rule.ruleCode}-${suffix}`;mapping.set(rule.billingRuleNo,next);rule.billingRuleNo=next;}
  for(const price of detail.prices){price.billingRuleNo=mapping.get(price.billingRuleNo);price.priceNo=`${price.priceNo}-next`;}
 }
 await page.route('**/admin/catalog/product-versions/*/configuration-price-batches',async r=>{
  const body=r.request().postDataJSON();batches.push({url:r.request().url(),body});
  if(failures-->0)return r.fulfill({status:503,json:{code:50,message:'批量价格保存失败'}});
  for(const row of body.rows){detail.prices=detail.prices.filter((price:any)=>!(price.billingRuleNo===row.billingRuleNo && price.currencyCode===row.currencyCode && !price.configurationPriceDimensionNo));detail.prices.push({...row.tiers[0],billingRuleNo:row.billingRuleNo,currencyCode:row.currencyCode,priceNo:`saved-${batches.length}-${row.currencyCode}`,configurationPriceDimensionNo:'',renewalAmountMinor:row.tiers[0].renewalAmountMinor??null,status:row.active===false?'inactive':'active'});}
  advance();
  if(loseResponse)return r.abort('failed');
  return r.fulfill({json:{code:0,data:{}}});
 });
 await page.route('**/admin/catalog/product-versions/*/priced-billing-rules',async r=>{
  const body=r.request().postDataJSON();periods.push(body);
  if(failures-->0)return r.fulfill({status:503,json:{code:50,message:'周期价格保存失败'}});
  const created={...rule(body.ruleCode,body.periodUnit),periodCount:body.periodCount,minimumPurchaseCycles:body.minimumPurchaseCycles,maximumRenewalCycles:body.maximumRenewalCycles??null};detail.billingRules.push(created);
  detail.prices.push({priceNo:'new-period-price',billingRuleNo:created.billingRuleNo,currencyCode:body.currencyCode,amountMinor:body.amountMinor,renewalAmountMinor:body.renewalAmountMinor??null,setupFeeMinor:body.setupFeeMinor,minimumQuantity:0,maximumQuantity:null,status:'active',configurationPriceDimensionNo:''});advance();
  if(loseResponse)return r.abort('failed');return r.fulfill({json:{code:0,data:{billingRuleNo:created.billingRuleNo}}});
 });
 await page.route('**/admin/catalog/product-versions/*/billing-rules',r=>{legacy++;return r.fulfill({status:500,json:{code:50,message:'普通周期不得分步创建'}});});
 const view=new CatalogWorkspacePage(page);
 return {view,detail,batches,periods,advance,failNext:()=>failures=1,loseResponse:()=>loseResponse=true,currencyReads:()=>currencyReads,legacy:()=>legacy};
}
async function openPricing(page:Page,view:CatalogWorkspacePage) {await view.goto();await view.openProductDetail('product-manual-service');await page.getByTestId('catalog-editor-tab-pricing').click();await expect(page.getByTestId('catalog-base-price-table')).toBeVisible();}
async function chooseCurrency(page:Page,code:string) {await page.getByTestId('catalog-base-price-table-currency').click();await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:new RegExp(`^${code}$`)}).click();}
async function openPeriod(page:Page) {await page.getByTestId('catalog-priced-period-panel').getByRole('button').click();await expect(page.getByTestId('catalog-priced-billing-form')).toBeVisible();}

test.describe('TC-61 商品直接计费表单',()=>{
 test('TC-61a: 多币种一次原子保存，精确零价与 JPY 精度，保存回填且无嵌套弹窗',async({adminPage})=>{
  const f=await pricingFixture(adminPage);await openPricing(adminPage,f.view);
  await expect(adminPage.getByTestId('catalog-base-price-amount-month:CNY')).toHaveValue('11.00');
  await adminPage.getByTestId('catalog-base-price-amount-month:CNY').fill('12.34');
  await chooseCurrency(adminPage,'JPY');await expect(adminPage.getByTestId('catalog-base-price-amount-once:JPY')).toHaveValue('500');
  await adminPage.getByTestId('catalog-base-price-amount-once:JPY').fill('0.5');await adminPage.getByTestId('catalog-base-price-table-save').click();
  await expect(adminPage.getByText('一次 · JPY 的价格未填写完整或金额精度不正确。',{exact:true})).toBeVisible();expect(f.batches).toHaveLength(0);
  await adminPage.getByTestId('catalog-base-price-amount-once:JPY').fill('0');await chooseCurrency(adminPage,'CNY');await expect(adminPage.getByTestId('catalog-base-price-amount-month:CNY')).toHaveValue('12.34');
  await adminPage.getByTestId('catalog-base-price-table-save').click();await expect.poll(()=>f.batches.length).toBe(1);
  expect(f.batches[0]!.body.rows).toEqual([{targetKind:'base',active:true,previousPriceNo:'cny-1',billingRuleNo:'billing-month-1',currencyCode:'CNY',tiers:[{minimumQuantity:0,maximumQuantity:null,amountMinor:'1234',setupFeeMinor:'50',renewalAmountMinor:'1000'}]},{targetKind:'base',active:true,previousPriceNo:'jpy-1',billingRuleNo:'billing-once-1',currencyCode:'JPY',tiers:[{minimumQuantity:0,maximumQuantity:null,amountMinor:'0',setupFeeMinor:'0'}]}]);
  await expect(adminPage.getByTestId('catalog-base-price-table-save')).toBeDisabled();await chooseCurrency(adminPage,'JPY');await expect(adminPage.getByTestId('catalog-base-price-amount-once:JPY')).toHaveValue('0');
  await expect(adminPage.getByRole('dialog')).toHaveCount(1);await f.view.screenshot('direct-base-prices-multicurrency-saved');
  await f.view.closeProductDetail();await openPricing(adminPage,f.view);await expect(adminPage.getByTestId('catalog-base-price-amount-month:CNY')).toHaveValue('12.34');
 });
 test('TC-61b: 失败保留价格输入，重试同锚点；丢失响应读回确认而不重复写入',async({adminPage})=>{
  const f=await pricingFixture(adminPage);await openPricing(adminPage,f.view);f.failNext();
  await adminPage.getByTestId('catalog-base-price-amount-month:CNY').fill('18');await adminPage.getByTestId('catalog-base-price-table-save').click();await expect(adminPage.getByText('批量价格保存失败',{exact:true})).toBeVisible();await expect(adminPage.getByTestId('catalog-base-price-amount-month:CNY')).toHaveValue('18');
  await f.view.screenshot('direct-base-prices-failure-retains-input');f.loseResponse();await adminPage.getByTestId('catalog-base-price-table-save').click();await expect.poll(()=>f.batches.length).toBe(2);expect(f.batches[1]!.body).toEqual(f.batches[0]!.body);
  await expect(adminPage.getByTestId('catalog-base-price-refresh')).toBeVisible();await adminPage.getByTestId('catalog-base-price-refresh').click();await expect(adminPage.getByTestId('catalog-base-price-table-save')).toBeDisabled();await expect(adminPage.getByTestId('catalog-base-price-amount-month:CNY')).toHaveValue('18.00');expect(f.batches).toHaveLength(2);await f.view.screenshot('direct-base-prices-response-lost-confirmed');
 });
 test('TC-61c: 其他页签等价保存推进锚点保留输入，真实并发改价禁止覆盖',async({adminPage})=>{
  const f=await pricingFixture(adminPage);let changes=0;
  await adminPage.route('**/admin/catalog/product-versions/*/fulfillment-policy',r=>{changes++;if(changes===2)f.detail.prices.find((price:any)=>price.currencyCode==='CNY').amountMinor='9900';f.advance();return r.fulfill({json:{code:0,data:{policy:{strategyCode:'manual'}}}});});
  await openPricing(adminPage,f.view);await adminPage.getByTestId('catalog-base-price-amount-month:CNY').fill('19.00');
  await f.view.openFulfillmentPolicy();await f.view.fulfillmentPolicyConfirm().click();await expect.poll(()=>changes).toBe(1);await adminPage.getByTestId('catalog-editor-tab-pricing').click();await expect(adminPage.getByTestId('catalog-base-price-amount-month:CNY')).toHaveValue('19.00');
  await adminPage.getByTestId('catalog-base-price-table-save').click();await expect.poll(()=>f.batches.length).toBe(1);expect(f.batches[0]!.url).toContain('version-manual-2');expect(f.batches[0]!.body.rows[0]).toMatchObject({previousPriceNo:'cny-1-next',billingRuleNo:'billing-month-2'});await expect(adminPage.getByTestId('catalog-base-price-table-save')).toBeDisabled();
  await adminPage.getByTestId('catalog-base-price-amount-month:CNY').fill('21.00');await f.view.openFulfillmentPolicy();await f.view.fulfillmentPolicyConfirm().click();await expect.poll(()=>changes).toBe(2);await adminPage.getByTestId('catalog-editor-tab-pricing').click();await expect(adminPage.getByTestId('catalog-base-price-amount-month:CNY')).toHaveValue('21.00');await expect(adminPage.getByTestId('catalog-base-price-table-save')).toBeDisabled();await expect(adminPage.getByTestId('catalog-base-price-reload')).toBeVisible();expect(f.batches).toHaveLength(1);await f.view.screenshot('direct-base-prices-conflict-preserves-input');
 });
 test('TC-61d: 新周期与首价一并提交，失败保留与稳定标识重试，不分步创建',async({adminPage})=>{
  const f=await pricingFixture(adminPage);await openPricing(adminPage,f.view);await openPeriod(adminPage);
  await adminPage.getByTestId('catalog-priced-count').fill('3');await adminPage.getByTestId('catalog-priced-amount').fill('28.50');await adminPage.getByTestId('catalog-priced-renewal').fill('29');await adminPage.getByTestId('catalog-priced-setup').fill('1');f.failNext();
  await adminPage.getByTestId('catalog-priced-save').click();await expect(adminPage.getByText('周期价格保存失败',{exact:true})).toBeVisible();await expect(adminPage.getByTestId('catalog-priced-amount')).toHaveValue('28.50');await f.view.screenshot('priced-period-atomic-failure-keeps-form');
  await adminPage.getByTestId('catalog-priced-save').click();await expect.poll(()=>f.periods.length).toBe(2);expect(f.periods[1]).toEqual(f.periods[0]);expect(f.periods[1]).toMatchObject({periodUnit:'month',periodCount:3,minimumPurchaseCycles:1,currencyCode:'CNY',amountMinor:'2850',renewalAmountMinor:'2900',setupFeeMinor:'100'});expect(f.legacy()).toBe(0);
  await expect(adminPage.getByTestId('catalog-priced-amount')).toHaveValue('');await expect(adminPage.getByTestId(`catalog-base-price-amount-${f.periods[1].ruleCode}:CNY`)).toHaveValue('28.50');await expect(adminPage.getByRole('dialog')).toHaveCount(1);await f.view.screenshot('priced-period-atomic-saved');
 });
 test('TC-61e: 一次性零价无需续费，丢失响应可确认成功，英文和窄屏可读',async({adminPage,mainLayout})=>{
  const f=await pricingFixture(adminPage);await mainLayout.switchLanguage('English');await openPricing(adminPage,f.view);
  await expect(adminPage.getByTestId('catalog-base-price-table')).toContainText('First-period price');await expect(adminPage.getByTestId('catalog-base-price-table-save')).toHaveText('Save base prices');await openPeriod(adminPage);
  await expect(adminPage.getByTestId('catalog-priced-save')).toHaveText('Save period and price');await adminPage.getByTestId('catalog-priced-period').click();await adminPage.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^once$/}).click();await expect(adminPage.getByTestId('catalog-priced-renewal')).toBeHidden();await expect(adminPage.getByTestId('catalog-priced-count')).toBeHidden();
  await adminPage.getByTestId('catalog-priced-amount').fill('0');f.loseResponse();await adminPage.getByTestId('catalog-priced-save').click();await expect.poll(()=>f.periods.length).toBe(1);expect(f.periods[0]).toMatchObject({periodUnit:'once',periodCount:1,minimumPurchaseCycles:1,amountMinor:'0',setupFeeMinor:'0'});expect(f.periods[0].renewalAmountMinor).toBeUndefined();await expect(adminPage.getByTestId('catalog-priced-refresh')).toBeVisible();await adminPage.getByTestId('catalog-priced-refresh').click();await expect(adminPage.getByTestId('catalog-priced-amount')).toHaveValue('');expect(f.periods).toHaveLength(1);
  await adminPage.setViewportSize({width:414,height:896});await expect.poll(()=>f.view.detailDrawer.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);await f.view.screenshot('direct-pricing-english-mobile');
 });
 test('TC-61f: 只读价格无写按钮、不读币种候选，不出现新建周期表单',async({adminPage})=>{
  const f=await pricingFixture(adminPage);await installCatalogAutomationMock(adminPage,{permissions:['idc:admin:catalog:product:list','idc:admin:catalog:product:query']});await adminPage.reload();await openPricing(adminPage,f.view);
  await expect(adminPage.getByTestId('catalog-base-price-table-save')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-priced-period-panel')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-base-price-table')).toContainText('11.00');expect(f.currencyReads()).toBe(0);expect(f.batches).toHaveLength(0);await f.view.screenshot('direct-pricing-read-only');
 });
 test('TC-61g: 停用价格改价保持停用，已有阶梯不被普通表格扁平覆盖',async({adminPage})=>{
  const f=await pricingFixture(adminPage);f.detail.prices[0].status='inactive';
  f.detail.prices.push({...f.detail.prices[1],priceNo:'jpy-tier-2',minimumQuantity:10,amountMinor:'400'});f.detail.prices[1].maximumQuantity=9;
  await openPricing(adminPage,f.view);await adminPage.getByTestId('catalog-base-price-amount-month:CNY').fill('9');await adminPage.getByTestId('catalog-base-price-table-save').click();await expect.poll(()=>f.batches.length).toBe(1);expect(f.batches[0]!.body.rows[0]).toMatchObject({active:false,previousPriceNo:'cny-1',tiers:[{amountMinor:'900'}]});await expect(adminPage.getByTestId('catalog-base-price-table-save')).toBeDisabled();
  await chooseCurrency(adminPage,'JPY');await expect(adminPage.getByTestId('catalog-base-price-amount-once:JPY')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-base-price-table')).toContainText('见高级明细');expect(f.batches).toHaveLength(1);await f.view.screenshot('direct-prices-inactive-and-advanced-protected');
 });

});
