import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';

test.use({baseURL:'http://localhost:6680',viewport:{width:1270,height:900},reducedMotion:'reduce'});

test.describe('TC-58 首页页签换行与原图动画',()=>{
 test.beforeEach(async({page})=>{
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());
   if(u.pathname.endsWith('/brand'))r.data={brand:{...previewBrand,navigation:[{label:{zhCN:'本方动态导航'},href:'/help.html'}]}};
   await route.fulfill({status:r.status,json:{code:0,data:r.data}});
  });
 });
 test('TC-58a: 482px七页签换行且最后一项可用',async({page},info)=>{
  const site=new HomeDetailPage(page);await site.open('/');await site.focusSection('.container_bg15');
  const strip=page.locator('.main-option'),tabs=strip.locator('.main-option-name');
  await expect(tabs).toHaveCount(7);
  const layout=await strip.evaluate(n=>({width:n.clientWidth,scroll:n.scrollWidth,height:n.clientHeight,overflow:getComputedStyle(n).overflowX,rows:new Set(Array.from(n.children).map(c=>Math.round(c.getBoundingClientRect().top))).size}));
  // Native Chrome reserves a vertical scrollbar; the same 1270 viewport has a 472px strip.
  expect(layout.width).toBeGreaterThanOrEqual(470);expect(layout.width).toBeLessThanOrEqual(482);expect(layout.scroll).toBeLessThanOrEqual(layout.width);expect(layout.height).toBeGreaterThan(80);expect(layout.rows).toBe(2);expect(layout.overflow).toBe('visible');
  await site.constrainSolutionStrip(482);
  expect(await strip.evaluate(n=>({width:n.clientWidth,scroll:n.scrollWidth,rows:new Set(Array.from(n.children).map(c=>Math.round(c.getBoundingClientRect().top))).size}))).toEqual({width:482,scroll:482,rows:2});
  await site.constrainSolutionStrip(null);
  await site.solution(6);await expect(tabs.nth(6)).toHaveAttribute('aria-selected','true');
  await expect(page.locator('.sidebar-profile:visible .name')).toHaveText('政府解决方案');
  await site.captureSection('.container_bg15',info.outputPath('desktop-solutions.png'));
  await site.setViewport(390,844);await site.focusSection('.container_bg15');
  const mobile=await strip.evaluate(n=>({width:n.clientWidth,scroll:n.scrollWidth,display:getComputedStyle(n).display,columns:getComputedStyle(n).gridTemplateColumns,children:Array.from(n.children).map(c=>({width:c.getBoundingClientRect().width,scroll:c.scrollWidth}))}));
  expect(mobile.scroll,JSON.stringify(mobile)).toBeLessThanOrEqual(mobile.width);
  await site.solution(5);await expect(tabs.nth(5)).toHaveAttribute('aria-selected','true');
  await site.captureSection('.container_bg15',info.outputPath('mobile-solutions.png'));
  await expect(page.locator('.horizontal-menu-navbar-menu')).toContainText('本方动态导航');
 });
 test('TC-58b: 四张原图加载、三精灵悬停、静态图及减少动态',async({page},info)=>{
  const site=new HomeDetailPage(page);await site.open('/');await site.focusSection('.bannerbelow-layoutcontainer');
  const cards=page.locator('[data-home-blocks] > .bannerbelow-list'),icons=cards.locator('.bannerbelow-item-icon');
  await expect(icons).toHaveCount(4);await expect(cards.locator('.bannerbelow-icon-container:visible')).toHaveCount(4);
  const files=['remoteL17137759208918.png','remoteL17137759367861.png','remoteL17137759611360.png','remoteL17137759901567.png'];
  for(const [i,file]of files.entries()){
   await expect(icons.nth(i)).toHaveCSS('background-image',new RegExp(file.replaceAll('.','[.]')));
   expect(await icons.nth(i).evaluate(async(n)=>{const image=new Image();image.src=getComputedStyle(n).backgroundImage.slice(5,-2);await image.decode();return image.naturalWidth>0;})).toBe(true);
  }
  await site.captureSection('.bannerbelow-layoutcontainer',info.outputPath('desktop-shortcuts.png'));
  await site.motion('no-preference');await site.hoverShortcut(0);
  await expect(icons.nth(0)).toHaveCSS('animation-name','animation_icon');
  const position=await icons.nth(0).evaluate(n=>getComputedStyle(n).backgroundPosition);
  await expect.poll(()=>icons.nth(0).evaluate(n=>getComputedStyle(n).backgroundPosition)).not.toBe(position);
  await site.leaveShortcut();await expect(icons.nth(0)).toHaveCSS('background-position','0px 0px');
  for(const i of [1,3]){await site.hoverShortcut(i);await expect(icons.nth(i)).toHaveCSS('animation-name','animation_icon');}
  await site.hoverShortcut(2);await expect(icons.nth(2)).toHaveCSS('animation-name','none');
  await site.motion('reduce');
  for(const i of [0,1,3]){
   await site.leaveShortcut();await site.hoverShortcut(i);await expect(icons.nth(i)).toHaveCSS('animation-name','animation_icon');
   const before=await icons.nth(i).evaluate(n=>getComputedStyle(n).backgroundPosition);
   await expect.poll(()=>icons.nth(i).evaluate(n=>getComputedStyle(n).backgroundPosition)).not.toBe(before);
   await site.leaveShortcut();await expect(icons.nth(i)).toHaveCSS('background-position','0px 0px');
  }
  await expect(cards.locator('[href]')).toHaveCount(0);
  await site.setViewport(390,844);await site.captureSection('.bannerbelow-layoutcontainer',info.outputPath('mobile-shortcuts.png'));
  expect(await cards.evaluateAll(ns=>ns.every(n=>n.getBoundingClientRect().right<=innerWidth))).toBe(true);
 });
});
