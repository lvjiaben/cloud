import {test,expect} from '../support/source-site';
import {test as liveTest} from '@playwright/test';
import {readFile} from 'node:fs/promises';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000}});
test.describe('TC117 电商云解决方案',()=>{
 test('TC117a 原文层级、现有入口、吸顶及本方咨询',async({page},info)=>{
  await page.route('**/x/**',r=>r.fulfill({json:{code:0,data:{items:[],categories:[],products:[]}}}));
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{siteName:'本方测试站点',navigation:[]}}}}));
  const writes:string[]=[],foreign:string[]=[];page.on('request',r=>{if(!['GET','HEAD'].includes(r.method()))writes.push(r.url());if(new URL(r.url()).hostname==='www.xiaobaiyun.cn')foreign.push(r.url());});
  await page.goto('/');
  const menu=page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'解决方案'});await menu.hover();
  const entry=page.locator('.lina-open .Submenu-item').filter({hasText:'电商解决方案'});await expect(entry).toHaveCount(1);await expect(entry).toContainText('了解线上商城与数字化经营的应用场景。');
  await entry.click();await expect(page).toHaveURL('http://localhost:6680/plan/electricity.html');await page.waitForLoadState('networkidle');expect((await page.reload())?.status()).toBe(200);await page.waitForLoadState('networkidle');
  const reference=JSON.parse(await readFile(new URL('../support/electricity-reference.json',import.meta.url),'utf8'));
  const normalize=(s:string)=>s.replace(/\s+/g,' ').trim();
  for(const selector of ['.globalBanner','.advantage-Content','.problem-Content','.architecture-Content','.correlation-Content','.more-Content']){
   expect(normalize(await page.locator('[data-electricity] '+selector).textContent()||'')).toBe(reference.text[selector]);
  }
  const selectors=['.globalBanner h1','.advantage-Content h2','.advantage-Content h3','.problem-Content h3','.problem-Content h5','.architecture-Content h2','.architecture-Content h3','.architecture-Content h5','.architecture-Content h1'];
  for(const selector of selectors)expect(await page.locator('[data-electricity] '+selector).count()).toBe(reference.headings[selector]);
  await expect(page.locator('[data-electricity-products]')).toBeEmpty();await expect(page.locator('.partner-Content img')).toHaveCount(12);
  await expect(page.locator('.sky-footer-container')).toBeVisible();
  expect(await page.evaluate(()=>[...document.styleSheets].some(s=>[...s.cssRules].some(r=>r instanceof CSSKeyframesRule&&r.name==='fadeInUp')))).toBe(true);
  expect(await page.locator('[data-electricity] img').evaluateAll(imgs=>imgs.every(i=>(i as HTMLImageElement).complete&&(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  await page.screenshot({path:info.outputPath('electricity-top.png')});
  expect(await page.locator('.pagebanner-title').evaluate(e=>e.getBoundingClientRect().top)).toBeGreaterThan(62);
  const card=page.locator('.advantage-Content-list-item').first();const shadow=await card.evaluate(e=>getComputedStyle(e).boxShadow);await card.hover();
  await expect.poll(()=>card.evaluate(e=>getComputedStyle(e).boxShadow)).not.toBe(shadow);await page.screenshot({path:info.outputPath('electricity-card-hover.png')});
  const tab=page.locator('.et-hero-tab a').filter({hasText:'解决方案架构'});await tab.click();
  await expect(page.locator('.et-hero-tab.active')).toHaveText('解决方案架构');await expect(page.locator('.et-container')).toHaveClass(/is-stuck/);
  await expect.poll(()=>page.locator('.et-container').evaluate(e=>Math.round(e.getBoundingClientRect().top))).toBe(0);
  await expect.poll(()=>page.locator('#electricity-architecture').evaluate(e=>Math.round(e.getBoundingClientRect().top))).toBe(74);
  await page.screenshot({path:info.outputPath('electricity-architecture.png')});
  await page.evaluate(()=>window.scrollTo(0,0));await expect(page.locator('.et-container')).not.toHaveClass(/is-stuck/);
  await page.locator('[data-public-locale-switch]').click();await expect(page.locator('.pagebanner-title')).toHaveText('E-commerce cloud solution');await expect(page.locator('.advantage-Content h3').first()).toHaveText('Efficient operations');await page.locator('[data-public-locale-switch]').click();
  await page.screenshot({path:info.outputPath('electricity-full.png'),fullPage:true});
  await page.locator('.pagebanner-button a').filter({hasText:'联系我们'}).click();await expect(page).toHaveURL('http://localhost:6680/contact.html');await page.waitForLoadState('networkidle');
  expect(writes).toEqual([]);expect(foreign).toEqual([]);
 });
 test('TC117b 品牌接口失败仍保留静态正文，重试恢复，减少动态效果',async({page},info)=>{
  await page.route('**/x/**',r=>r.fulfill({json:{code:0,data:{items:[],categories:[],products:[]}}}));
  let fail=true;await page.route('**/x/idc-portal/api/v1/public/brand',r=>fail?r.fulfill({status:503,json:{code:50}}):r.fulfill({json:{code:0,data:{brand:{siteName:'本方测试站点',navigation:[]}}}}));
  await page.emulateMedia({reducedMotion:'reduce'});await page.goto('/plan/electricity.html');
  await expect(page.locator('.pagebanner-title')).toHaveText('电商云解决方案');await expect(page.locator('[data-brand-retry]')).toBeVisible();
  await expect(page.locator('.problem-item').first()).toHaveCSS('animation-name','none');
  await page.screenshot({path:info.outputPath('electricity-brand-failure.png')});
  fail=false;await page.locator('[data-brand-retry]').click();await expect(page.locator('.lina-brand-error')).toHaveCount(0);await expect(page.locator('[data-brand-name]').first()).toContainText('本方测试站点');
  await page.locator('.et-hero-tab a').filter({hasText:'面临的问题'}).click();await expect(page.locator('.et-hero-tab.active')).toHaveText('面临的问题');
  await page.screenshot({path:info.outputPath('electricity-recovered.png')});
 });
 liveTest('TC117c 实际6680入口、刷新、资源及本方接口',async({page},info)=>{
  liveTest.setTimeout(90000);
  const errors:string[]=[],foreign:string[]=[];page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(new URL(r.url()).hostname==='www.xiaobaiyun.cn')foreign.push(r.url());});
  await page.goto('http://127.0.0.1:6680/');await page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:'解决方案'}).hover();await page.locator('.lina-open .Submenu-item').filter({hasText:'电商解决方案'}).click();
  await expect(page).toHaveURL('http://127.0.0.1:6680/plan/electricity.html');await page.waitForLoadState('networkidle');expect((await page.reload())?.status()).toBe(200);await page.waitForLoadState('networkidle');
  await expect(page.locator('.pagebanner-title')).toHaveText('电商云解决方案');await expect(page.locator('.lina-brand-error')).toHaveCount(0);
  expect(await page.locator('[data-electricity] img').evaluateAll(imgs=>imgs.every(i=>(i as HTMLImageElement).complete&&(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:info.outputPath('electricity-live-full.png'),fullPage:true});
  await page.locator('.et-hero-tab a').filter({hasText:'面临的问题'}).click();await expect(page.locator('.et-hero-tab.active')).toHaveText('面临的问题');await page.screenshot({path:info.outputPath('electricity-live-problems.png')});
  const brand=await page.request.get('http://127.0.0.1:6680/x/idc-portal/api/v1/public/brand');expect(brand.status()).toBe(200);expect((await brand.json()).data.brand).toBeTruthy();
  expect(errors).toEqual([]);expect(foreign).toEqual([]);
 });
});
