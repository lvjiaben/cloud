import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {SafetySolutionPage} from '../pages/SafetySolutionPage';

test.use({baseURL:'http://localhost:6680',headless:true});
const reference=JSON.parse(await readFile(new URL('../support/safety-copy-reference.json',import.meta.url),'utf8'));
// HTML parsing normalizes CRLF; preserve every other character and space.
const copyReference=reference.copy.map((item:{text:string})=>({...item,text:item.text.replace(/\r\n?/g,'\n')}));

test.describe('TC-127 安全页实际入口与附件原文',()=>{
 test('TC-127a: 实际HTTP、52项原文与层级、菜单标签和桌面手机锚点',async({page},info)=>{
  test.setTimeout(90000);
  const errors:string[]=[],brandCalls:string[]=[],foreign:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',request=>{
   const url=new URL(request.url());
   if(url.pathname==='/x/idc-portal/api/v1/public/brand')brandCalls.push(url.href);
   if(/xiaobaiyun|baiyuan/i.test(url.hostname))foreign.push(url.href);
  });
  await page.addInitScript(()=>localStorage.setItem('lina.website.locale','zh-CN'));
  await page.route('**/x/**',r=>r.fulfill({json:{code:0,data:{items:[],categories:[],products:[]}}}));
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'验收站点'},navigation:[]}}}}));
  const site=new SafetySolutionPage(page),response=await site.open();
  expect(response?.status()).toBe(200);
  await site.ready();
  expect(await site.copy()).toEqual(copyReference);
  expect(brandCalls).toHaveLength(1);
  await expect(page.locator('script[type=module][src="/lina/site.js"]')).toHaveCount(1);
  expect(await page.evaluate(async()=>{
   const module=await import(new URL('/lina/safety.js',location.href).href);
   return module.initializeSafety(document)===module.initializeSafety(document);
  })).toBe(true);
  await expect(page.locator('[data-safety] main')).toHaveCount(1);
  expect(await page.locator('[data-safety] img[src]').count()).toBe(21);
  await expect(page.locator('.partner-Content-logo-item img[src^="/lina/safety-assets/"]')).toHaveCount(12);
  await expect.poll(()=>page.locator('[data-safety] img[src]').evaluateAll(items=>items.every(item=>(item as HTMLImageElement).complete&&(item as HTMLImageElement).naturalWidth>0))).toBe(true);
  await expect(page.locator('.correlation-Content-row')).toBeEmpty();
  await site.screenshot(info.outputPath('safety-live-desktop.png'));
  await site.menu('解决方案');
  await expect(page.locator('.lina-open .Submenu-menuInfo-title')).toHaveJSProperty('tagName','H1');
  await expect(page.locator('.lina-open .Submenu-menuInfo>.Submenu-desc')).toHaveText(reference.navigation[1].description);
  const groups=reference.navigation[1].children;
  for(const group of groups){
   await site.menuTab(group.label);
   await expect(page.locator('.lina-open .lina-nav-category-view:not([hidden]) .Submenu-title')).toHaveText(group.children.map((item:{label:string})=>item.label));
   expect(await page.locator('.lina-open .lina-nav-category-view:not([hidden]) .Submenu-desc').allTextContents()).toEqual(group.children.map((item:{description:string})=>item.description));
  }
  await expect(page.locator('.lina-open .Submenu-item').filter({hasText:'安全解决方案'})).toHaveAttribute('href','http://localhost:6680/plan/safety.html');
  await site.screenshot(info.outputPath('safety-live-menu.png'));
  await site.escapeMenu();
  for(const id of ['advantages','problems','architecture','products','partners','more']){
   await site.section(id);
   await expect(page.locator('.et-hero-tab a[href="#safety-'+id+'"]')).toHaveAttribute('aria-current','location');
  }
  await site.section('architecture');
  await expect(page.locator('.architecture-Content-right')).toBeVisible();
  await site.screenshot(info.outputPath('safety-live-architecture.png'));
  await page.setViewportSize({width:390,height:844});
  await site.open();await site.ready();
  expect(await site.copy()).toEqual(copyReference);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await site.screenshot(info.outputPath('safety-live-mobile.png'));
  await site.menu('解决方案');await site.menuTab(reference.navigation[1].children[0].label);
  await expect(page.locator('.lina-open .lina-nav-category-view:not([hidden])')).toBeVisible();
  await site.escapeMenu();
  expect(await page.locator('.more-Content-item').evaluateAll(items=>items.map(e=>e.getAttribute('href')))).toEqual(['/plan/electricity.html','/plan/web.html','/plan/game.html','/404.html','/plan/safety.html']);
  for(const id of ['advantages','problems','architecture','products','partners','more'])await site.section(id);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  expect(foreign).toEqual([]);expect(errors).toEqual([]);
 });

 test('TC-127b: 品牌失败保留原文，重新读取后仅展示本方资料',async({page},info)=>{
  await page.route('**/x/**',r=>r.fulfill({json:{code:0,data:{items:[],categories:[],products:[]}}}));
  let fail=true;
  const brand={siteName:{zhCN:'验收站点'},footer:{zhCN:'验收公司'},copyright:'验收版权',contactPhone:'01012345678',contactEmail:'contact@example.invalid',businessQq:'123456',companyAddress:'验收地址',icpNumber:'本方备案',publicSecurityNumber:'本方公安备案',navigation:[],headerLogoUrl:'',footerLogoUrl:''};
  await page.route('**/x/idc-portal/api/v1/public/brand',route=>route.fulfill(fail?{status:503,json:{code:50}}:{json:{code:0,data:{brand}}}));
  const site=new SafetySolutionPage(page);await site.open();await site.ready();
  await expect(page.locator('[data-brand-retry]')).toBeVisible();
  expect(await site.copy()).toEqual(copyReference);
  expect(await page.locator('body').innerText()).not.toMatch(/小白云|江苏白猿|1531335070|400-8876675|苏ICP备/);
  await site.screenshot(info.outputPath('safety-live-brand-failed.png'));
  fail=false;await site.retryBrand();
  await expect(page.locator('[data-brand-footer]')).toHaveText('验收公司');
  await expect(page.locator('[data-footer-contact=phone] a')).toHaveText(brand.contactPhone);
  await expect(page.locator('[data-footer-copyright]')).toHaveText(brand.copyright);
  await expect(page.locator('[data-footer-contact=email] a')).toHaveText(brand.contactEmail);
  await expect(page.locator('[data-footer-detail=qq] a')).toHaveText(brand.businessQq);
  await expect(page.locator('[data-footer-detail=address] a')).toHaveText(brand.companyAddress);
  await expect(page.locator('[data-filing-kind=public-security] .filing-title')).toHaveText(brand.publicSecurityNumber);
  await expect(page.locator('[data-filing-kind=icp] .filing-title')).toHaveText(brand.icpNumber);
  await expect(page.locator('.horizontal-menu-navbar-logo [data-brand-name]')).toHaveText('验收站点');
  await expect(page.locator('[data-safety-contact]').first()).toHaveAttribute('href','http://localhost:6680/contact.html');
  await expect(page.locator('[data-brand-retry]')).toHaveCount(0);
 });
});


