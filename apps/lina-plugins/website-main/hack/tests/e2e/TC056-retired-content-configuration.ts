import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-56 固定展示不再消费退役配置',()=>{
 for(const mode of ['stale','failed'])test('TC-56 '+mode,async({page},info)=>{
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/brand')){if(mode==='failed')return route.fulfill({status:503,json:{code:503}});r.data={brand:{...previewBrand,siteName:{zhCN:'本方站名'},logoUrl:'/must-not-load-logo.svg',heroTitle:{zhCN:'不应显示的旧值'},heroDescription:{zhCN:'不应显示的旧值'},templateConfig:{heroslides:'不应显示的旧值 | - | - | -',partnerheading:'不应显示的旧值',partners:'不应显示的旧值 | -',regionheading:'不应显示的旧值',regions:'不应显示的旧值 | -',productintrotitle:'不应显示的旧值',icpnumber:'不应显示的旧值'}}};}await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new HomeDetailPage(page);await site.open('/');
  await expect(page.locator('body')).not.toContainText('不应显示的旧值');
  await expect(page.locator('.swipervertical-title')).toHaveText(['模拟商品 01','模拟商品 02','模拟商品 03','模拟商品 04']);
  const slides=page.locator('.Rotation-layoutcontainer .swiper-slide');await expect(slides).toHaveCount(4);
  expect(await slides.locator('img').evaluateAll(ns=>ns.every(n=>(n as HTMLImageElement).complete&&(n as HTMLImageElement).naturalWidth>0))).toBe(true);
  await page.getByRole('button',{name:'下一张',exact:true}).click();await expect(page.locator('.swiper-text:visible .Title')).toHaveText('模拟商品 02');
  await expect(page.locator('.Rotation-layoutcontainer a[href]')).toHaveCount(4);
  await expect(page.locator('.systemproduct-left-title')).toHaveText('全部商品');await expect(page.locator('.systemproduct-left-desc')).toHaveText('');
  await expect(page.locator('.container_bg30 .sk-title')).toHaveText('全新网络体验');await expect(page.locator('.container_bg30 .sk-desc')).toHaveText('全球数十家顶级运营商合作，满足多行业应用需求，为您提供安全、稳定、合规的云服务。');
  await expect(page.locator('.partners-item-row > .partners-item-page:first-child .partners-item:not([data-partner-repeat])')).toHaveCount(15);
  expect(await page.locator('.container_bg30 img').evaluateAll(ns=>ns.every(n=>(n as HTMLImageElement).complete&&(n as HTMLImageElement).naturalWidth>0))).toBe(true);
  await expect(page.locator('[data-filing-kind]:visible')).toHaveCount(0);await expect(page.locator('img[src*="must-not-load-logo"]')).toHaveCount(0);
  await site.captureSection('.container_bg30',info.outputPath(mode+'-fixed-partners.png'));
  if(mode==='stale')await expect(page.locator('[data-brand-name]').first()).toHaveText('本方站名');
 });
 test('TC-56 motion: 三行滚动、悬停与减少动态',async({page},info)=>{
  await page.emulateMedia({reducedMotion:'no-preference'});
  await page.route('**/x/**',async route=>{const q=route.request(),r=previewResponse(new URL(q.url()),q.method());await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new HomeDetailPage(page);await site.open('/');await site.focusSection('.container_bg30');await site.leavePartners();
  const first=page.locator('.partners-item-page').first();const initial=await first.evaluate(n=>getComputedStyle(n).transform);
  await expect.poll(()=>first.evaluate(n=>getComputedStyle(n).transform)).not.toBe(initial);
  await site.hoverPartners();await expect(first).toHaveCSS('animation-play-state','paused');
  await site.leavePartners();await expect(first).toHaveCSS('animation-play-state','running');
  await site.captureSection('.container_bg30',info.outputPath('running-fixed-partners.png'));
  await page.emulateMedia({reducedMotion:'reduce'});await expect(first).toHaveCSS('animation-name','none');
 });
});
