import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {GoingseaSolutionPage} from '../pages/GoingseaSolutionPage';
test.use({baseURL:'http://127.0.0.1:6680',headless:true});
const reference=JSON.parse(await readFile(new URL('../support/goingsea-copy-reference.json',import.meta.url),'utf8'));
const copy=reference.copy.map((item:{text:string})=>({...item,text:item.text.replace(/\r\n?/g,'\n')}));
const imageReference=JSON.parse(await readFile(new URL('../support/goingsea-image-reference.json',import.meta.url),'utf8'));
test.describe('TC-128 出海页实际入口与原站独立基准',()=>{
 for(const mobile of [false,true])test(mobile?'TC-128b: 手机原文层级、图片和六锚点':'TC-128a: 桌面原文层级、图片和本方导航',async({page},info)=>{
  test.setTimeout(90000);
  const errors:string[]=[],foreign:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',r=>{if(/xiaobaiyun|baiyuan/i.test(new URL(r.url()).hostname))foreign.push(r.url());});
  await page.addInitScript(()=>localStorage.setItem('lina.website.locale','zh-CN'));
  if(mobile)await page.setViewportSize({width:390,height:844});
  const site=new GoingseaSolutionPage(page),response=await site.open();await site.ready();
  expect(response?.status()).toBe(200);expect(await site.copy()).toEqual(copy);
  await expect(page.locator('.architecture-advantage h1')).toHaveText('架构优势');
  await expect(page.locator('.correlation-Content-row')).toBeEmpty();
  const descriptions=page.locator('.problem-item-content');
  for(const description of await descriptions.all()){
   await expect(description).toHaveCSS('-webkit-line-clamp','none');
   expect(await description.evaluate(e=>e.scrollHeight<=e.clientHeight+1)).toBe(true);
  }
  if(mobile)for(const title of await page.locator('.sky-footer-title').all())expect(await title.evaluate(e=>e.getBoundingClientRect().height)).toBeLessThanOrEqual(48);
  expect(await page.locator('[data-goingsea] img,.sky-footer-top img').evaluateAll(items=>items.map(i=>i.getAttribute('src')))).toEqual(imageReference.images.map((i:{local:string})=>i.local));
  await expect.poll(()=>page.locator('[data-goingsea] img,.sky-footer-top img').evaluateAll(items=>items.every(i=>(i as HTMLImageElement).complete&&(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  await expect(page.locator('.globalBanner')).toHaveCSS('background-image',/goingsea-assets\/remoteL17139312433523/);
  const backgrounds=[['.partner_bg','::before','remoteL17138735335796'],['.problem-Content-left',null,'remoteL17063678634493'],['.problem-left-ul li', '::before','a_select'],['.problem-item',null,'remoteL17063679119861'],['.architecture-left-ul li','::before','remoteL17128148186845'],['.more-Content-row',null,'remoteL17063678634493']];
  for(const [selector,pseudo,name]of backgrounds)expect(await page.locator(selector!).first().evaluate((e,pseudo)=>getComputedStyle(e,pseudo).backgroundImage,pseudo)).toContain(name);
  await site.screenshot(info.outputPath('goingsea-'+(mobile?'mobile':'desktop')+'-initial.png'));
  for(const id of ['advantages','problems','architecture','products','partners','more']){
   await site.section(id);
   await expect(page.locator('.et-hero-tab a[href="#goingsea-'+id+'"]')).toHaveAttribute('aria-current','location');
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
   await site.screenshot(info.outputPath('goingsea-'+(mobile?'mobile':'desktop')+'-'+id+'.png'));
  }
  await expect(page.locator('.more-Content-item').nth(4)).toHaveAttribute('href','/plan/safety.html');
  await expect(page.locator('.more-Content-item').nth(3)).toHaveAttribute('href','/404.html');
  await site.menu('解决方案');
  await expect(page.locator('.lina-open a[href$="/plan/Goingtosea.html"]')).toHaveCount(1);
  await site.screenshot(info.outputPath('goingsea-menu.png'));
  await site.escapeMenu();
  expect(await page.evaluate(async()=>{const m=await import(new URL('/lina/goingsea.js',location.href).href);return m.initializeGoingsea(document)===m.initializeGoingsea(document);})).toBe(true);
  expect(errors).toEqual([]);expect(foreign).toEqual([]);
 });
 test('TC-128c: 品牌失败仍保留原文及图片，重试后只呈现本方身份',async({page},info)=>{
  let failed=true;
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill(failed?{status:503,json:{code:50}}:{json:{code:0,data:{brand:{siteName:{zhCN:'出海验收站点'},footer:{zhCN:'本方出海公司'},contactPhone:'01012345678',copyright:'本方版权',icpNumber:'本方备案',navigation:[]}}}}));
  const site=new GoingseaSolutionPage(page);await site.open();await site.ready();
  await expect(page.locator('[data-brand-retry]')).toBeVisible();expect(await site.copy()).toEqual(copy);
  await expect.poll(()=>page.locator('[data-goingsea] img').evaluateAll(items=>items.every(i=>(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  await site.screenshot(info.outputPath('goingsea-brand-failed.png'));
  failed=false;await site.retryBrand();
  await expect(page.locator('[data-brand-footer]')).toHaveText('本方出海公司');
  await expect(page.locator('[data-footer-contact=phone] a')).toHaveText('01012345678');
  await expect(page.locator('.horizontal-menu-navbar-logo [data-brand-name]')).toHaveText('出海验收站点');
  await expect(page.locator('[data-goingsea-contact]').first()).toHaveAttribute('href','http://127.0.0.1:6680/contact.html');
  expect(await page.locator('body').innerText()).not.toMatch(/小白云|江苏白猿|1531335070|400-8876675/);
  await site.screenshot(info.outputPath('goingsea-brand-recovered.png'));
 });
});
