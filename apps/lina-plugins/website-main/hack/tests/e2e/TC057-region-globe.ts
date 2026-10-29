import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse} from '../support/home-preview-data.mjs';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-57 地域球面与失败恢复',()=>{
 for(const failure of [false,true])test('TC-57'+(failure?'b: 纹理失败保留文字':'a: 真实球面与窄屏'),async({page},info)=>{
  await page.route('**/x/**',async route=>{const q=route.request(),r=previewResponse(new URL(q.url()),q.method());await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  if(failure)await page.route('**/static/region/globe.png',route=>route.fulfill({status:404,body:''}));
  const site=new HomeDetailPage(page);await site.open('/');
  await expect(page.locator('.network_canvas')).toHaveAttribute('data-globe-state',failure?'unavailable':'ready');
  await expect(page.locator('.container_bg27 .sk-title')).toHaveText('全球云计算资源 BGP链路');
  await expect(page.locator('.certificate_title h1')).toHaveText(['北上广深','中国香港','美国US']);
  expect(await page.locator('.certificate_icon img').evaluateAll(ns=>ns.every(n=>(n as HTMLImageElement).complete&&(n as HTMLImageElement).naturalWidth>0))).toBe(true);
  if(failure)await expect(page.locator('#network_canvas')).toBeHidden();
  else {await expect(page.locator('#network_canvas')).toBeVisible();expect(await page.locator('#network_canvas').evaluate((n:HTMLCanvasElement)=>n.width===1037&&n.height===1037)).toBe(true);
   await site.focusSection('.container_bg27');const globe=page.locator('.network_canvas'),canvas=page.locator('#network_canvas');
   // The reference keeps decorative rotation running in both system motion modes.
   for(const reducedMotion of ['reduce','no-preference'] as const){
    await page.emulateMedia({reducedMotion});const still=await canvas.screenshot();
    await expect.poll(async()=>Buffer.compare(still,await canvas.screenshot())).not.toBe(0);
   }
   await page.emulateMedia({reducedMotion:'reduce'});
   const beforeDrag=await globe.screenshot();await site.dragGlobe();
   await expect.poll(async()=>Buffer.compare(beforeDrag,await globe.screenshot())).not.toBe(0);
   // Exercise the persisted page lifecycle without claiming native browser BFCache eligibility.
   await site.restoreCachedPage();await expect(globe).toHaveAttribute('data-globe-state','ready');
  }
  await site.captureSection('.container_bg27',info.outputPath('desktop-region.png'));
  await page.setViewportSize({width:390,height:844});await site.captureSection('.container_bg27',info.outputPath('mobile-region.png'));
  const region=await page.locator('.container_bg27').boundingBox();expect(region!.x+region!.width).toBeLessThanOrEqual(391);
 });
 for(const failure of ['webgl','vendor','context'])test('TC-57'+({webgl:'c',vendor:'d',context:'e'}[failure])+': '+failure+'失败保留地域卡',async({page},info)=>{
  await page.route('**/x/**',async route=>{const q=route.request(),r=previewResponse(new URL(q.url()),q.method());await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  if(failure==='webgl')await page.addInitScript(()=>{const original=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){if(String(type).includes('webgl'))return null;return original.call(this,type,...args);} as typeof original;});
  if(failure==='vendor')await page.route('**/static/vendor/three.min.js',route=>route.fulfill({status:404,body:''}));
  const site=new HomeDetailPage(page);await site.open('/');
  if(failure==='context'){await expect(page.locator('.network_canvas')).toHaveAttribute('data-globe-state','ready');await site.loseGlobeContext();}
  await expect(page.locator('.network_canvas')).toHaveAttribute('data-globe-state','unavailable');
  await expect(page.locator('#network_canvas')).toBeHidden();
  await expect(page.locator('.certificate_title h1')).toHaveText(['北上广深','中国香港','美国US']);
  await expect(page.locator('.container_bg27 .feature-title')).toHaveText(['中国大陆优化访问','联通各大洲国际网络']);
  await site.captureSection('.container_bg27',info.outputPath(failure+'-fallback.png'));
 });
});
