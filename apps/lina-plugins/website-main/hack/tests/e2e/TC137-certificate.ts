import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {CertificatePage} from '../pages/CertificatePage';
test.use({baseURL:'http://127.0.0.1:6680',websiteGateway:true,headless:true});
const reference=JSON.parse(await readFile(new URL('../support/certificate-copy-reference.json',import.meta.url),'utf8'));
const additions=[
 ['以规范为基石，以信任见长','每一份信任，都值得认真回应。我们希望通过清晰的信息展示、审慎的服务表达与持续完善的服务体验，让客户在了解、选择与合作过程中更加从容。'],
 ['资质展示','让资质信息清晰呈现，让合作判断有据可循。'],
 ['值得信赖，始于每一处认真'],
 ['进一步了解我们，开启合作沟通','如需了解服务内容及相关资料，欢迎与我们联系。'],
];
test.describe('TC-137 资质荣誉原文、授权新增栏目及真实入口',()=>{
 for(const mobile of [false,true])test(mobile?'TC-137b: 手机布局与空证书':'TC-137a: 桌面原文、图片与新增内容',async({page},info)=>{
  const errors:string[]=[],foreign:string[]=[];page.on('pageerror',e=>errors.push(e.message));page.on('request',r=>{if(/xiaobaiyun|baiyuan/.test(new URL(r.url()).hostname))foreign.push(r.url());});
  if(mobile)await page.setViewportSize({width:390,height:844});
  const site=new CertificatePage(page);expect((await site.open())?.status()).toBe(200);expect(await site.copy()).toEqual(reference.copy);
  expect(await page.locator('.pagebanner-title').evaluate(e=>{const r=e.getBoundingClientRect();return r.top>=document.querySelector('.nav-layoutcontainer')!.getBoundingClientRect().bottom&&r.bottom<innerHeight;})).toBe(true);
  await expect(page.locator('[data-certificate] main>section')).toHaveCount(4);await expect(page.locator('[data-certificate] main h2')).toHaveText(additions.map(s=>s[0]));
  for(const [i,section] of additions.entries())if(section[1])await expect(page.locator('[data-certificate] main>section').nth(i).locator('.certificate-section-heading>p')).toHaveText(section[1]);
  await expect(page.locator('.certificate-value h3')).toHaveText(['让信息更清晰','让沟通更顺畅','让体验持续向前']);
  await expect(page.locator('.certificate-value p')).toHaveText(['以易于理解的方式组织重要信息，让客户更方便地了解服务内容与合作边界。','重视每一次咨询与反馈，让需求表达更直接，让沟通过程更有条理。','以长期视角看待服务，持续关注使用体验与实际需求，让每一次改进都更贴近客户。']);
  await expect(page.locator('[data-certificate-grid]')).toHaveAttribute('data-empty','true');await expect(page.locator('.certificate-card')).toHaveCount(0);await expect(page.locator('dialog')).toHaveCount(0);
  expect(await page.evaluate(async()=>{const m=await import('/lina/certificate.js');return m.initializeCertificate(document)===m.initializeCertificate(document);})).toBe(true);await expect(page.locator('[data-certificate] main')).toHaveCount(1);
  expect(await page.evaluate(async()=>{const banner=document.querySelector('.globalBanner')!,style=getComputedStyle(banner),src=style.backgroundImage.match(/url\(["']?(.*?)["']?\)/)![1];const img=new Image();img.src=src;await img.decode();const r=banner.getBoundingClientRect();return {loaded:img.naturalWidth>0&&img.naturalHeight>0,local:new URL(src).origin===location.origin,proportional:style.backgroundSize==='cover',visible:r.width===innerWidth&&r.height>=180};})).toEqual({loaded:true,local:true,proportional:true,visible:true});
  await page.screenshot({path:info.outputPath((mobile?'mobile':'desktop')+'-initial.png'),animations:'disabled'});
  for(const id of ['principle','qualifications','values','consult']){await site.screenshotSection(id,info.outputPath((mobile?'mobile':'desktop')+'-'+id+'.png'));expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);}
  expect(await page.locator('[data-certificate] h1,[data-certificate] h2,[data-certificate] h3,[data-certificate] p').evaluateAll(es=>es.every(e=>e.scrollHeight<=e.clientHeight+1))).toBe(true);
  await expect(page.locator('[data-certificate-consult]')).toHaveText('联系咨询 →');await expect(page.locator('[data-certificate-consult]')).toHaveAttribute('href',/\/contact\.html$/);
  await page.locator('.sky-footer-top').scrollIntoViewIfNeeded();await page.screenshot({path:info.outputPath((mobile?'mobile':'desktop')+'-footer.png'),animations:'disabled'});
  await expect(page.locator('.sky-footer-nav-item a').filter({hasText:'荣誉资质'})).toHaveAttribute('href','/certificate.html');
  expect(errors).toEqual([]);expect(foreign).toEqual([]);
 });
 test('TC-137c: 品牌失败保留正文，重试恢复本方资料',async({page},info)=>{
  let fail=true;await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill(fail?{status:503,json:{code:50}}:{json:{code:0,data:{brand:{siteName:{zhCN:'本方资质验收'},footer:{zhCN:'本方公司'},contactPhone:'01012345678',copyright:'本方版权',navigation:[],footerNavigation:[]}}}}));
  const site=new CertificatePage(page);await site.open();await expect(page.locator('[data-brand-retry]')).toBeVisible();expect(await site.copy()).toEqual(reference.copy);
  await expect(page.locator('.sky-footer-nav-item a').filter({hasText:'荣誉资质'})).toHaveAttribute('href','/certificate.html');
  await expect(page.locator('[data-certificate]')).not.toContainText('江苏白猿');await page.screenshot({path:info.outputPath('brand-failed.png'),animations:'disabled'});
  fail=false;await site.retry();await expect(page.locator('[data-brand-retry]')).toHaveCount(0);await expect(page.locator('[data-brand-footer]')).toHaveText('本方公司');await expect(page).toHaveTitle('资质荣誉 - 本方资质验收');expect(await site.copy()).toEqual(reference.copy);await expect(page.locator('.certificate-card')).toHaveCount(0);await page.screenshot({path:info.outputPath('brand-recovered.png'),animations:'disabled'});
 });
 test('TC-137d: 首页了解我们菜单进入实际页面',async({page},info)=>{
  test.setTimeout(90000);
  const site=new CertificatePage(page);await page.goto('/');await page.waitForLoadState('networkidle');await site.menu();await page.screenshot({path:info.outputPath('home-entry.png'),animations:'disabled'});await site.enter();await expect(page).toHaveURL(/\/certificate\.html$/);expect(await site.copy()).toEqual(reference.copy);await expect(page.locator('main h2')).toHaveText(additions.map(s=>s[0]));
 });
 test('TC-137e: 预留卡片格式与图片预览，仅浏览器临时测试数据',async({page},info)=>{
  const moduleSource=await readFile(new URL('../../../frontend/public/lina/certificate-content.js',import.meta.url),'utf8');
  await page.route('**/lina/certificate-content.js',r=>r.fulfill({contentType:'application/javascript',body:moduleSource.replace('export const certificateRecords=[];',`export const certificateRecords=[{name:'图片预览测试',image:'/lina/certificate-assets/remoteL17069813671571.webp',issuer:'测试字段',validity:'测试字段'}];`)}));
  const site=new CertificatePage(page);await site.open();await expect(page.locator('.certificate-card')).toHaveCount(1);await expect(page.locator('[data-certificate-issuer]')).toHaveText('测试字段');await expect(page.locator('[data-certificate-validity]')).toHaveText('测试字段');
  await site.preview();await expect(page.getByRole('dialog')).toBeVisible();expect(await page.getByRole('dialog').locator('img').evaluate((e:HTMLImageElement)=>e.complete&&e.naturalWidth>0)).toBe(true);await page.screenshot({path:info.outputPath('preview-test-only.png'),animations:'disabled'});await site.closePreview();await expect(page.getByRole('dialog')).toHaveCount(0);await expect(page.locator('.certificate-preview')).toBeFocused();
 });
 test('TC-137f: 既有金融页原文菜单也指向本方资质页',async({page})=>{
  const site=new CertificatePage(page);await page.goto('/plan/financial.html');await page.waitForLoadState('networkidle');await site.menu();
  await expect(page.locator('.lina-open a').filter({has:page.getByText('资质荣誉',{exact:true})})).toHaveAttribute('href',/^http:\/\/127\.0\.0\.1:6680\/certificate\.html$/);
 });
});
