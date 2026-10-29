import {test,expect} from '@playwright/test';
import {WebSolutionPage} from '../pages/WebSolutionPage';
test.use({baseURL:'http://127.0.0.1:6680',headless:true});
test.describe('TC-119 网站解决方案实际部署',()=>{
 test('TC-119a: 真实公开接口、刷新、资源、按钮和中英文',async({page},info)=>{
  test.setTimeout(90000);
  const site=new WebSolutionPage(page),errors:string[]=[],forbidden:string[]=[],broken:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{const u=new URL(r.url());if(u.origin!=='http://127.0.0.1:6680'||u.pathname.includes('/account/')||!['GET','HEAD'].includes(r.method()))forbidden.push(r.url());});
  page.on('response',r=>{if(r.status()>=400)broken.push(r.url());});
  await site.enterFromNavigation();await expect(page).toHaveURL(/\/plan\/web.html$/);await expect(page.locator('.pagebanner-title')).toHaveText('网站解决方案');await expect(page.locator('.lina-brand-error')).toHaveCount(0);
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=3');expect(response.status()).toBe(200);const {data}=await response.json();
  expect(await page.locator('.web-product h3').allTextContents()).toEqual(data.products.map((p:any)=>p.nameZhCN));
  await expect(page.locator('.problem-Content a').first()).toHaveAttribute('href','/contact.html');
  const config=await page.request.get('/lina/runtime-config.json');expect(config.status()).toBe(200);const {customerUrl}=await config.json();await expect(page.locator('[data-customer="/account/support"]').first()).toHaveAttribute('href',new URL('/account/support',customerUrl).href);await expect(page.locator('[data-customer="/account/support"]').first()).not.toHaveAttribute('title',/尚未配置/);
  expect(await page.locator('[data-web-solution] img').evaluateAll(images=>images.every(i=>(i as HTMLImageElement).complete&&(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  await site.screenshot(info.outputPath('web-live-desktop.png'));await site.section('problems');await expect(site.nav('problems')).toHaveAttribute('aria-current','location');await site.screenshot(info.outputPath('web-live-problems.png'));
  expect((await page.reload())?.status()).toBe(200);await page.waitForLoadState('networkidle');await expect(site.nav('problems')).toHaveAttribute('aria-current','location');
  await site.section('products');await site.screenshot(info.outputPath('web-live-products.png'));
  await page.evaluate(()=>scrollTo(0,0));await site.language();await expect(page.locator('.pagebanner-title')).toHaveText('Website solution');expect(await page.locator('[data-web-solution]').innerText()).not.toMatch(/\p{Script=Han}/u);
  await site.language();await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});await site.open();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await site.screenshot(info.outputPath('web-live-mobile.png'));
  expect(errors).toEqual([]);expect(forbidden).toEqual([]);expect(broken).toEqual([]);
 });
});
