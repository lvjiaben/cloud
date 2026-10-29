import {test,expect} from '../support/source-site';
import {NavigationMotionPage} from '../pages/NavigationMotionPage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'no-preference'});
test.describe('TC-75 导航动画节奏',()=>{
 test.beforeEach(async({page})=>{
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{navigation:['生态合作','支持与服务','了解我们'].map(label=>({label,group:true,children:[{label:'本方帮助',href:'/help.html'}]}))}}}}));
 });
 test('TC-75a: 延迟、平滑展开收起、快速划过及键盘',async({page},info)=>{
  const site=new NavigationMotionPage(page);await site.open('/');await expect(page.locator('.lina-nav-panel')).toHaveCount(3);await site.observe();
  await site.hoverTitle('生态合作');await page.waitForTimeout(60);await site.leaveNavigation();await page.waitForTimeout(240);
  expect((await site.events()).filter((e:any)=>e.type==='slide')).toHaveLength(0);
  await site.hoverTitle('支持与服务');await expect(site.panel()).toBeVisible();await site.settle();
  let events=await site.events(),slide=events.find((e:any)=>e.type==='slide'),enter=events.filter((e:any)=>e.type==='enter').at(-1);
  expect(slide.at-enter.at).toBeGreaterThanOrEqual(185);expect(slide.at-enter.at).toBeLessThan(400);expect(slide.duration).toBe(150);
  expect(slide.samples.some((h:number)=>h>5&&h<Math.max(...slide.samples)-5)).toBeTruthy();
  await site.enterPanel();await expect(site.panel()).toBeVisible();await site.screenshot(info.outputPath('opened.png'));
  await site.leaveNavigation();await site.settle();await expect(page.locator('.lina-nav-panel:visible')).toHaveCount(0);
  events=await site.events();slide=events.filter((e:any)=>e.type==='slide').at(-1);expect(slide.duration).toBe(150);expect(slide.samples[0]).toBeGreaterThan(slide.samples.at(-1));
  await site.focusGroup(2);await expect(site.panel().locator('h2')).toHaveText('了解我们');await site.escape();await site.settle();await expect(page.locator('.lina-nav-panel:visible')).toHaveCount(0);await expect(page.locator('[aria-expanded]').nth(2)).toBeFocused();
 });
 test('TC-75b: 动画中反向打开及减少动态',async({page},info)=>{
  const site=new NavigationMotionPage(page);await site.open('/');await expect(page.locator('.lina-nav-panel')).toHaveCount(3);await site.observe();
  await site.focusGroup(0);await page.waitForTimeout(40);await site.leaveNavigation();await site.focusGroup(1);await site.focusGroup(0);await site.settle();
  await expect(site.panel().locator('h2')).toHaveText('生态合作');await expect(page.locator('.lina-nav-panel:visible')).toHaveCount(1);
  await site.escape();await site.settle();await page.emulateMedia({reducedMotion:'reduce'});
  const count=(await site.events()).filter((e:any)=>e.type==='slide').length;
  await site.focusGroup(1);await expect(site.panel()).toBeVisible();await site.escape();await expect(page.locator('.lina-nav-panel:visible')).toHaveCount(0);
  expect((await site.events()).filter((e:any)=>e.type==='slide')).toHaveLength(count);await site.screenshot(info.outputPath('closed.png'));
 });
});
