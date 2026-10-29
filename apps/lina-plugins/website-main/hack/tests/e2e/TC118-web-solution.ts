import {test,expect} from '../support/source-site';
import {WebSolutionPage} from '../pages/WebSolutionPage';
import {readFile} from 'node:fs/promises';
import type {Page} from '@playwright/test';
test.use({baseURL:'http://localhost:6680',headless:true});
const sample={publicSlug:'local-web',nameZhCN:'本方发布的服务器',nameEnUS:'Our published server',summaryZhCN:'CPU：4核心；内存：8GB',summaryEnUS:'CPU: 4 cores; RAM: 8GB',availability:'available',defaultConfigurationMonthlyPrice:{currencyCode:'CNY',amountMinor:'12345'}};
async function setup(page:Page,mode:()=>string=()=> 'ready'){
 await page.route('**/x/**',r=>r.fulfill({json:{code:0,data:{items:[],categories:[],products:[]}}}));
 await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'本方站点',enUS:'Our site'},navigation:[],templateConfig:{relatedproductsweb:"local-web"}}}}}));
 await page.route('**/x/idc-core/api/v1/public/catalog/products?**',r=>mode()==='error'?r.fulfill({status:503,json:{code:50}}):r.fulfill({json:{code:0,data:{products:mode()==='empty'?[]:[sample]}}}));
}
test.describe('TC-118 网站解决方案',()=>{
 test('TC-118a: 原文、导航历史、真实字段投影、语言与数据边界',async({page},info)=>{
  await setup(page);const site=new WebSolutionPage(page),errors:string[]=[],forbidden:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{const u=new URL(r.url());if(u.origin!=='http://localhost:6680'||u.pathname.includes('/account/')||!['GET','HEAD'].includes(r.method()))forbidden.push(r.url());});
  await site.enterFromNavigation();await expect(page).toHaveURL(/\/plan\/web.html$/);await expect(page.locator('.et-hero-tab a')).toHaveCount(6);
  const source=JSON.parse(await readFile(new URL('../support/web-solution-reference.json',import.meta.url),'utf8')),normalize=(s:string)=>s.replace(/\s+/g,' ').trim();
  expect(normalize(await page.locator('#web-problems').textContent()||'')).toBe(source.problemText);expect(normalize(await page.locator('#web-more').textContent()||'')).toBe(source.moreText);
  for(const claim of source.withheld)expect(normalize(await page.locator('[data-web-solution]').textContent()||'')).toContain(claim.text);
  await expect(page.locator('.web-pending')).toHaveCount(0);await expect(page.locator('.partner-Content img')).toHaveCount(12);
  await expect(page.locator('.sky-footer-container')).toBeVisible();
  await expect(page.locator('.web-product h3')).toHaveText(sample.nameZhCN);await expect(page.locator('.web-product-price')).toHaveText('默认配置月费 ¥123.45');await expect(page.locator('.web-product-link')).toHaveAttribute('href','/products.html?product=local-web');
  expect(await page.locator('[data-web-solution] img').evaluateAll(images=>images.every(i=>(i as HTMLImageElement).complete&&(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  await site.screenshot(info.outputPath('web-desktop.png'));
  expect(await page.locator('.pagebanner-title').evaluate(e=>e.getBoundingClientRect().top)).toBeGreaterThan(62);
  const card=page.locator('.advantage-Content-list-item').first(),before=await card.evaluate(e=>getComputedStyle(e).boxShadow);await site.hover();await expect.poll(()=>card.evaluate(e=>getComputedStyle(e).boxShadow)).not.toBe(before);await site.screenshot(info.outputPath('web-hover.png'));
  await site.section('problems');await expect(site.nav('problems')).toHaveAttribute('aria-current','location');await expect(page.locator('.problem-item').first()).toHaveClass(/is-visible/);
  await site.section('architecture');await expect(site.nav('architecture')).toHaveAttribute('aria-current','location');await site.screenshot(info.outputPath('web-architecture.png'));
  await page.goBack();await expect(page).toHaveURL(/#web-problems$/);await expect(site.nav('problems')).toHaveAttribute('aria-current','location');await page.goForward();await expect(site.nav('architecture')).toHaveAttribute('aria-current','location');
  await page.waitForLoadState('networkidle');expect((await page.reload())?.status()).toBe(200);await page.waitForLoadState('networkidle');await expect(site.nav('architecture')).toHaveAttribute('aria-current','location');
  await page.locator('#web-products').evaluate(e=>e.scrollIntoView());await expect(site.nav('products')).toHaveAttribute('aria-current','location');
  await page.evaluate(()=>scrollTo(0,0));await site.language();await expect(page.locator('.pagebanner-title')).toHaveText('Website solution');await expect(page.locator('#web-problems .problem-left-title')).toHaveText('Challenges');await expect(page.locator('.web-product h3')).toHaveText(sample.nameEnUS);await expect(page.locator('.web-product-link')).toHaveText('View product');await expect(page).toHaveTitle('Website solution - Our site');
  expect(await page.locator('[data-web-solution]').innerText()).not.toMatch(/\p{Script=Han}/u);
  await expect(page.locator('[data-customer="/account/support"]').first()).toHaveAttribute('href','http://localhost:6032/account/support');
  await page.waitForLoadState('networkidle');await page.reload();await expect(page.locator('.pagebanner-title')).toHaveText('Website solution');await page.waitForLoadState('networkidle');
  expect(errors).toEqual([]);expect(forbidden).toEqual([]);
 });
 test('TC-118b: 手机、减少动态效果、横向导航及全页英文',async({page},info)=>{
  await setup(page);await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});const site=new WebSolutionPage(page);await site.open('#web-problems');
  await expect(site.nav('problems')).toHaveAttribute('aria-current','location');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await expect(page.locator('.web-reveal').first()).toHaveCSS('opacity','1');
  await site.section('partners');await expect(site.nav('partners')).toHaveAttribute('aria-current','location');await site.screenshot(info.outputPath('web-mobile-partners.png'));
  await page.evaluate(()=>scrollTo(0,0));await site.language();await expect(page.locator('.pagebanner-title')).toHaveText('Website solution');await expect(page.locator('#web-advantages .advantage-Content-list-item-content').first()).toContainText('When website users need to choose cloud server configurations');await expect(page.locator('#web-partners .partner-Content-row')).toBeEmpty();
  await page.locator('.lina-menu-toggle').click();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await site.screenshot(info.outputPath('web-mobile-english.png'));
  expect(await page.locator('.pagebanner-title').evaluate(e=>e.getBoundingClientRect().top)).toBeGreaterThan(62);
 });
 test('TC-118c: 商品错误重试、空态和上下架显示',async({page},info)=>{
  let mode='error';await setup(page,()=>mode);const site=new WebSolutionPage(page);await site.open();await expect(page.locator('.web-product-state')).toContainText('商品加载失败，请重试。');await site.section('products');await site.screenshot(info.outputPath('web-products-error.png'));
  mode='empty';await site.retry();await expect(page.locator('.web-product-state')).toHaveText('暂无关联商品。');
  await page.route('**/x/idc-core/api/v1/public/catalog/products?**',r=>r.fulfill({json:{code:0,data:{products:[{...sample,availability:'unavailable',defaultConfigurationMonthlyPrice:null}]}}}));
  await page.reload();await expect(page.locator('.web-product-link')).toHaveText('查看商品');await expect(page.locator('.web-product-link')).toHaveAttribute('href','/products.html?product=local-web');await expect(page.locator('.web-product-price')).toHaveCount(0);await page.waitForLoadState('networkidle');
 });
 test('TC-118d: 键盘导航与品牌失败不影响静态正文',async({page})=>{
  await setup(page);await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({status:503,json:{code:50}}));
  const site=new WebSolutionPage(page);await site.open();await expect(page.locator('.pagebanner-title')).toHaveText('网站解决方案');await expect(page.locator('[data-brand-retry]')).toBeVisible();
  await site.keyboardSection('problems');await expect(page).toHaveURL(/#web-problems$/);await expect(site.nav('problems')).toHaveAttribute('aria-current','location');await expect(page.locator('.problem-item')).toHaveCount(4);
  await page.waitForLoadState('networkidle');
 });
});
