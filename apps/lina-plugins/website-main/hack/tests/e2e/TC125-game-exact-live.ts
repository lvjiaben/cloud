import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {GameSolutionPage} from '../pages/GameSolutionPage';

test.use({baseURL:'http://127.0.0.1:6680',headless:true});
const reference=JSON.parse(await readFile(new URL('../support/game-copy-reference.json',import.meta.url),'utf8'));
// HTML parsing normalizes CRLF; preserve every other character and space.
const copyReference=reference.copy.map((item:{text:string})=>({...item,text:item.text.replace(/\r\n?/g,'\n')}));

test.describe('TC-125 游戏页实际入口与附件原文',()=>{
 test('TC-125a: 实际HTTP、53项原文与层级、菜单标签和桌面手机锚点',async({page},info)=>{
  test.setTimeout(90000);
  const errors:string[]=[],brandCalls:string[]=[],foreign:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',request=>{
   const url=new URL(request.url());
   if(url.pathname==='/x/idc-portal/api/v1/public/brand')brandCalls.push(url.href);
   if(/xiaobaiyun|baiyuan/i.test(url.hostname))foreign.push(url.href);
  });
  await page.addInitScript(()=>localStorage.setItem('lina.website.locale','zh-CN'));
  const site=new GameSolutionPage(page),response=await site.open();
  expect(response?.status()).toBe(200);
  await site.ready();
  expect(await site.copy()).toEqual(copyReference);
  expect(brandCalls).toHaveLength(1);
  await expect(page.locator('script[type=module][src="/lina/site.js"]')).toHaveCount(1);
  expect(await page.evaluate(async()=>{
   const module=await import(new URL('/lina/game.js',location.href).href);
   return module.initializeGame(document)===module.initializeGame(document);
  })).toBe(true);
  await expect(page.locator('[data-game] main')).toHaveCount(1);
  expect(await page.locator('[data-game] img[src]').count()).toBe(21);
  await expect(page.locator('.game-partner-image-slot')).toHaveCount(12);
  const images=JSON.parse(await readFile(new URL('../support/game-image-reference.json',import.meta.url),'utf8'));
  expect(await page.locator('[data-game] img[src],.sky-footer-top img[src]').evaluateAll(items=>items.map(item=>item.getAttribute('src')))).toEqual(images.images);
  await expect.poll(()=>page.locator('[data-game] img[src],.sky-footer-top img[src]').evaluateAll(items=>items.every(item=>(item as HTMLImageElement).complete&&(item as HTMLImageElement).naturalWidth>0))).toBe(true);
  for(const src of images.backgrounds){
   const asset=await page.request.get(src);expect(asset.ok(),src).toBe(true);
   expect(asset.headers()['content-type']).toMatch(/^image\//);
  }
  await expect(page.locator('.globalBanner')).toHaveCSS('background-image',/remoteL17139270745759/);
  await expect(page.locator('.more-item-img').first()).toBeVisible();
  await expect(page.locator('.sky-footer-icon').first()).toHaveCSS('filter','brightness(0) invert(1)');
  const partnerColumns=()=>page.locator('.game-partner-image-slot').evaluateAll(items=>items.filter(item=>Math.abs(item.getBoundingClientRect().top-items[0].getBoundingClientRect().top)<1).length);
  expect(await partnerColumns()).toBe(6);
  await expect(page.locator('.correlation-Content-row .web-product-state')).toHaveText('暂无关联商品。');
  await site.screenshot(info.outputPath('game-live-desktop.png'));
  await site.menu('解决方案');
  await expect(page.locator('.lina-open .Submenu-menuInfo-title')).toHaveJSProperty('tagName','H1');
  await expect(page.locator('.lina-open .Submenu-menuInfo>.Submenu-desc')).toHaveText(reference.navigation[1].description);
  const groups=reference.navigation[1].children;
  for(const group of groups){
   await site.menuTab(group.label);
   await expect(page.locator('.lina-open .lina-nav-category-view:not([hidden]) .Submenu-title')).toHaveText(group.children.map((item:{label:string})=>item.label));
   expect(await page.locator('.lina-open .lina-nav-category-view:not([hidden]) .Submenu-desc').allTextContents()).toEqual(group.children.map((item:{description:string})=>item.description));
  }
  await site.screenshot(info.outputPath('game-live-menu.png'));
  await site.escapeMenu();
  for(const id of ['advantages','problems','architecture','products','partners','more']){
   await site.section(id);
   await expect(page.locator('.et-hero-tab a[href="#game-'+id+'"]')).toHaveAttribute('aria-current','location');
  }
  await site.section('architecture');
  await expect(page.locator('.architecture-Content-right')).toBeVisible();
  await site.screenshot(info.outputPath('game-live-architecture.png'));
  await site.section('partners');await site.screenshot(info.outputPath('game-live-partners.png'));
  await site.section('more');await site.screenshot(info.outputPath('game-live-more-footer.png'));
  await page.setViewportSize({width:390,height:844});
  await site.open();await site.ready();
  expect(await site.copy()).toEqual(copyReference);
  await expect.poll(()=>page.locator('[data-game] img[src]').evaluateAll(items=>items.every(item=>(item as HTMLImageElement).complete&&(item as HTMLImageElement).naturalWidth>0))).toBe(true);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await site.screenshot(info.outputPath('game-live-mobile.png'));
  for(const id of ['advantages','problems','architecture','products','partners','more'])await site.section(id);
  expect(await partnerColumns()).toBe(3);
  await site.section('partners');await site.screenshot(info.outputPath('game-live-mobile-partners.png'));
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  expect(foreign).toEqual([]);expect(errors).toEqual([]);
 });

 test('TC-125b: 品牌失败保留原文，重新读取后仅展示本方资料',async({page},info)=>{
  let fail=true;
  const brand={siteName:{zhCN:'验收站点'},footer:{zhCN:'验收公司'},copyright:'验收版权',contactPhone:'01012345678',contactEmail:'contact@example.invalid',businessQq:'123456',companyAddress:'验收地址',icpNumber:'本方备案',publicSecurityNumber:'本方公安备案',navigation:[],headerLogoUrl:'',footerLogoUrl:''};
  await page.route('**/x/idc-portal/api/v1/public/brand',route=>route.fulfill(fail?{status:503,json:{code:50}}:{json:{code:0,data:{brand}}}));
  const site=new GameSolutionPage(page);await site.open();await site.ready();
  await expect(page.locator('[data-brand-retry]')).toBeVisible();
  expect(await site.copy()).toEqual(copyReference);
  expect(await page.locator('body').innerText()).not.toMatch(/小白云|江苏白猿|1531335070|400-8876675|苏ICP备/);
  await site.screenshot(info.outputPath('game-live-brand-failed.png'));
  fail=false;await site.retryBrand();
  await expect(page.locator('[data-brand-footer]')).toHaveText('验收公司');
  await expect(page.locator('[data-footer-contact=phone] a')).toHaveText(brand.contactPhone);
  await expect(page.locator('[data-footer-copyright]')).toHaveText(brand.copyright);
  await expect(page.locator('[data-filing-kind=icp] .filing-title')).toHaveText(brand.icpNumber);
  await expect(page.locator('.horizontal-menu-navbar-logo [data-brand-name]')).toBeEmpty();
  await expect(page.locator('[data-game-contact]').first()).toHaveAttribute('href','http://127.0.0.1:6680/contact.html');
  await expect(page.locator('[data-brand-retry]')).toHaveCount(0);
 });
});
