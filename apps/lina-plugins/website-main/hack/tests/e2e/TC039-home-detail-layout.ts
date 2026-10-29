import {test,expect} from '../support/source-site';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';
import {HomeDetailPage} from '../pages/HomeDetailPage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-39 首页细节与真实尺寸回归',()=>{
 test('TC-39d: 三张横幅和四张能力图片从本地公开路径加载',async({page},info)=>{
  const site=new HomeDetailPage(page);
  const heroes=['remoteL17136807293281.jpg','remoteL17139270188081.jpg','remoteL17139312433523.jpg'];
  const benefits=['remoteL17140335196703.png','remoteL17140335436547.png','remoteL17140335582829.png','remoteL17140335823614.png'];
  await page.route('**/x/**',async route=>{const request=route.request(),url=new URL(request.url()),result=previewResponse(url,request.method());
   if(url.pathname.endsWith('/brand')){const config={...previewBrand.templateConfig};config.heroslides=config.heroslides.split('\n').slice(0,2).map((row,i)=>{const cells=row.split('|');cells[2]='/static/homepage/'+heroes[i+1];return cells.join('|');}).join('\n');config.benefits=config.benefits.split('\n').map((row,i)=>{const cells=row.split('|');cells[2]='/static/homepage/'+benefits[i];return cells.join('|');}).join('\n');result.data={brand:{...previewBrand,heroImage:'/static/homepage/'+heroes[0],templateConfig:config}};}
   await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});
  });
  await site.open('/index.html');
  await expect.poll(()=>page.locator('.Rotation-layoutcontainer img[src*="/static/homepage/"],.chooseUs-itme-icon img[src*="/static/homepage/"]').evaluateAll(images=>images.length===7&&images.every(image=>(image as HTMLImageElement).complete&&(image as HTMLImageElement).naturalWidth>0))).toBe(true);
  for(const name of heroes)await expect(page.locator('.Rotation-layoutcontainer img[src$="'+name+'"]')).toHaveCount(1);
  for(const name of benefits)await expect(page.locator('.chooseUs-itme-icon img[src$="'+name+'"]')).toHaveCount(1);
  await site.captureSection('.Rotation-layoutcontainer',info.outputPath('local-hero.png'));
  await site.captureSection('section.container_bg1:has(.chooseUs-container)',info.outputPath('local-benefits.png'));
 });
 test('TC-39c: 十五项伙伴分三行，超量或空资料隐藏且不恢复旧平铺资料',async({page},info)=>{
  const site=new HomeDetailPage(page);let count=15;
  const names=Array.from({length:15},(_,i)=>'模拟伙伴 '+(i+1));
  await page.route('**/x/**',async route=>{const request=route.request(),url=new URL(request.url()),result=previewResponse(url,request.method());
   if(url.pathname.endsWith('/brand'))result.data={brand:{...previewBrand,templateConfig:{...previewBrand.templateConfig,partner1title:'不应恢复的旧平铺伙伴',partners:Array.from({length:count},(_,i)=>'模拟伙伴 '+(i+1)+' | /static/partners/aliy.png').join('\n')}}};
   await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});
  });
  await site.open('/index.html');await expect(page.locator('.partners-item-container:visible')).toHaveCount(3);
  await expect.poll(async()=>(await site.tracks()).flatMap(row=>row.names)).toEqual(names);
  expect((await site.tracks()).map(row=>row.names.length)).toEqual([5,5,5]);
  await expect(page.locator('.partners-item:visible')).toHaveCount(15);
  await expect.poll(()=>page.locator('.partners-item:visible img').evaluateAll(images=>images.length===15&&images.every(image=>(image as HTMLImageElement).complete&&(image as HTMLImageElement).naturalWidth>0))).toBe(true);
  await site.captureSection('.container_bg30',info.outputPath('15-partners.png'));
  for(count of [16,0]){await site.open('/index.html');await expect(page.locator('.container_bg30')).toBeHidden();await expect(page.locator('.partners-item:visible')).toHaveCount(0);await site.captureSection('.sky-footer-top',info.outputPath(count+'-partners-hidden.png'));}
 });
 test('TC-39a: 伙伴轨道填满桌面宽度，缩放不累积副本，悬停与减少动态效果有效',async({page})=>{
  const site=new HomeDetailPage(page);await site.motion('no-preference');
  await page.route('**/x/**',async route=>{const r=route.request(),result=previewResponse(new URL(r.url()),r.method());await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});});
  await site.open('/index.html');await expect(page.locator('.partners-item-container:visible')).toHaveCount(3);
  await site.focusSection('.container_bg30');await site.leavePartners();
  for(const width of [1440,1920,1280,1440]){
   await site.setViewport(width,900);
   await expect.poll(async()=> (await site.tracks()).every(t=>t.width>=t.viewport-1)).toBe(true);
   const tracks=await site.tracks();expect(tracks.flatMap(t=>t.names)).toEqual(Array.from({length:6},(_,i)=>'模拟伙伴 '+(i+1)));
   expect(tracks.every(t=>t.duplicateHidden&&t.copies<20)).toBe(true);
  }
  const first=page.locator('.partners-item-page').first();await expect(first).toHaveCSS('animation-play-state','running');
  const before=(await site.tracks())[0].transform;await expect.poll(async()=>(await site.tracks())[0].transform).not.toBe(before);
  await site.hoverPartners();await expect(first).toHaveCSS('animation-play-state','paused');
  const paused=(await site.tracks())[0].transform;await page.waitForTimeout(200);expect((await site.tracks())[0].transform).toBe(paused);
  await site.capture('home-partner-final-paused',false);await site.leavePartners();await site.motion('reduce');
  await expect(first).toHaveCSS('animation-name','none');await expect(page.locator('.partners-item:visible')).toHaveCount(6);
  expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(1440);await site.capture('home-partner-final-reduced',false);
 });
 test('TC-39b: 空横幅不压住快捷区，地域底卡只使用配置文本',async({page})=>{
  const site=new HomeDetailPage(page);let empty=false;
  await page.route('**/x/**',async route=>{const r=route.request(),url=new URL(r.url()),result=previewResponse(url,r.method());
   if(empty&&url.pathname.endsWith('/brand'))result.data={brand:{...previewBrand,heroTitle:{zhCN:''},heroDescription:{zhCN:''},heroImage:'',templateConfig:{...previewBrand.templateConfig,heroslides:'',shortcuts:''}}};
   await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});
  });
  await site.open('/index.html');await expect(page.locator('.lina-home-carousel')).toBeVisible();const populated=await site.geometry();expect(Number.parseFloat(populated.margin)).toBeLessThan(0);
  empty=true;await site.open('/index.html');await expect(page.locator('.lina-hero-empty')).toBeVisible();await expect(page.locator('[data-home-blocks] > *')).toHaveCount(4);
  await expect(page.locator('[data-home-blocks] > .bannerbelow-list-empty')).toHaveCount(4);
  for(const card of await page.locator('[data-home-blocks] > .bannerbelow-list-empty').all()){
   await expect(card).toHaveAttribute('aria-hidden','true');await expect(card).not.toHaveAttribute('href');await expect(card).toHaveCSS('pointer-events','none');
  }
  await expect(page.locator('[data-home-blocks] .bannerbelow-icon-container:visible')).toHaveCount(4);
  await expect(page.locator('[data-home-blocks] .bannerbelow-title')).toHaveText(['','','','']);
  await expect(page.locator('[data-home-blocks] .bannerbelow-description')).toHaveText(['','','','']);
  const geometry=await site.geometry();expect(geometry.margin).toBe('0px');expect(geometry.shortcutTop).toBeGreaterThanOrEqual(geometry.heroBottom-1);
  await site.capture('home-empty-hero-shortcuts',false);
  await expect(page.locator('.network_footer .certificate_title h1')).toHaveText(['模拟地域说明 1','模拟地域说明 2','模拟地域说明 3']);
  await expect(page.locator('.network_canvas')).toBeHidden();await expect(page.locator('.network_right')).toBeHidden();
  await expect(page.locator('.network_footer .certificate_icon:visible')).toHaveCount(0);
  await site.focusSection('.network_footer');await site.capture('home-region-final-text-only',false);
 });
});
