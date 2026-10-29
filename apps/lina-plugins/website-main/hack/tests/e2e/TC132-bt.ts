import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {BtPage} from '../pages/BtPage';
import {canonicalBtCopy} from '../support/bt-copy.mjs';
const live=process.env.BT_LIVE==='1';
test.use({baseURL:live?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:live,headless:true});
const reference=JSON.parse(await readFile(new URL('../support/bt-copy-reference.json',import.meta.url),'utf8'));
const assets=JSON.parse(await readFile(new URL('../../../docs/bt-images.json',import.meta.url),'utf8'));
test.describe('TC-132 宝塔附件、图片、复制与真实菜单',()=>{
 for(const mobile of [false,true])test(mobile?'TC-132b: 手机布局、图片与五锚点':'TC-132a: 桌面原文、图片与五锚点',async({page},info)=>{
  test.setTimeout(90000);const errors:string[]=[],foreign:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{const u=new URL(r.url());if(/xiaobaiyun|baiyuan/.test(u.hostname))foreign.push(r.url());});
  if(mobile)await page.setViewportSize({width:390,height:844});
  const site=new BtPage(page);expect((await site.open())?.status()).toBe(200);
  expect(canonicalBtCopy(await site.copy())).toEqual(canonicalBtCopy(reference.copy));
  await expect(page.locator('[data-bt] main>.et-slide')).toHaveCount(5);await expect(page.locator('.command .container-item>h2')).toHaveCount(7);
  expect(await page.evaluate(async()=>{const m=await import(new URL('/lina/bt.js',location.href).href);return m.initializeBt(document)===m.initializeBt(document);})).toBe(true);
  await expect(page.locator('[data-bt] main')).toHaveCount(1);
  expect(await page.locator('[data-bt] img').evaluateAll(nodes=>nodes.every(i=>i.getAttribute('src')!.startsWith('/lina/bt-assets/')&&(i as HTMLImageElement).naturalWidth>0))).toBe(true);
  expect(await page.evaluate(async paths=>Promise.all(paths.map(async path=>{const img=new Image();img.src=path;try{await img.decode();return img.naturalWidth>0;}catch{return false;}})),assets.map((a:{local:string})=>a.local))).toEqual(Array(9).fill(true));
  expect(await page.locator('[data-bt] .globalBanner').evaluate(e=>{const r=e.getBoundingClientRect();return getComputedStyle(e).backgroundImage.includes('remoteL17069717876526.png')&&r.width>300&&r.height>=350&&r.top<innerHeight;})).toBe(true);
  expect(await page.locator('[data-bt] h1').evaluate(e=>{const r=e.getBoundingClientRect();return r.top>=62&&r.bottom<innerHeight;})).toBe(true);
  await site.screenshot(info.outputPath('bt-'+(mobile?'mobile':'desktop')+'-hero.png'));
  for(const id of ['bt-advantages','bt-systems','install','bt-mount','bt-notice']){await site.section(id);await expect(page.locator('.et-hero-tab a[href="#'+id+'"]').first()).toHaveAttribute('aria-current','location');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);if(['bt-advantages','install'].includes(id))await site.screenshot(info.outputPath('bt-'+(mobile?'mobile':'desktop')+'-'+id+'.png'));}
  expect(await page.locator('.problem-content-item-content').evaluateAll(nodes=>nodes.every(e=>e.getBoundingClientRect().height>20))).toBe(true);
  expect(await page.locator('script[src*="/lina/site.js"]').count()).toBe(1);
  expect(errors).toEqual([]);expect(foreign).toEqual([]);
 });
 test('TC-132c: 七条复制逐字载荷、失败与手动选择',async({page,context})=>{
  const site=new BtPage(page);await site.open();await context.grantPermissions(['clipboard-read','clipboard-write']);
  await page.locator('.copyBtn').first().click();await expect(page.locator('.bt-copy-feedback')).toHaveText('复制成功');expect(await page.evaluate(()=>navigator.clipboard.readText())).toBe(reference.commands[0].copyText);
  await page.evaluate(()=>{(window as any).copied=[];Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async(value:string)=>{(window as any).copied.push(value);}}});});
  for(let i=0;i<7;i++){await page.locator('.copyBtn').nth(i).click();await expect(page.locator('.bt-copy-feedback')).toHaveText('复制成功');}
  expect(await page.evaluate(()=>(window as any).copied)).toEqual(reference.commands.map((c:{copyText:string})=>c.copyText));
  await page.evaluate(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async()=>{throw Error('denied');}}}));
  await page.locator('.copyBtn').last().click();await expect(page.locator('.bt-copy-feedback')).toHaveAttribute('data-state','failure');expect(await page.evaluate(()=>getSelection()?.toString())).toBe(reference.commands[6].displayText);
  await page.evaluate(()=>Object.defineProperty(navigator,'clipboard',{configurable:true,value:undefined}));await page.locator('.copyBtn').first().click();await expect(page.locator('.bt-copy-feedback')).toHaveAttribute('data-state','failure');
 });
 test('TC-132d: 品牌失败重试恢复、正文与图片保留',async({page},info)=>{
  let fail=true;const brand={siteName:{zhCN:'本方品牌验收'},footer:{zhCN:'本方公司'},copyright:'本方版权',contactPhone:'01012345678',navigation:[]};
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill(fail?{status:503,json:{code:50}}:{json:{code:0,data:{brand}}}));
  const site=new BtPage(page);await site.open();await expect(page.locator('[data-brand-retry]')).toBeVisible();expect(canonicalBtCopy(await site.copy())).toEqual(canonicalBtCopy(reference.copy));
  expect(await page.locator('.advantage-left img').evaluate(e=>(e as HTMLImageElement).naturalWidth>0)).toBe(true);await site.screenshot(info.outputPath('bt-brand-failed.png'));
  fail=false;await page.locator('[data-brand-retry]').click();await expect(page.locator('[data-brand-footer]')).toHaveText('本方公司');await expect(page.locator('[data-brand-retry]')).toHaveCount(0);await expect(page).toHaveTitle('宝塔面板 - 本方品牌验收');expect(canonicalBtCopy(await site.copy())).toEqual(canonicalBtCopy(reference.copy));await site.screenshot(info.outputPath('bt-brand-recovered.png'));
 });
 test('TC-132e: 首页真实菜单、Escape关闭与直接刷新',async({page},info)=>{
  const site=new BtPage(page);await page.goto('/');await page.waitForLoadState('networkidle');await site.menu();await expect(page.locator('.lina-open a').filter({has:page.getByText('宝塔面板',{exact:true})})).toHaveAttribute('href',/\/bt\.html$/);
  await page.keyboard.press('Escape');await expect(page.locator('.lina-open')).toHaveCount(0);await site.menu();await site.screenshot(info.outputPath('bt-home-menu.png'));await site.enter();await expect(page).toHaveURL(/\/bt\.html$/);expect(canonicalBtCopy(await site.copy())).toEqual(canonicalBtCopy(reference.copy));await page.reload();await site.ready();
 });
});
