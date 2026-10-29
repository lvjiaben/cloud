import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {CommerceNavigationPage} from '../pages/CommerceNavigationPage';
import type {Page,Route} from '@playwright/test';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-85 分类手风琴与同页面商品配置',()=>{
 async function fixture(page:Page){
  await controlledBusiness(page);
  const categories=[{categoryNo:'child-a',nameZhCN:'弹性云'},{categoryNo:'child-b',nameZhCN:'物理机'}];
  const product={publicSlug:'navigation-product',nameZhCN:'导航测试商品',categoryNo:'child-a',availability:'available',currencies:['CNY']};
  const configuration={configurationRevision:'a'.repeat(43),valid:true,values:[],fields:[]};
  await page.route('**'+prefix+'/catalog/products**',async(route:Route)=>{
   const url=new URL(route.request().url());
   if(url.pathname.endsWith('/quote'))return route.fulfill({json:{code:0,data:{...configuration,currencyCode:'CNY',terms:[]}}});
   if(/\/purchase-configuration(?:\/evaluate)?$/.test(url.pathname))return route.fulfill({json:{code:0,data:configuration}});
   return route.fulfill({json:{code:0,data:url.pathname.endsWith('/products')?{products:[product],categories,nextCursor:''}:{product}}});
  });
  return new CommerceNavigationPage(page);
 }
 test('TC-85a: 整行单开、可全关，展开不更换分类或商品',async({page})=>{
  const site=await fixture(page);await site.observeMotion();await site.open('/products.html?categoryNo=child-a');
  await expect(site.group('弹性云')).toHaveAttribute('aria-expanded','true');
  const before=page.url();await site.toggle('物理机');
  await expect(site.group('弹性云')).toHaveAttribute('aria-expanded','false');
  await expect(site.group('物理机')).toHaveAttribute('aria-expanded','true');
  expect(page.url()).toBe(before);await expect(page.locator('[data-products]')).toContainText('导航测试商品');
  await site.toggle('物理机');await expect(site.group('物理机')).toHaveAttribute('aria-expanded','false');
  await expect(page.locator('.lina-catalog-toggle[aria-expanded=true]')).toHaveCount(0);
  expect(await site.arrowDuration('物理机')).toBe('0.3s');
  expect(await site.motionDurations()).toEqual(expect.arrayContaining([180]));
  const count=(await site.motionDurations()).length;await site.reduceMotion();await site.toggle('弹性云');
  expect((await site.motionDurations()).length).toBe(count);await expect(page.locator('[data-catalog-category=child-a]')).toBeVisible();
 });
 test('TC-85b: 列表、配置、返回前进均不重新加载文档，刷新和旧链接可直达',async({page})=>{
  const site=await fixture(page),documents:string[]=[],errors:string[]=[];
  page.on('request',r=>{if(r.isNavigationRequest()&&r.frame()===page.mainFrame())documents.push(r.url());});
  page.on('pageerror',e=>errors.push(e.message));
  await site.open('/products.html?categoryNo=child-a');await expect(page.locator('[data-products] .product-card')).toHaveCount(1);
  await site.toggle('物理机');await expect(site.group('物理机')).toHaveAttribute('aria-expanded','true');
  await expect(site.group('弹性云')).toHaveAttribute('aria-expanded','false');
  await site.buy();await expect(page.locator('.allocation-header-title h1')).toHaveText('导航测试商品');
  await expect(site.group('物理机')).toHaveAttribute('aria-expanded','true');
  await expect(site.group('弹性云')).toHaveAttribute('aria-expanded','false');
  const categorySidebar=page.locator('.shopping-cart-side [data-categories]'),configuredSidebar=await categorySidebar.innerHTML();
  await expect(site.server('navigation-product')).toHaveAttribute('aria-current','page');
  await expect(site.server('navigation-product')).toBeHidden();
  await expect(page.locator('[data-catalog-category=child-b]')).toBeVisible();
  expect(new URL(page.url()).pathname).toBe('/products.html');await expect(page.locator('[data-page=configure]')).toHaveCount(1);
  await expect(page.locator('.shopping-cart-side .lina-configure-model-list')).toHaveCount(0);
  await expect(page.locator('#addCartForm [data-models]')).toBeVisible();
  await site.browserBack();await expect(page.locator('[data-products] .product-card')).toHaveCount(1);
  await expect(site.group('物理机')).toHaveAttribute('aria-expanded','true');
  await expect(site.group('弹性云')).toHaveAttribute('aria-expanded','false');
  await site.browserForward();await expect(page.locator('.allocation-header-title h1')).toHaveText('导航测试商品');
  await expect(site.group('物理机')).toHaveAttribute('aria-expanded','true');
  await expect(page.locator('.shopping-cart-side [data-categories]')).toHaveJSProperty('innerHTML',configuredSidebar);
  await expect(page.locator('#addCartForm [data-models]')).toBeVisible();
  expect(documents).toHaveLength(1);console.log('TC085 screenshot: '+await site.capture('tc085-configure',false));
  await site.reload();await expect(page.locator('.allocation-header-title h1')).toHaveText('导航测试商品');
  await site.open('/configure.html?product=navigation-product');await expect(page.locator('.allocation-header-title h1')).toHaveText('导航测试商品');
  await expect(page.locator('link[href$="lina/configuration-controls.css"]')).toHaveCount(1);
  expect(new URL(page.url()).pathname).toBe('/products.html');expect(errors).toEqual([]);
 });
 test('TC-85c: 视图片段失败可重试，配置失败不准加车，离开后旧请求不覆盖目录',async({page})=>{
  const site=await fixture(page);let fragmentFails=true,detailFails=true,hold=false,waiting=false,release=()=>{};
  const blocked=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/lina/configure-panel.html',route=>fragmentFails?route.fulfill({status:503,body:''}):route.fallback());
  await page.route('**'+prefix+'/catalog/products/navigation-product',async route=>{
   if(detailFails)return route.fulfill({status:503,json:{code:50}});
   if(hold){waiting=true;await blocked;}
   return route.fallback();
  });
  await site.open('/products.html?categoryNo=child-a');await site.buy();
  await expect(page.locator('[data-status]')).toHaveText('页面暂时无法加载，请重试。');
  expect(new URL(page.url()).searchParams.has('action')).toBe(false);
  fragmentFails=false;await site.retry();await expect(page.locator('[data-status]')).toHaveText('无法读取商品配置，请重试。');
  await expect(page.locator('[data-add]')).toBeDisabled();console.log('TC085 screenshot: '+await site.capture('tc085-configure-failure',false));
  detailFails=false;hold=true;
  try{await site.refreshConfiguration();await expect.poll(()=>waiting).toBe(true);await site.backToCatalog();await expect(page.locator('[data-products] .product-card')).toHaveCount(1);}finally{release();}
  await expect(page.locator('[data-status]')).toHaveText('');await expect(page.locator('#addCartForm')).toHaveCount(0);
  await site.setViewport(390,844);expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(390);
  console.log('TC085 screenshot: '+await site.capture('tc085-catalog-mobile',false));
 });
});
