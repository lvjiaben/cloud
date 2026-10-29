import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {FinancialSolutionPage} from '../pages/FinancialSolutionPage';

const live=process.env.FINANCIAL_LIVE==='1';
test.use({baseURL:live?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:live,headless:true});
const reference=JSON.parse(await readFile(new URL('../support/financial-copy-reference.json',import.meta.url),'utf8'));
const copy=reference.copy.map((i:{text:string})=>({...i,text:i.text.replace(/\r\n?/g,'\n')}));
const images=JSON.parse(await readFile(new URL('../support/financial-image-reference.json',import.meta.url),'utf8'));

test.describe('TC-129 金融方案附件原文与实际入口',()=>{
 for(const mobile of [false,true])test(mobile?'TC-129b: 手机原文、六锚点和布局':'TC-129a: 桌面原文、菜单和本方资料',async({page},info)=>{
  test.setTimeout(90000);
  const errors:string[]=[],foreign:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('request',r=>{if(/xiaobaiyun|baiyuan/i.test(new URL(r.url()).hostname))foreign.push(r.url());});
  await page.addInitScript(()=>localStorage.setItem('lina.website.locale','zh-CN'));
  if(mobile)await page.setViewportSize({width:390,height:844});
  const site=new FinancialSolutionPage(page),response=await site.open();await site.ready();
  expect(response?.status()).toBe(200);expect(await site.copy()).toEqual(copy);
  await expect(page.locator('.problem-left-title')).toHaveJSProperty('tagName','H3');
  await expect(page.locator('.problem-left-desc')).toHaveJSProperty('tagName','H5');
  await expect(page.locator('.architecture-advantage h1')).toHaveText('架构优势');
  await expect(page.locator('.problem-left-ul li')).toHaveCount(6);
  await expect(page.locator('.problem-item')).toHaveCount(4);
  expect(await page.locator('.problem-item-content').evaluateAll(items=>items.every(e=>e.scrollHeight<=e.clientHeight+1))).toBe(true);
  await expect(page.locator('.correlation-Content-row')).toBeEmpty();
  await expect(page.locator('.partner-Content-logo-item img')).toHaveCount(12);
  expect(await page.locator('.partner-Content-logo-item img').evaluateAll(nodes=>nodes.every(node=>node.getBoundingClientRect().width>=60))).toBe(true);
  await expect(page.locator('[data-financial] img')).toHaveCount(21);
  expect(await page.locator('[data-financial] img').evaluateAll(async nodes=>Promise.all(nodes.map(async node=>{const img=node as HTMLImageElement;try{await img.decode();return img.naturalWidth>0&&img.naturalHeight>0;}catch{return false;}})))).toEqual(Array(21).fill(true));
  expect(await page.evaluate(async paths=>Promise.all(paths.map(async path=>{const img=new Image();img.src=path;try{await img.decode();return img.naturalWidth>0;}catch{return false;}})),images.assets)).toEqual(Array(images.assets.length).fill(true));
  expect(await page.locator('.globalBanner').evaluate(e=>getComputedStyle(e).backgroundImage)).toContain(images.backgrounds.banner);
  expect(await page.evaluate(async()=>{const m=await import(new URL('/lina/financial.js',location.href).href);return m.initializeFinancial(document)===m.initializeFinancial(document);})).toBe(true);
  await expect(page.locator('[data-financial] main')).toHaveCount(1);
  await site.screenshot(info.outputPath('financial-'+(mobile?'mobile':'desktop')+'-initial.png'));
  for(const id of ['advantages','problems','architecture','products','partners','more']){
   await site.section(id);
   await expect(page.locator('.et-hero-tab a[href="#financial-'+id+'"]')).toHaveAttribute('aria-current','location');
   expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
   if(['problems','architecture','partners','more'].includes(id))await site.screenshot(info.outputPath('financial-'+(mobile?'mobile':'desktop')+'-'+id+'.png'));
  }
  await site.top();await site.menu('解决方案');
  const groups=reference.navigation.find((i:{label:string})=>i.label==='解决方案').children;
  for(const group of groups){
   await site.menuTab(group.label);
   const cards=page.locator('.lina-open .lina-nav-category-view:not([hidden]) .Submenu-title');
   await expect(cards).toHaveText(group.children.map((i:{label:string})=>i.label));
   expect(await page.locator('.lina-open .lina-nav-category-view:not([hidden]) .Submenu-desc').allTextContents()).toEqual(group.children.map((i:{description:string})=>i.description));
  }
  await site.menuTab(groups[0].label);
  await expect(page.locator('.lina-open a').filter({has:page.getByText('金融解决方案',{exact:true})})).toHaveAttribute('href',new URL('/plan/financial.html',page.url()).href);
  await site.screenshot(info.outputPath('financial-'+(mobile?'mobile':'desktop')+'-menu.png'));await site.escapeMenu();
  const brand=await page.evaluate(async()=>{const r=await fetch('/x/idc-portal/api/v1/public/brand');return (await r.json()).data.brand;});
  if(brand?.contactPhone)await expect(page.locator('[data-footer-contact=phone] a')).toHaveText(brand.contactPhone);
  await expect(page.locator('[data-financial-contact]').first()).toHaveAttribute('href',/\/contact\.html$/);
  expect(errors).toEqual([]);expect(foreign).toEqual([]);
 });

 test('TC-129c: 品牌失败与重试恢复不改变正文',async({page},info)=>{
  let fail=true;
  const brand={siteName:{zhCN:'金融验收站'},footer:{zhCN:'本方公司'},copyright:'本方版权',contactPhone:'01012345678',contactEmail:'contact@example.invalid',navigation:[],headerLogoUrl:'',footerLogoUrl:'',icpNumber:'本方备案'};
  await page.route('**/x/idc-portal/api/v1/public/brand',route=>route.fulfill(fail?{status:503,json:{code:50}}:{json:{code:0,data:{brand}}}));
  const site=new FinancialSolutionPage(page);await site.open();await site.ready();
  await expect(page.locator('[data-brand-retry]')).toBeVisible();expect(await site.copy()).toEqual(copy);
  expect(await page.locator('body').innerText()).not.toMatch(/小白云|江苏白猿|400-8876675|1531335070/);
  await site.screenshot(info.outputPath('financial-brand-failed.png'));
  fail=false;await site.retryBrand();
  await expect(page.locator('[data-brand-footer]')).toHaveText('本方公司');
  await expect(page.locator('[data-footer-contact=phone] a')).toHaveText(brand.contactPhone);
  await expect(page.locator('[data-footer-copyright]')).toHaveText(brand.copyright);
  await expect(page.locator('[data-brand-retry]')).toHaveCount(0);
  expect(await site.copy()).toEqual(copy);await site.screenshot(info.outputPath('financial-brand-recovered.png'));
 });

 test('TC-129d: 首页旧404入口改为金融本地页面',async({page})=>{
  const site=new FinancialSolutionPage(page);await site.home();await site.menu('解决方案');await site.menuTab('行业解决方案');
  await site.enterFinancial();await site.ready();
  await expect(page).toHaveURL(/\/plan\/financial\.html$/);expect(await site.copy()).toEqual(copy);
 });
});
