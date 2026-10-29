import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'no-preference'});
test.describe('TC-76 首页滚动动画',()=>{
 test.beforeEach(async({page})=>{
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());
   if(u.pathname.endsWith('/brand'))r.data={brand:{...previewBrand}};
   await route.fulfill({status:r.status,json:{code:0,data:r.data}});
  });
 });
 test('TC-76a: 动画区域滚入视口后播放且首屏外保持静止',async({page},info)=>{
  const site=new HomeDetailPage(page);await site.open('/');
  const news=page.locator('.comprehensive-news-right');
  await expect(news).toHaveClass(/lina-scroll-pending/);
  await expect(news).not.toHaveClass(/lina-scroll-revealed/);
  await expect(news).toHaveCSS('animation-name','none');
  await page.screenshot({path:info.outputPath('before-scroll.png')});
  await site.focusSection('.comprehensive-news-right');
  await expect(news).toHaveClass(/lina-scroll-revealed/);
  await expect(news).toHaveCSS('animation-name','slideInRight');
  await news.evaluate(async el=>{await Promise.all(el.getAnimations().map(animation=>animation.finished));});
  await page.screenshot({path:info.outputPath('after-scroll.png')});
 });
 test('TC-76b: 减少动态时能力区脉冲及进场动画静止且内容可见',async({page},info)=>{
  await page.emulateMedia({reducedMotion:'reduce'});
  const site=new HomeDetailPage(page);await site.open('/');
  const benefits=page.locator('.chooseUs-min-container');
  await expect(benefits).toHaveClass(/lina-scroll-static/);
  await expect(benefits).toBeVisible();
  await expect(benefits).toHaveCSS('animation-name','none');
  const news=page.locator('.comprehensive-news-right');
  await expect(news).toBeVisible();
  await expect(news).toHaveCSS('animation-name','none');
  await page.screenshot({path:info.outputPath('reduced-motion.png')});
 });
 test('TC-76c: 手机新闻区单列完整展示并在滚入视口后播放',async({page},info)=>{
  await page.setViewportSize({width:390,height:844});
  const site=new HomeDetailPage(page);await site.open('/');
  const news=page.locator('.comprehensive-news-right');
  await expect(news).toHaveClass(/lina-scroll-pending/);
  await expect(news).toHaveCSS('animation-name','none');
  await site.focusSection('.comprehensive-news-right');
  await expect(news).toHaveClass(/lina-scroll-revealed/);
  await expect(news).toHaveCSS('animation-name','slideInRight');
  await news.evaluate(async el=>{await Promise.all(el.getAnimations().map(animation=>animation.finished));});
  const layout=await news.evaluate(el=>{const rect=el.getBoundingClientRect(),left=document.querySelector('.comprehensive-news-left')!.getBoundingClientRect();return {width:rect.width,left:rect.left,right:rect.right,top:rect.top,leftBottom:left.bottom,viewport:innerWidth};});
  expect(layout.width).toBeGreaterThan(300);
  expect(layout.left).toBeGreaterThanOrEqual(0);
  expect(layout.right).toBeLessThanOrEqual(layout.viewport);
  expect(layout.top).toBeGreaterThanOrEqual(layout.leftBottom);
  await page.screenshot({path:info.outputPath('mobile-news.png'),animations:'disabled'});
 });
});
