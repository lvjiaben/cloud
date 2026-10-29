import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/WebsitePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},customerOrigin:'http://localhost:6032',apiOrigin:'http://127.0.0.1:6030'});

// Controlled contract inputs, isolated to browser responses. No real catalog,
// cart, account or order is created; assertions cover purchase rejection only.
const root='/x/idc-core/api/v1/public/catalog/products/controlled-desktop-input';
const revision='r'.repeat(43);
const field={fieldToken:'controlled-boolean',type:'boolean',labelZhCN:'需要附加服务',visible:true,required:true};
const values=[{fieldToken:field.fieldToken,type:'boolean',booleanValue:false}];
const view={configurationRevision:revision,fields:[field],values};

test.describe('TC-8 桌面配置拒绝与恢复契约',()=>{
 test.beforeEach(async({page})=>{
  await page.route('**'+root,route=>route.fulfill({json:{code:0,data:{product:{publicSlug:'controlled-desktop-input',nameZhCN:'受控配置输入',availability:'available',currencies:['CNY']}}}}));
  await page.route('**'+root+'/purchase-configuration',route=>route.fulfill({json:{code:0,data:view}}));
  await page.route('**'+root+'/purchase-configuration/evaluate',route=>route.fulfill({json:{code:0,data:{...view,valid:true}}}));
  await page.route('**'+root+'/purchase-configuration/quote',route=>route.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[{billingTermToken:'controlled-term',labelZhCN:'受控周期',minimumPurchaseCycles:2,status:'final',totalAmount:'100'}]}}}));
 });
 test('TC-8a: 必填布尔值为否仍有效，数量和周期越界不能提交',async({page})=>{
  const site=new WebsitePage(page);let writes=0;
  await page.route('**/public/commerce/**',route=>{writes++;return route.abort();});
  await site.open('/configure.html?product=controlled-desktop-input');await site.chooseTerm();
  const checkbox=page.getByLabel('需要附加服务');await expect(checkbox).not.toBeChecked();
  expect(await checkbox.evaluate((input:HTMLInputElement)=>input.checkValidity())).toBeTruthy();
  const bounds=await checkbox.boundingBox();expect(bounds).not.toBeNull();expect(bounds!.width).toBeGreaterThanOrEqual(16);expect(bounds!.width).toBeLessThanOrEqual(24);expect(bounds!.height).toBeGreaterThanOrEqual(16);
  await expect(page.locator('[data-add]')).toBeEnabled();
  for(const [name,value]of [['quantity','0'],['quantity','1001'],['quantity','1.5'],['cycles','1'],['cycles','121'],['cycles','2.5']]as const){
   await site.setPurchaseCount('quantity','1');await site.setPurchaseCount('cycles','2');await site.setPurchaseCount(name,value);
   await expect(page.locator('[data-add]')).toBeDisabled();
   // Browser form validity is independently verified, not just button styling.
   expect(await page.locator('#addCartForm').evaluate((form:HTMLFormElement)=>form.checkValidity())).toBeFalsy();
  }
  expect(writes).toBe(0);await site.capture('desktop-count-rejections');
 });
 test('TC-8b: 配置重新读取发现商品已下架时清空报价并阻止提交',async({page})=>{
  const site=new WebsitePage(page);await site.open('/configure.html?product=controlled-desktop-input');await site.chooseTerm();
  await expect(page.locator('[data-add]')).toBeEnabled();
  await page.route('**'+root,route=>route.fulfill({json:{code:0,data:{product:{publicSlug:'controlled-desktop-input',availability:'unavailable'}}}}));
  await site.refreshConfiguration();await expect(page.locator('[data-status]')).toHaveText('商品当前不可订购。');
  await expect(page.locator('[data-add]')).toBeDisabled();await expect(page.locator('[data-quote]')).toBeEmpty();
  await site.capture('desktop-withdrawn-rejection');
 });
 test('TC-8c: 加车请求失败时不跳转、不报告成功且禁止重用旧报价',async({page})=>{
  const site=new WebsitePage(page);let opens=0,adds=0;
  // Use the configured customer host, so this tests the API error path rather
  // than the separate cross-host ownership guard. No cart request reaches backend.
  await page.route('**/public/commerce/carts',route=>{opens++;return route.fulfill({status:503,json:{code:503}});});
  await page.route('**/public/commerce/cart-items',route=>{adds++;return route.abort();});
  await site.open('/configure.html?product=controlled-desktop-input');await site.chooseTerm();await site.submit();
  await expect(page.locator('[data-status]')).toHaveText('购物车暂不可用，尚未提交商品，请重新报价后重试。');
  expect(opens).toBe(1);expect(adds).toBe(0);
  await expect(page).toHaveURL(/\/configure\.html\?product=/);await expect(page.locator('[data-add]')).toBeDisabled();await expect(page.locator('[data-quote]')).toBeEmpty();
  await expect(page.locator('#addCartForm')).not.toHaveAttribute('inert','');await site.capture('desktop-cart-failure');
 });
 test('TC-8d: 先填写有效周期再选择报价不丢失输入，低于后台下限才调整',async({page})=>{
  const site=new WebsitePage(page);await site.open('/configure.html?product=controlled-desktop-input');
  await expect(page.locator('[data-quote] button').first()).toBeVisible();
  await site.setPurchaseCount('cycles','4');await site.chooseTerm();
  await expect(page.locator('[name=cycles]')).toHaveValue('4');
  await expect(page.locator('[data-add]')).toBeEnabled();
  await site.setPurchaseCount('cycles','1');await site.chooseTerm();
  await expect(page.locator('[name=cycles]')).toHaveValue('2');
  await expect(page.locator('[data-add]')).toBeEnabled();
 });
});


