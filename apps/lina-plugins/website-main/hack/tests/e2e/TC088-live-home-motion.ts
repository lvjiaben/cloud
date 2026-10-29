import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';

test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:900}});
test.describe('TC-88 实际官网轮播与快捷卡动画',()=>{
 for(const mode of ['no-preference','reduce'] as const)test('TC-88 '+mode,async({page},info)=>{
  await page.emulateMedia({reducedMotion:mode});
  if(process.env.WEBSITE_ALIGNMENT_CANDIDATE){const body=await readFile(resolve(process.env.WEBSITE_ALIGNMENT_CANDIDATE,'ordinary-after-lina-site.css'));await page.route('**/lina/site.css',route=>route.fulfill({contentType:'text/css',body}));}
  if(process.env.WEBSITE_MOTION_CANDIDATE)for(const asset of ['lina/server-hero.js','lina/site.css']){
   const body=await readFile(resolve(process.env.WEBSITE_MOTION_CANDIDATE,'ordinary-after-'+asset.replaceAll('/','-')));
   await page.route('**/'+asset,route=>route.fulfill({contentType:asset.endsWith('.css')?'text/css':'application/javascript',body}));
  }
  const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=4');expect(response.ok(),'公开目录 HTTP '+response.status()).toBe(true);
  const rows=(await response.json()).data.products;expect(rows.length).toBeGreaterThan(1);
  await page.goto('/');
  const active=page.locator('.swiper-text:visible .Title'),hero=page.locator('.Rotation-layoutcontainer');
  await expect(active).toHaveText(rows[0].nameZhCN||rows[0].nameEnUS);
  const titles=hero.locator('.swipervertical-title');
  const alignment=()=>titles.evaluateAll(nodes=>nodes.map(n=>{const range=document.createRange();range.selectNodeContents(n);const text=range.getBoundingClientRect(),box=n.getBoundingClientRect(),style=getComputedStyle(n),parent=n.parentElement!.getBoundingClientRect();return {align:style.textAlign,textRight:text.right,contentRight:box.right-parseFloat(style.paddingRight),width:box.width,parentWidth:parent.width,naturalWidth:text.width+parseFloat(style.paddingLeft)+parseFloat(style.paddingRight)};}));
  const initial=await alignment();expect(initial.every(n=>n.align==='right')).toBe(true);
  expect(initial.every(n=>Math.abs(n.textRight-n.contentRight)<1)).toBe(true);
  expect(initial.every(n=>n.width<n.parentWidth-10&&Math.abs(n.width-n.naturalWidth)<1)).toBe(true);
  await expect(active).toHaveText(rows[1].nameZhCN||rows[1].nameEnUS,{timeout:7500});
  await expect(hero.locator('.swipervertical-title.active')).toHaveText(rows[1].nameZhCN||rows[1].nameEnUS);
  await expect(hero.locator('.swiper-slide.active')).toHaveAttribute('data-index','1');
  const changed=await alignment();expect(changed.every(n=>n.align==='right')).toBe(true);
  expect(changed.every(n=>Math.abs(n.textRight-n.contentRight)<1)).toBe(true);
  expect(changed.every(n=>n.width<n.parentWidth-10&&Math.abs(n.width-n.naturalWidth)<1)).toBe(true);
  const progress=hero.locator('.pagination-bullet-active');
  expect(await progress.evaluate(n=>getComputedStyle(n,'::after').animationName)).toBe('bulletProgress');
  await page.screenshot({path:info.outputPath('second-server.png')});
  const cards=page.locator('[data-home-blocks] > .bannerbelow-list'),icons=cards.locator('.bannerbelow-item-icon');
  for(const index of [0,1,3]){
   await page.mouse.move(0,0);await cards.nth(index).hover();
   await expect(icons.nth(index)).toHaveCSS('animation-name','animation_icon');
   const initial=await icons.nth(index).evaluate(n=>getComputedStyle(n).backgroundPosition);
   await expect.poll(()=>icons.nth(index).evaluate(n=>getComputedStyle(n).backgroundPosition)).not.toBe(initial);
   await page.screenshot({path:info.outputPath('shortcut-'+index+'.png')});
   await page.mouse.move(0,0);await expect(icons.nth(index)).toHaveCSS('background-position','0px 0px');
  }
  await cards.nth(2).hover();await expect(icons.nth(2)).toHaveCSS('animation-name','none');
  expect(errors).toEqual([]);
 });
});
