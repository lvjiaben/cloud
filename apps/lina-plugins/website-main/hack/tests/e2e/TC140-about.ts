import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {parseGameSource} from '../support/game-copy.mjs';
import {collectAboutSource} from '../support/about-copy.mjs';
import {AboutPage} from '../pages/AboutPage';
const reference=collectAboutSource(parseGameSource(await readFile(new URL('../fixtures/about/source.html',import.meta.url),'utf8')));
test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});
test.describe('TC-140 公司简介附件还原与空简介格式',()=>{
 for(const mobile of [false,true])test('TC-140'+(mobile?'b':'a')+': 原文层级、全部卡片与锚点、图片及长文布局 '+(mobile?'手机':'桌面'),async({page},info)=>{
  test.setTimeout(90000);if(mobile)await page.setViewportSize({width:390,height:844});
  const errors:string[]=[],foreign:string[]=[];page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(/xiaobaiyun\.cn/.test(r.url()))foreign.push(r.url());});
  const site=new AboutPage(page);expect((await site.open())?.status()).toBe(200);expect(await site.copy()).toEqual(reference);
  await expect(page.locator('[data-about-introduction]')).toBeEmpty();await expect(page.locator('[data-about-metric]')).toHaveText(['','','','']);await expect(page.locator('[data-about-company]')).not.toContainText('白猿');
  expect(await site.initializeAgain()).toBe(true);await site.screenshot(info.outputPath('hero.png'));
  expect(await page.locator('.pagebanner-description').evaluate(e=>e.scrollHeight<=e.clientHeight+1)).toBe(true);
  expect(await page.locator('.globalBanner').evaluate(e=>{const banner=e.getBoundingClientRect(),title=e.querySelector('h1')!.getBoundingClientRect(),description=e.querySelector('.pagebanner-description')!.getBoundingClientRect();return title.top>=banner.top&&description.bottom<=banner.bottom;})).toBe(true);
  const ids=['briefIntroduction','culture','programme'];for(let i=0;i<ids.length;i++){await site.anchor(i);await expect(page.locator('.et-hero-tab').nth(i)).toHaveClass(/active/);const top=await page.locator('#'+ids[i]).evaluate(e=>e.getBoundingClientRect().top);expect(top).toBeGreaterThanOrEqual(116);expect(top).toBeLessThan(160);await site.screenshot(info.outputPath(ids[i]+'.png'));}
  for(let i=0;i<4;i++){await site.expand(i,mobile);const card=page.locator('.programme-content-item').nth(i);await expect(card).toHaveClass(/active/);await expect(card.locator('.programme-content-item-desc')).toBeVisible();await expect(page.locator('.programme-content-item-desc:visible')).toHaveCount(1);expect(await card.locator('.programme-content-item-desc').evaluate(e=>e.scrollHeight<=e.clientHeight+1)).toBe(true);await site.screenshot(info.outputPath('card-'+i+'.png'));}
  expect(await site.copy()).toEqual(reference);const links=await page.locator('.programme-content-item-button a').evaluateAll(es=>es.map(e=>e.getAttribute('href')));expect(links).toEqual(['/plan/electricity.html','/plan/game.html','/plan/safety.html','/plan/web.html']);for(const link of links)expect((await page.request.get(link!)).status()).toBe(200);
  const course=page.locator('.pagebanner-button a').nth(1);await expect(course).toHaveAttribute('href','/404.html?feature=about-course');expect((await page.request.get('/404.html?feature=about-course')).status()).toBe(404);
  expect(await page.locator('main.et-main img').evaluateAll(es=>es.every((e:HTMLImageElement)=>e.complete&&e.naturalWidth>0&&e.clientWidth>0&&e.clientHeight>0&&(Math.abs(e.clientWidth/e.clientHeight-e.naturalWidth/e.naturalHeight)<.03||getComputedStyle(e).objectFit==='cover')))).toBe(true);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  expect(await page.locator('#culture .culture-Text').evaluateAll(es=>es.every(e=>e.scrollWidth<=e.clientWidth+1&&e.scrollHeight<=e.clientHeight+1))).toBe(true);
  await expect(page.locator('[data-about-consult]')).toHaveAttribute('href',/\/contact\.html$/);
  await site.injectLongCopy();await site.section('briefIntroduction');expect(await page.locator('.companyIntroduction-right-content p').evaluateAll(es=>es.every(e=>e.scrollHeight<=e.clientHeight+1))).toBe(true);expect(await page.evaluate(()=>document.querySelector('#culture')!.getBoundingClientRect().top>=document.querySelector('.companyIntroduction-right-content')!.getBoundingClientRect().bottom)).toBe(true);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await site.screenshot(info.outputPath('long-copy-test-only.png'));await page.reload();await expect(page.locator('[data-about-introduction]')).toBeEmpty();expect(errors).toEqual([]);expect(foreign).toEqual([]);
 });
 test('TC-140c: 品牌接口失败、手动重试恢复且不污染空简介',async({page},info)=>{
  let fail=true;await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill(fail?{status:503,json:{code:50}}:{json:{code:0,data:{brand:{siteName:{zhCN:'本方网站'},footer:{zhCN:'本方公司'},contactPhone:'01012345678',navigation:[],footerNavigation:[]}}}}));
  const site=new AboutPage(page);await site.open();await expect(page.locator('[data-brand-retry]')).toBeVisible();await expect(page.locator('[data-about-company]')).toBeEmpty();expect(await site.copy()).toEqual(reference);await site.screenshot(info.outputPath('brand-failed.png'));
  fail=false;await site.retry();await expect(page.locator('[data-brand-retry]')).toHaveCount(0);await expect(page.locator('[data-about-company]')).toHaveText('本方公司');await expect(page).toHaveTitle('公司简介 - 本方网站');await expect(page.locator('[data-about-introduction]')).toBeEmpty();await expect(page.locator('[data-about-metric]')).toHaveText(['','','','']);await site.screenshot(info.outputPath('brand-recovered.png'));
 });
 for(const mobile of [false,true])test('TC-140d: 首页菜单与页脚统一复用about路径 '+(mobile?'手机':'桌面'),async({page},info)=>{
  test.setTimeout(90000);if(mobile)await page.setViewportSize({width:390,height:844});const site=new AboutPage(page);await page.goto('/');await site.menu();await expect(page.locator('.lina-open a').filter({has:page.getByText('公司简介',{exact:true})})).toHaveAttribute('href',/\/about\.html$/);await site.screenshot(info.outputPath('home-menu.png'));await site.enter();await expect(page).toHaveURL(/\/about\.html$/);expect(await site.copy()).toEqual(reference);await expect(page.locator('.sky-footer-nav-item a').filter({hasText:'公司简介'})).toHaveAttribute('href','/about.html');
 });
 test('TC-140e: 历史配置目标统一、已有金融菜单入口与键盘卡片',async({page})=>{
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'本方网站'},footer:{zhCN:'本方公司'},navigation:[{label:{zhCN:'了解我们'},children:[{label:{zhCN:'公司简介'},href:'/Introduction.html'}]}],footerNavigation:[{label:{zhCN:'公司简介'},href:'/Introduction.html'}]}}}}));
  const site=new AboutPage(page);await page.goto('/plan/financial.html');await site.menu();await expect(page.locator('.lina-open a').filter({has:page.getByText('公司简介',{exact:true})})).toHaveAttribute('href',/\/about\.html$/);await site.enter();await expect(page.locator('.sky-footer-nav-item a').filter({hasText:'公司简介'})).toHaveAttribute('href','/about.html');
  const card=page.locator('.programme-content-item').nth(3);await card.focus();await card.press('Enter');await expect(card.locator('.programme-content-item-desc')).toBeVisible();await expect(page.locator('.programme-content-item.active')).toHaveCount(1);
 });
 test('TC-140f: 全部本地展示图和背景可解码',async({page})=>{
  const assets=JSON.parse(await readFile(new URL('../../../docs/about-assets-20261025.json',import.meta.url),'utf8'));const site=new AboutPage(page);await site.open();
  const results=await page.evaluate(async files=>Promise.all(files.map(async file=>{const img=new Image();img.src='/'+file;try{await img.decode();return {file,loaded:img.naturalWidth>0&&img.naturalHeight>0};}catch{return {file,loaded:false};}})),assets.map(a=>a.file));expect(results.filter(r=>!r.loaded)).toEqual([]);
 });
});
