import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {AssurePage} from '../pages/AssurePage';
import {canonicalAssureCopy} from '../support/assure-copy.mjs';
const live=process.env.ASSURE_LIVE==='1';
test.use({baseURL:live?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:live,headless:true});
const reference=JSON.parse(await readFile(new URL('../support/assure-copy-reference.json',import.meta.url),'utf8'));
const expected=canonicalAssureCopy(reference.copy);
const assets=JSON.parse(await readFile(new URL('../../../docs/assure-images-source-20261025.json',import.meta.url),'utf8'));
test.describe('TC-131 服务保障附件原文与实际入口',()=>{
 for(const mobile of [false,true])test(mobile?'TC-131b: 手机原文、三栏目与图片':'TC-131a: 桌面原文、三栏目与本方链接',async({page},info)=>{
  test.setTimeout(90000);const errors:string[]=[],foreign:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(/xiaobaiyun|baiyuan/.test(new URL(r.url()).hostname))foreign.push(r.url());});
  if(mobile)await page.setViewportSize({width:390,height:844});
  const site=new AssurePage(page);expect((await site.open())?.status()).toBe(200);
  expect(canonicalAssureCopy(await site.copy())).toEqual(expected);
  await expect(page.locator('.refundNotice p')).toHaveText('退款政策');
  await expect(page.locator('.reporting-text-title')).toHaveCount(2);
  expect(await page.evaluate(async()=>{const m=await import(new URL('/lina/assure.js',location.href).href);return m.initializeAssure(document)===m.initializeAssure(document);})).toBe(true);
  await expect(page.locator('[data-assure] main')).toHaveCount(1);
  expect(await page.locator('.pagebanner-title').evaluate(e=>{const r=e.getBoundingClientRect(),nav=document.querySelector('.nav-layoutcontainer')!.getBoundingClientRect();return r.top>=nav.bottom&&r.bottom<innerHeight;})).toBe(true);
  expect(await page.evaluate(async names=>Promise.all(names.map(async name=>{const img=new Image();img.src='/lina/assure-assets/'+name;try{await img.decode();return img.naturalWidth>0;}catch{return false;}})),assets.map((i:{name:string})=>i.name))).toEqual(Array(assets.length).fill(true));
  expect(await page.locator('[data-assure] img').evaluateAll(nodes=>nodes.every(e=>e.getBoundingClientRect().width>=20))).toBe(true);
  await site.screenshot(info.outputPath('assure-'+(mobile?'mobile':'desktop')+'-initial.png'));
  for(const id of ['support','refund','report']){
   await site.section(id);await expect(page.locator('.et-hero-tab a[href="#assure-'+id+'"]').first()).toHaveAttribute('aria-current','location');
   await expect(page.locator('.et-hero-tab.active')).toHaveCount(1);
   await expect(page.locator('.et-hero-tab.active a')).toHaveAttribute('href','#assure-'+id);
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
   await site.screenshot(info.outputPath('assure-'+(mobile?'mobile':'desktop')+'-'+id+'.png'));
   if(id==='refund')await page.locator('.refundNotice-content').screenshot({path:info.outputPath('assure-'+(mobile?'mobile':'desktop')+'-refund-full.png'),animations:'disabled',style:'[data-assure] .et-container{visibility:hidden}'});
  }
  expect(await page.locator('.refundNotice-content').evaluate(e=>e.scrollHeight<=e.clientHeight+1)).toBe(true);
  await page.locator('.sky-footer-top').scrollIntoViewIfNeeded();
  expect(await page.locator('.sky-footer-title').evaluateAll(nodes=>nodes.every(e=>{const r=e.getBoundingClientRect();return r.width>=84&&r.height<=48&&e.scrollWidth<=e.clientWidth+1;}))).toBe(true);
  await site.screenshot(info.outputPath('assure-'+(mobile?'mobile':'desktop')+'-footer.png'));
  const brandResponse=await page.request.get('/x/idc-portal/api/v1/public/brand');const brand=(await brandResponse.json()).data.brand;
  if(brand.contactPhone)await expect(page.locator('[data-assure-phone]')).toContainText(brand.contactPhone);
  await expect(page.locator('[data-assure-phone]')).not.toContainText('400-8876675');
  await expect(page.locator('[data-assure-consult]')).toHaveAttribute('href',/\/contact\.html$/);
  await expect(page.locator('[data-assure] [data-customer="/account/support"]')).toHaveAttribute('href',/\/account\/support$/);
  await expect(page.locator('[data-assure] [data-customer="/account/support/new"]')).toHaveAttribute('href',/\/account\/support\/new$/);
  await expect(page.locator('.refundService .document-title')).toHaveAttribute('href','/404.html?feature=assure-refund');
  await expect(page.locator('.reporting-btn')).toHaveAttribute('href','/404.html?feature=assure-report');
  expect(errors).toEqual([]);expect(foreign).toEqual([]);
 });
 test('TC-131c: 品牌失败重试恢复、电话更新清空不改变原文',async({page},info)=>{
  let fail=true;const brand={siteName:{zhCN:'保障验收'},footer:{zhCN:'本方公司'},copyright:'本方版权',contactPhone:'01012345678',navigation:[]};
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill(fail?{status:503,json:{code:50}}:{json:{code:0,data:{brand}}}));
  const site=new AssurePage(page);await site.open();await expect(page.locator('[data-brand-retry]')).toBeVisible();
  expect(canonicalAssureCopy(await site.copy())).toEqual(expected);await expect(page.locator('[data-assure-phone]')).not.toContainText('400-8876675');
  await site.screenshot(info.outputPath('assure-brand-failed.png'));fail=false;await site.retry();
  await expect(page.locator('[data-assure-phone]')).toHaveText('24小时服务热线： '+brand.contactPhone);
  await expect(page.locator('[data-brand-footer]')).toHaveText('本方公司');await expect(page.locator('[data-brand-retry]')).toHaveCount(0);
  expect(canonicalAssureCopy(await site.copy())).toEqual(expected);await site.screenshot(info.outputPath('assure-brand-recovered.png'));
  brand.contactPhone='';await page.reload();await site.ready();await expect(page.locator('[data-assure-phone]')).not.toHaveAttribute('href',/.+/);
 });
 test('TC-131d: 实际首页支持与服务入口',async({page},info)=>{
  const site=new AssurePage(page);await page.goto('/');await page.waitForLoadState('networkidle');await site.menu();
  await site.screenshot(info.outputPath('assure-home-entry.png'));await site.enter();await expect(page).toHaveURL(/\/Assure\.html$/);
  expect(canonicalAssureCopy(await site.copy())).toEqual(expected);
 });
});
