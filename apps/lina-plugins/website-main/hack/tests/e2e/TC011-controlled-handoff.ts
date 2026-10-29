import {test,expect} from '../support/source-site';
import {resolve} from 'node:path';
import {mkdir} from 'node:fs/promises';
import {WebsitePage} from '../pages/WebsitePage';

// Browser-only contracts: all test product and handoff responses are intercepted.
// This suite does not authenticate, persist drafts, create carts or place orders.
const root='/x/idc-core/api/v1/public/catalog/products/controlled-handoff';
const revision='r'.repeat(43);
const view={configurationRevision:revision,fields:[{fieldToken:'note',type:'single_line',labelZhCN:'交付备注',visible:true,required:false},{fieldToken:'document',type:'file',labelZhCN:'附件',visible:true,required:true}],values:[{fieldToken:'note',type:'single_line',textValue:'保留输入 & + 中文'}]};

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},customerOrigin:'http://localhost:6032',apiOrigin:'http://127.0.0.1:6030'});

test.describe('TC-11 官网配置交接受控回归',()=>{
 test.beforeEach(async({page})=>{
  await page.route('**'+root,route=>route.fulfill({json:{code:0,data:{product:{publicSlug:'controlled-handoff',nameZhCN:'受控配置交接（非真实商品）',availability:'available',currencies:['CNY']}}}}));
  await page.route('**'+root+'/purchase-configuration',route=>route.fulfill({json:{code:0,data:view}}));
  await page.route('**'+root+'/purchase-configuration/evaluate',route=>route.fulfill({json:{code:0,data:{...view,valid:false}}}));
 });
 test('TC-11a 保存失败时保留选择与数量，重复点击只提交一次',async({page})=>{
  const site=new WebsitePage(page);let calls=0;let release:()=>void=()=>{};
  const held=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/public/purchase-configuration-handoffs',async route=>{calls++;await held;await route.fulfill({status:503,json:{code:503}});});
  await site.open('/configure.html?product=controlled-handoff');
  await site.setPurchaseCount('quantity','3');await site.setPurchaseCount('cycles','2');
  await site.saveConfiguration();await expect(page.locator('#addCartForm')).toHaveAttribute('inert','');
  await expect(page.locator('[data-status]')).toHaveText('正在保存选择，随后前往用户中心…');
  await site.repeatConfigurationSave();expect(calls).toBe(1);release();
  await expect(page.locator('[data-status]')).toHaveText('未能保存配置，尚未跳转。请稍后重试，当前选择仍保留。');
  expect(calls).toBe(1);await expect(page.getByLabel('交付备注')).toHaveValue('保留输入 & + 中文');
  await expect(page.locator('[name=quantity]')).toHaveValue('3');await expect(page.locator('[name=cycles]')).toHaveValue('2');
  await expect(page).toHaveURL(/\/configure.html\?product=controlled-handoff$/);
  await expect(page.getByRole('link',{name:'已保存配置，打开用户中心'})).toHaveCount(0);
  const output=resolve(import.meta.dirname,'../../../../../../temp/20261017');await mkdir(output,{recursive:true});
  await page.screenshot({path:resolve(output,Date.now()+'-controlled-handoff-failed.png'),fullPage:true});
 });
 test('TC-11b 保存成功只传定位信息，后端收到原字段和数量周期',async({page,context})=>{
  const reference='a'.repeat(64);let body:any;
  await page.route('**/public/purchase-configuration-handoffs',route=>{body=route.request().postDataJSON();return route.fulfill({json:{code:0,data:{reference,expiresAt:Date.now()+600000}}});});
  await context.route('**/account/products/controlled-handoff/configure?**',route=>route.fulfill({contentType:'text/html',body:'<h1>受控目标：未进行真实登录或购物车测试</h1>'}));
  const site=new WebsitePage(page);await site.open('/configure.html?product=controlled-handoff');
  await site.setPurchaseCount('quantity','3');await site.setPurchaseCount('cycles','2');
  await site.saveConfiguration();
  await expect(page).toHaveURL(/\/configure\.html\?product=controlled-handoff$/);
  const link=page.getByRole('link',{name:'已保存配置，打开用户中心'});
  await expect(link).toHaveAttribute('target','_blank');
  await expect(link).toHaveAttribute('rel','noopener');
  const [popup]=await Promise.all([page.waitForEvent('popup'),link.click()]);
  await expect(popup).toHaveURL(new RegExp('/account/products/controlled-handoff/configure\\?handoff='+reference+'&currency=CNY$'));
  expect(await popup.evaluate(()=>window.opener)).toBeNull();
  const popupURL=new URL(popup.url());
  expect(popupURL.searchParams.size).toBe(2);
  await popup.close();
  expect(body).toEqual({publicSlug:'controlled-handoff',configurationRevision:revision,currencyCode:'CNY',billingTermToken:'',quantity:3,purchaseCycles:2,values:view.values});
  expect(new URL(page.url()).searchParams.get('product')).toBe('controlled-handoff');
  expect(new URL(page.url()).searchParams.size).toBe(1);
 });
 test('TC-11c 无效保存响应不跳转，不能误报配置已保存',async({page})=>{
  await page.route('**/public/purchase-configuration-handoffs',route=>route.fulfill({json:{code:0,data:{reference:'tampered',expiresAt:Date.now()+1000}}}));
  const site=new WebsitePage(page);await site.open('/configure.html?product=controlled-handoff');
  await site.saveConfiguration();
  await expect(page.locator('[data-status]')).toHaveText('未能保存配置，尚未跳转。请稍后重试，当前选择仍保留。');
  await expect(page).toHaveURL(/\/configure.html\?product=controlled-handoff$/);
  await expect(page.getByRole('link',{name:'已保存配置，打开用户中心'})).toHaveCount(0);
 });
});



