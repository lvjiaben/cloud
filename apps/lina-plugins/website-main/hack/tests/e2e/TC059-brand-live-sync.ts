import {test,expect} from '../support/source-site';
import {BrandSyncPage} from '../pages/BrandSyncPage';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1270,height:900},reducedMotion:'reduce'});
const initial={siteName:'隔离旧站名',siteTitle:'隔离旧标题',companyName:'隔离旧公司',contactPhone:'01000000000',contactEmail:'old@example.invalid',description:'隔离旧简介',menu:'旧导航'};
const updated={siteName:'隔离新站名',siteTitle:'隔离新标题',companyName:'隔离新公司',contactPhone:'02000000000',contactEmail:'new@example.invalid',description:'隔离新简介',menu:'新导航'};
const project=(s:typeof initial)=>({...previewBrand,siteName:{zhCN:s.siteName},seoTitle:{zhCN:s.siteTitle},footer:{zhCN:s.companyName},contactPhone:s.contactPhone,contactEmail:s.contactEmail,seoDescription:{zhCN:s.description},navigation:[{label:{zhCN:s.menu},group:true,children:[{label:{zhCN:'本方帮助'},href:'/help.html'}]}]});
test.describe('TC-59 公共资料无需重打包同步',()=>{
 test('TC-59a: 隔离保存、重读、公开投影及DOM，无导航操作和轮播重置',async({page,context},info)=>{
  let stored={...initial},requests=0;
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/brand')){requests++;r.data={brand:project(stored)};}await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new BrandSyncPage(page);await site.open('/');await expect(page.locator('[data-brand-name]').first()).toHaveText(initial.siteName);
  const groups=page.locator('.horizontal-menu-navbar-menu > li > [aria-expanded]');
  const groupIndex=(await groups.allTextContents()).findIndex(text=>text.trim()===initial.menu);expect(groupIndex).toBeGreaterThanOrEqual(0);
  await site.hero(2);await site.focusGroup(groupIndex);await site.markNavigation();const toggle=groups.nth(groupIndex);await expect(toggle).toBeFocused();
  const admin=await context.newPage();
  await admin.route('**/__brand-admin-state',async route=>{if(route.request().method()==='PUT')stored={...route.request().postDataJSON()};await route.fulfill({json:stored});});
  await admin.route('**/__brand-admin',route=>route.fulfill({contentType:'text/html; charset=utf-8',body:`<form>${Object.keys(initial).map(k=>'<label>'+k+'<input name="'+k+'" aria-label="'+k+'"></label>').join('')}<button>保存</button><output></output></form><script>fetch('/__brand-admin-state').then(r=>r.json()).then(s=>{for(const[k,v]of Object.entries(s))document.querySelector('[name='+k+']').value=v;document.querySelector('output').textContent='已读取'});document.querySelector('form').onsubmit=async e=>{e.preventDefault();await fetch('/__brand-admin-state',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(Object.fromEntries(new FormData(e.target)))});document.querySelector('output').textContent='已保存'}</script>`}));
  try{
   await admin.goto('http://localhost:6680/__brand-admin');await expect(admin.locator('output')).toHaveText('已读取');
   for(const[k,v]of Object.entries(updated))await admin.getByLabel(k,{exact:true}).fill(v);
   await admin.getByRole('button',{name:'保存'}).click();await expect(admin.locator('output')).toHaveText('已保存');
   await admin.reload();await expect(admin.locator('output')).toHaveText('已读取');for(const[k,v]of Object.entries(updated))await expect(admin.getByLabel(k,{exact:true})).toHaveValue(v);
   await admin.screenshot({path:info.outputPath('synthetic-admin-reloaded.png')});
  }finally{await admin.close();}
  // Synthetic public responses exercise the browser chain; no actual admin configuration is written.
  await site.refreshBrand();await expect(page.locator('[data-brand-name]').first()).toHaveText(updated.siteName);await expect(page).toHaveTitle(updated.siteTitle);
  await expect(page.locator('[data-brand-footer]')).toHaveText(updated.companyName);await expect(page.locator('[data-footer-contact=phone] a')).toHaveText(updated.contactPhone);await expect(page.locator('[data-footer-contact=email] a')).toHaveText(updated.contactEmail);await expect(page.locator('meta[name=description]')).toHaveAttribute('content',updated.description);
  await expect(page.locator('.swipervertical-title').nth(2)).toHaveAttribute('aria-pressed','true');
  await expect(page.locator('.horizontal-menu-navbar-menu [data-sync-marker]')).toHaveCount(1);await expect(toggle).toBeFocused();await expect(toggle).toHaveAttribute('aria-expanded','true');await expect(toggle).toHaveText(initial.menu);
  await site.exitNavigation();await expect(page.locator('.horizontal-menu-navbar-menu')).toContainText(updated.menu);await expect(page.locator('.horizontal-menu-navbar-menu')).not.toContainText(initial.menu);
  await site.markNavigation();await site.focusGroup(groupIndex);stored={...updated,companyName:'只改公司名称'};
  const count=requests;await site.refreshBrand();await expect.poll(()=>requests).toBeGreaterThan(count);await expect(page.locator('[data-brand-footer]')).toHaveText(stored.companyName);await expect(page.locator('.horizontal-menu-navbar-menu [data-sync-marker]')).toHaveCount(1);await expect(toggle).toBeFocused();await expect(toggle).toHaveAttribute('aria-expanded','true');
  await site.captureSection('.nav-layoutcontainer',info.outputPath('public-dynamic-updated.png'));
 });
 test('TC-59b: 可见30秒检查、隐藏停止、切回和持久页面恢复、清空联系方式',async({page},info)=>{
  await page.clock.install({time:new Date('2026-10-20T00:00:00Z')});await page.clock.pauseAt(new Date('2026-10-20T00:00:01Z'));let stored={...initial},requests=0;
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/brand')){requests++;r.data={brand:project(stored)};}await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new BrandSyncPage(page);await site.open('/');await expect(page.locator('[data-footer-contact=phone]')).toBeVisible();const start=requests;stored={...updated};
  await page.clock.runFor(29000);expect(requests).toBe(start);await page.clock.runFor(1100);await expect(page.locator('[data-brand-name]').first()).toHaveText(updated.siteName);
  await site.visibility(true);const hiddenCount=requests;stored={...updated,siteName:'隐藏后新站名',contactPhone:'',contactEmail:'',description:''};await page.clock.runFor(90000);expect(requests).toBe(hiddenCount);
  await site.visibility(false);await expect(page.locator('[data-brand-name]').first()).toHaveText(stored.siteName);await expect(page.locator('[data-footer-contact=phone]')).toBeHidden();await expect(page.locator('[data-footer-contact=email]')).toBeHidden();await expect(page.locator('meta[name=description]')).toHaveAttribute('content','');
  await expect(page.locator('[data-footer-contact=phone] a')).toBeEmpty();await expect(page.locator('[data-footer-contact=email] a')).not.toHaveAttribute('href');
  stored={...updated};await site.lifecycleRestore();await expect(page.locator('[data-brand-name]').first()).toHaveText(updated.siteName);await expect(page.locator('[data-footer-contact=email] a')).toHaveText(updated.contactEmail);
  await site.captureSection('[data-footer-contact=email]',info.outputPath('contact-updated.png'));
 });
 test('TC-59c: 合并并发、后台失败保留最近成功资料及恢复',async({page},info)=>{
  let calls=0,active=0,maxActive=0;let release!:()=>void;const gate=new Promise<void>(r=>release=r);
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(!u.pathname.endsWith('/brand'))return route.fulfill({status:r.status,json:{code:0,data:r.data}});const n=++calls;active++;maxActive=Math.max(maxActive,active);if(n===1)await gate;active--;return route.fulfill(n===2?{status:503,json:{code:503}}:{json:{code:0,data:{brand:project(n===1?initial:updated)}}});});
  const site=new BrandSyncPage(page);await site.open('/');await expect.poll(()=>calls).toBe(1);await site.refreshBrand();await site.refreshBrand();await site.refreshBrand();expect(calls).toBe(1);release();await expect.poll(()=>calls).toBe(2);await expect(page.locator('[data-brand-name]').first()).toHaveText(initial.siteName);await expect(page.locator('.lina-brand-error')).toHaveCount(0);expect(maxActive).toBe(1);
  await site.refreshBrand();await expect(page.locator('[data-brand-name]').first()).toHaveText(updated.siteName);await site.captureSection('.nav-layoutcontainer',info.outputPath('retained-and-recovered.png'));
 });
 test('TC-59d: 首次失败可手动恢复，旧Logo与内容配置仍不消费',async({page},info)=>{
  let failed=true;await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/brand')){if(failed)return route.fulfill({status:503,json:{code:503}});r.data={brand:{...project(updated),logoUrl:'/retired-logo-must-not-load.png',templateConfig:{herotitle:'不应消费的旧文案'}}};}await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new BrandSyncPage(page);await site.open('/');await expect(page.getByText('网站资料暂时无法加载，请重试。')).toBeVisible();failed=false;await site.retryBrand();await expect(page.locator('[data-brand-name]').first()).toHaveText(updated.siteName);await expect(page.locator('.lina-brand-error')).toHaveCount(0);await expect(page.locator('body')).not.toContainText('不应消费的旧文案');await expect(page.locator('img[src*="retired-logo"]')).toHaveCount(0);await site.captureSection('.nav-layoutcontainer',info.outputPath('first-failure-recovered.png'));
 });
});
