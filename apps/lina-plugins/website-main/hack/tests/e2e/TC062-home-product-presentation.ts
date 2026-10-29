import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewProducts,previewResponse} from '../support/home-preview-data.mjs';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'reduce'});
test.describe('TC-62 首页商品区原视觉与本方资料',()=>{
 test('TC-62a: 四列卡片、字体、角标、悬停与本方购买权限',async({page},info)=>{
  const rows=structuredClone(previewProducts.slice(0,10));rows[1].availability='unavailable';
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/catalog/products'))r.data={products:rows,categories:[],nextCursor:''};await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new HomeDetailPage(page);await site.open('/');
  const cards=page.locator('[data-products] .systemproduct-item');await expect(cards).toHaveCount(10);
  await expect(cards.locator('.systemproduct-name')).toHaveText(rows.map(p=>p.nameZhCN));await expect(cards.first().locator('.systemproduct-desc')).toHaveText(rows[0].summaryZhCN);
  await expect(cards.first()).toHaveAttribute('href','/configure.html?product='+rows[0].publicSlug);await expect(cards.nth(1)).not.toHaveAttribute('href');await expect(cards.nth(1)).toContainText('暂不可订购');
  await site.focusSection('.class-systemproducts');
  await expect(page.locator('.systemproduct-left-title')).toHaveText('全部商品');await expect(page.locator('.systemproduct-left-title')).toHaveCSS('font-size','20px');
  await expect(cards.first().locator('.systemproduct-name')).toHaveCSS('font-size','16px');await expect(cards.first().locator('.systemproduct-name')).toHaveCSS('line-height','26px');
  await expect(cards.first().locator('.systemproduct-desc')).toHaveCSS('font-size','14px');await expect(cards.first().locator('.systemproduct-desc')).toHaveCSS('line-height','26px');
  await expect(cards.locator('.systemproduct-tagline span')).toHaveText(rows.map(()=>'NEW'));
  const ribbon=cards.first().locator('.systemproduct-tagline img');await expect(ribbon).toHaveJSProperty('naturalWidth',67);await expect(ribbon).toHaveCSS('width','57px');
  const geometry=await cards.evaluateAll(ns=>ns.map(n=>{const r=n.getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height};}));
  expect(new Set(geometry.slice(0,4).map(r=>r.y)).size).toBe(1);expect(geometry[4].y).toBeGreaterThan(geometry[0].y);expect(geometry[1].x-geometry[0].x-geometry[0].w).toBeCloseTo(12,0);
  expect(geometry[0].h).toBe(147);await cards.first().hover();await expect(cards.first()).toHaveCSS('box-shadow','rgba(0, 0, 0, 0.1) 0px 0px 10px 0px');
  await site.captureSection('.class-systemproducts',info.outputPath('product-grid.png'));
 });
 test('TC-62b: 左右入场、减少动态、窄屏与失败不混入原站商品',async({page},info)=>{
  let failed=false;await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/catalog/products')&&failed)return route.fulfill({status:503,json:{code:503}});await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  await page.emulateMedia({reducedMotion:'no-preference'});const site=new HomeDetailPage(page);await site.open('/');await site.focusSection('.class-systemproducts');
  await expect(page.locator('[data-home-product-intro]')).toHaveCSS('animation-name','slideInLeft');await expect(page.locator('.systemproduct-right-container')).toHaveCSS('animation-name','slideInRight');
  await page.emulateMedia({reducedMotion:'reduce'});await expect(page.locator('[data-home-product-intro]')).toHaveCSS('animation-name','none');await expect(page.locator('.systemproduct-right-container')).toHaveCSS('animation-name','none');
  await page.setViewportSize({width:390,height:844});await site.focusSection('.class-systemproducts');
  const bounds=await page.locator('.class-systemproducts').evaluate(n=>({width:n.clientWidth,scroll:n.scrollWidth}));expect(bounds.scroll).toBeLessThanOrEqual(bounds.width+1);
  const first=await page.locator('[data-products] .systemproduct-item').first().boundingBox();expect(first).not.toBeNull();expect(first!.width).toBeGreaterThan(300);expect(first!.x+first!.width).toBeLessThanOrEqual(390);
  await site.captureSection('.class-systemproducts',info.outputPath('product-mobile.png'));
  failed=true;await site.open('/');await expect(page.locator('[data-status]')).toContainText('商品加载失败');await expect(page.locator('[data-products] .systemproduct-item')).toHaveCount(0);await expect(page.locator('.class-systemproducts')).not.toContainText('四川成都弹性云');
  await site.captureSection('.class-systemproducts',info.outputPath('product-failed.png'));
 });
});
