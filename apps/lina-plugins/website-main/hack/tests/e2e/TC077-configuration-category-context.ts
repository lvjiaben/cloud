import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {ConfigurationControlsPage} from '../pages/ConfigurationControlsPage';
import type {Page} from '@playwright/test';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},headless:true});
const root=prefix+'/catalog/products/configuration-context',revision='k'.repeat(43);
const defaultNotice='温馨提示：服务器禁止涉及色情、赌博、政治、诈骗等违法违规业务，请遵守所在地以及中国大陆法律法规，合法使用云产品资源。';
const categories=[
 {categoryNo:'cloud',parentCategoryNo:'',nameZhCN:'弹性云',purchaseNotice:'登录后可查看本分类适用的配置与价格。',usageNotice:'请按适用法律法规使用云资源。'},
 {categoryNo:'xiangyang',parentCategoryNo:'cloud',nameZhCN:'湖北襄阳弹性云',purchaseNotice:'电信优化与BGP线路，具体规格以当前商品配置为准。',usageNotice:'服务器资源仅用于合法合规业务。<script>alert(1)</script>'}
];

type CategoryMode='complete'|'missing-child'|'empty-purchase'|'empty-usage'|'unavailable';
async function setup(page:Page,initialMode:CategoryMode='complete'){
 let mode=initialMode;
 await controlledBusiness(page);
 await page.route('**'+prefix+'/catalog/products?limit=1',route=>{
  if(mode==='unavailable')return route.fulfill({status:503,json:{code:50,data:null}});
  const published=categories.map(category=>({...category}));
  if(mode==='missing-child')published.pop();
  if(mode==='empty-purchase')published[1].purchaseNotice='';
  if(mode==='empty-usage')published[1].usageNotice='';
  return route.fulfill({json:{code:0,data:{categories:published,products:[],nextCursor:''}}});
 });
 await page.route('**'+root,route=>route.fulfill({json:{code:0,data:{product:{publicSlug:'configuration-context',nameZhCN:'ECS - 湖北襄阳 | 弹性云',categoryNo:'xiangyang',availability:'available',currencies:['CNY']}}}}));
 await page.route('**'+root+'/purchase-configuration',route=>route.fulfill({json:{code:0,data:{configurationRevision:revision,fields:[],values:[]}}}));
 await page.route('**'+root+'/purchase-configuration/evaluate',route=>route.fulfill({json:{code:0,data:{configurationRevision:revision,fields:[],values:[],valid:true}}}));
 await page.route('**'+root+'/purchase-configuration/quote',route=>route.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[{billingTermToken:'month',labelZhCN:'月付',minimumPurchaseCycles:1,status:'final',totalAmount:'9900'}]}}}));
 const site=new ConfigurationControlsPage(page);await site.open('/configure.html?product=configuration-context');return {site,setMode:(next:CategoryMode)=>{mode=next;}};
}

test.describe('TC-77 小白云式配置页分类上下文与提示（隔离受控数据）',()=>{
 test('TC-77a: 父子分类、顶部购买说明和底部温馨提示来自公开分类接口',async({page})=>{
  const {site}=await setup(page);
  await expect(page.locator('[data-configure-category]')).toHaveText('商品分类：弹性云 / 湖北襄阳弹性云');
  await expect(page.locator('[data-catalog-purchase-notice]')).toHaveText('电信优化与BGP线路，具体规格以当前商品配置为准。');
  await expect(page.locator('[data-catalog-usage-notice]')).toHaveText('服务器资源仅用于合法合规业务。<script>alert(1)</script>');
  await expect(page.locator('[data-catalog-usage-notice] script')).toHaveCount(0);
  await expect(page.locator('[data-catalog-product][aria-current="page"]')).toHaveText('ECS - 湖北襄阳 | 弹性云');
  await expect(page.locator('[data-catalog-product][aria-current="page"]')).toBeVisible();
  await expect(page.locator('#server-group-xiangyang .lina-server-menu-feedback')).toBeEmpty();
  await expect(page.locator('[data-catalog-title]')).toHaveText('区域（ECS - 湖北襄阳 | 弹性云）');
  await expect(page.locator('[data-quote] button')).toHaveText('月：CNY 99.00');
  console.log('TC077 screenshot: '+await new ConfigurationControlsPage(page).capture('tc077-category-context-desktop'));
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  console.log('TC077 screenshot: '+await new ConfigurationControlsPage(page).capture('tc077-category-context-mobile',false));
 });
 test('TC-77b: 分类未发布或单项提示为空时不继承、不残留',async({page})=>{
  const {site,setMode}=await setup(page,'missing-child');
  await expect(page.locator('[data-configure-category]')).toBeHidden();
  await expect(page.locator('[data-catalog-purchase-notice]')).toBeHidden();
  await expect(page.locator('[data-catalog-usage-notice]')).toHaveText(defaultNotice);
  await expect(page.locator('[data-quote] button')).toHaveText('月：CNY 99.00');

  setMode('empty-purchase');await page.reload();
  await expect(page.locator('[data-configure-category]')).toHaveText('商品分类：弹性云 / 湖北襄阳弹性云');
  await expect(page.locator('[data-catalog-purchase-notice]')).toBeHidden();
  await expect(page.locator('[data-catalog-usage-notice]')).toHaveText('服务器资源仅用于合法合规业务。<script>alert(1)</script>');
  setMode('empty-usage');await page.reload();
  await expect(page.locator('[data-catalog-purchase-notice]')).toHaveText('电信优化与BGP线路，具体规格以当前商品配置为准。');
  await expect(page.locator('[data-catalog-usage-notice]')).toHaveText(defaultNotice);
 });
 test('TC-77c: 分类读取失败展示默认使用提示且报价可用，重试恢复本方内容',async({page})=>{
  const {site,setMode}=await setup(page,'unavailable');
  await expect(page.locator('[data-configure-category]')).toBeHidden();
  await expect(page.locator('[data-catalog-purchase-notice]')).toBeHidden();
  await expect(page.locator('[data-catalog-usage-notice]')).toHaveText(defaultNotice);
  await expect(page.locator('[data-quote] button')).toHaveText('月：CNY 99.00');
  await page.locator('[data-quote] button').first().click();
  await expect(page.locator('[data-add]')).toBeEnabled();
  console.log('TC077 screenshot: '+await site.capture('tc077-category-read-failure'));

  setMode('complete');await page.getByRole('button',{name:'重新加载商品分类',exact:true}).click();
  await expect(page.locator('[data-configure-category]')).toHaveText('商品分类：弹性云 / 湖北襄阳弹性云');
  await expect(page.locator('[data-catalog-purchase-notice]')).toHaveText('电信优化与BGP线路，具体规格以当前商品配置为准。');
  await expect(page.locator('[data-catalog-usage-notice]')).toContainText('服务器资源仅用于合法合规业务。');
 });
});
