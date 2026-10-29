import {test,expect} from '../support/source-site';
import {AffiliatePage} from '../pages/AffiliatePage';
import {readFile} from 'node:fs/promises';
test.use({baseURL:'http://localhost:6680'});
test.describe('TC-113 联盟推广介绍页',()=>{
 test('TC-113a: 桌面动画、锚点历史、翻译与公开数据边界',async({page},info)=>{
  const site=new AffiliatePage(page),errors:string[]=[],privateCalls:string[]=[],foreign:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',r=>{const u=new URL(r.url());if(u.pathname.includes('/account/'))privateCalls.push(u.pathname);if(u.origin!=='http://localhost:6680')foreign.push(r.url());});
  await site.open();await expect(page.locator('.aff-nav a')).toHaveCount(5);
  await expect(page.locator('.aff-hero h1')).toHaveText('分享云服务，连接更多可能');
  await expect(page.locator('[data-aff-customer]').first()).toHaveAttribute('href','http://localhost:6032/account/referrals');
  await expect.poll(()=>page.locator('.aff-hero-art').evaluate(e=>(e as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
  await expect(page.locator('.aff-hero-art')).toHaveCSS('animation-name','aff-float');
  await site.screenshot(info.outputPath('affiliate-desktop.png'));
  await site.section('advantages');await expect(page.locator('.aff-card').first()).toHaveCSS('opacity','1');
  await site.hover();await expect(page.locator('.aff-card').first()).toHaveCSS('translate','0px -4px');await site.screenshot(info.outputPath('affiliate-hover.png'));
  await site.section('process');await expect(site.nav('process')).toHaveAttribute('aria-current','location');await expect(page.locator('.aff-step')).toHaveCount(4);await expect(page.locator('.aff-step').last()).toHaveClass(/is-visible/);await site.screenshot(info.outputPath('affiliate-process.png'));
  await site.section('rules');await expect(page.locator('#rules')).toContainText('公开规则尚未发布');
  await page.goBack();await expect(page).toHaveURL(/#process$/);await expect(site.nav('process')).toHaveAttribute('aria-current','location');
  await page.goForward();await expect(page).toHaveURL(/#rules$/);await page.waitForLoadState('networkidle');await page.reload();await page.waitForLoadState('networkidle');await expect(site.nav('rules')).toHaveAttribute('aria-current','location');
  await page.locator('#methods').evaluate(e=>e.scrollIntoView());await expect(site.nav('methods')).toHaveAttribute('aria-current','location');
  await site.section('faq');await expect(page.locator('.aff-question')).toHaveCount(3);await site.screenshot(info.outputPath('affiliate-faq.png'));
  await site.language();await expect(page.locator('#rules h2')).toHaveText('Understand the rules before sharing');await expect(page.locator('#faq h2')).toHaveText('A few things you may want to know');await expect(page.locator('[data-aff-customer]').first()).toHaveText('Get referral code');
  await page.waitForLoadState('networkidle');expect(privateCalls).toEqual([]);expect(foreign).toEqual([]);expect(errors).toEqual([]);
 });
 test('TC-113b: 手机、减少动态效果与未配置客户中心',async({page},info)=>{
  await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});
  const site=new AffiliatePage(page);await site.open('#methods');await expect(site.nav('methods')).toHaveAttribute('aria-current','location');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await expect(page.locator('.aff-hero-art')).toHaveCSS('animation-name','none');await expect(page.locator('.aff-method').first()).toHaveCSS('opacity','1');
  await site.screenshot(info.outputPath('affiliate-mobile.png'));await site.section('faq');await expect(site.nav('faq')).toHaveAttribute('aria-current','location');await page.waitForLoadState('networkidle');
  await site.language();await expect(page.locator('#faq h2')).toHaveText('A few things you may want to know');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 });
 test('TC-113c: 未配置客户中心时不伪造跳转',async({page})=>{
  await page.route('**/aff.html',async route=>{const body=await readFile(new URL('../../../frontend/public/aff.html',import.meta.url),'utf8');await route.fulfill({contentType:'text/html',body});});
  // Clear only this isolated test browser context's injected runtime configuration.
  await page.addInitScript(()=>{Object.defineProperty(window,'__LINA_TEMPLATE__',{value:{},writable:false});});
  await page.route('**/lina/runtime-config.json',route=>route.fulfill({status:404,body:''}));
  const site=new AffiliatePage(page);await site.open();await expect(page.locator('[data-aff-customer]').first()).toHaveAttribute('aria-disabled','true');await expect(page.locator('[data-aff-customer]').first()).not.toHaveAttribute('href',/.+/);
 });
});
