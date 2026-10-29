import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse} from '../support/home-preview-data.mjs';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-83 首页球面与运营商持续动画',()=>{
 for(const reducedMotion of ['reduce','no-preference'] as const){
  test('TC-83'+(reducedMotion==='reduce'?'a':'b')+': '+reducedMotion+'下球面旋转、三行连续滚动及悬停恢复',async({page},info)=>{
   await page.emulateMedia({reducedMotion});
   await page.route('**/x/**',async route=>{
    const request=route.request(),response=previewResponse(new URL(request.url()),request.method());
    await route.fulfill({status:response.status,json:{code:0,data:response.data}});
   });
   const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
   const site=new HomeDetailPage(page);await site.open('/');
   await expect(page.locator('.network_canvas')).toHaveAttribute('data-globe-state','ready');
   await site.focusSection('.container_bg27');
   const canvas=page.locator('#network_canvas');
   const firstFrame=await canvas.screenshot();
   await expect.poll(async()=>Buffer.compare(firstFrame,await canvas.screenshot())).not.toBe(0);
   // Do not disable page animations here: that would reset the carrier tracks
   // before their pause/resume assertions below.
   await page.screenshot({path:info.outputPath('globe.png')});
   await site.focusSection('.container_bg30');await site.leavePartners();
   await expect(page.locator('.partners-item-container:visible')).toHaveCount(3);
   const before=await site.tracks();
   expect(before.every(row=>row.width>=row.viewport&&row.duplicateHidden)).toBe(true);
   const tracks=page.locator('.partners-item-row > .partners-item-page');
   await expect(tracks).toHaveCount(6);
   for(const track of await tracks.all()){
    await expect(track).toHaveCSS('animation-name','scroll');
    await expect(track).toHaveCSS('animation-duration','60s');
    await expect(track).toHaveCSS('animation-play-state','running');
   }
   await expect.poll(async()=> (await site.tracks()).every((row,i)=>row.transform!==before[i].transform)).toBe(true);
   await site.hoverPartners();
   const first=tracks.first();await expect(first).toHaveCSS('animation-play-state','paused');
   await first.evaluate(async element=>{await Promise.all(element.getAnimations().map(animation=>animation.ready));});
   const paused=(await site.tracks())[0].transform;
   await page.waitForTimeout(250);expect((await site.tracks())[0].transform).toBe(paused);
   await site.leavePartners();await expect(first).toHaveCSS('animation-play-state','running');
   await expect.poll(async()=> (await site.tracks())[0].transform).not.toBe(paused);
   await site.captureSection('.container_bg30',info.outputPath('carriers.png'));
   expect(errors).toEqual([]);
  });
 }
});