// Direct HTTP preview: no source-file interception and no mocked brand response.
test('TC-127c: 实际源码预览HTTP与手机布局',async({browser},info)=>{
 test.setTimeout(60000);
 const context=await browser.newContext({viewport:{width:1440,height:900}});
 try{
 const page=await context.newPage();
 const brandResponse=page.waitForResponse(r=>new URL(r.url()).pathname==='/x/idc-portal/api/v1/public/brand');
 const response=await page.goto('http://127.0.0.1:16680/plan/safety.html');
 expect(response?.status()).toBe(200);
 await page.locator('[data-safety] main').waitFor();
 const site=new SafetySolutionPage(page);
 expect(await site.copy()).toEqual(copyReference);
 const publicBrand=await brandResponse;
 expect(publicBrand.status()).toBe(200);
 const payload=await publicBrand.json();expect(payload.code).toBe(0);
 const brand=payload.data.brand;
 const siteName=typeof brand.siteName==='string'?brand.siteName:brand.siteName?.zhCN||brand.siteName?.enUS||'';
 await expect(page.locator('.horizontal-menu-navbar-logo [data-brand-name]')).toHaveAttribute('aria-label',siteName);
 if(brand.contactPhone)await expect(page.locator('[data-footer-contact=phone] a')).toHaveText(brand.contactPhone);
 if(brand.contactEmail)await expect(page.locator('[data-footer-contact=email] a')).toHaveText(brand.contactEmail);
 await expect(page.locator('[data-brand-retry]')).toHaveCount(0);
 await site.screenshot(info.outputPath('safety-http-desktop.png'));
 await page.setViewportSize({width:390,height:844});
 await site.section('architecture');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await site.screenshot(info.outputPath('safety-http-mobile.png'));
 }finally{await context.close();}
});
