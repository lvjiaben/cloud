import {test,expect} from '../support/source-site';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';
import {HomeDetailPage} from '../pages/HomeDetailPage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
const titles=['全天候品质服务','极速服务应答','客户价值为先','全方位安全保障'];
test.describe('TC-40 授权静态展示与跨页页脚',()=>{
 test('TC-40a: 十页共享四图四文，不接受旧说明配置覆盖或点击跳转',async({page},info)=>{
  const site=new HomeDetailPage(page),errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
  await page.route('**/x/**',async route=>{const r=route.request(),url=new URL(r.url()),result=previewResponse(url,r.method());
   if(url.pathname.endsWith('/brand'))result.data={brand:{...previewBrand,templateConfig:{...previewBrand.templateConfig,promises:'不应覆盖静态原文 | javascript:alert(1)'}}};
   await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});
  });
  for(const path of ['index.html','products.html','configure.html','news.html','announcements.html','help.html','downloads.html','content.html','about.html','contact.html']){
   await site.open('/'+path);await expect(page.locator('[data-brand-name]').first()).toHaveText('模拟预览 · 官网模板');
   await expect(page.locator('.sky-footer-top')).toBeVisible();await expect(page.locator('.sky-footer-title')).toHaveText(titles);
   await expect.poll(()=>page.locator('.sky-footer-top img').evaluateAll(images=>images.length===4&&images.every(image=>(image as HTMLImageElement).complete&&(image as HTMLImageElement).naturalWidth>0))).toBe(true);
   await expect(page.locator('.sky-footer-top [href],.sky-footer-top [onclick],.sky-footer-top [tabindex]')).toHaveCount(0);
   await expect(page.locator('.sky-footer-top')).toHaveCSS('display','flex');
   await expect(page.locator('.sky-footer-top')).toHaveCSS('border-bottom-color','rgb(58, 58, 58)');
   for(const title of await page.locator('.sky-footer-title').all())await expect(title).toHaveCSS('color','rgb(255, 255, 255)');
   for(const icon of await page.locator('.sky-footer-top img').all())await expect(icon).toHaveCSS('width','38px');
   const tops=await page.locator('.sky-footer-block').evaluateAll(cards=>cards.map(card=>Math.round(card.getBoundingClientRect().top)));
   expect(new Set(tops).size).toBe(1);
   expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(1440);
   await site.focusSection('.sky-footer-top');const screenshot=await site.capture('static-footer-'+path.replace('.html',''),false);
   await info.attach(path,{body:JSON.stringify({screenshot,titles}),contentType:'application/json'});
  }
  expect(errors).toEqual([]);
 });
 test('TC-40b: 方案页签保留原特征，能力四组标签按原文显示',async({page},info)=>{
  const site=new HomeDetailPage(page);let long=false;
  await page.route('**/x/**',async route=>{const r=route.request(),url=new URL(r.url()),result=previewResponse(url,r.method());
   if(long&&url.pathname.endsWith('/brand')){const config={...previewBrand.templateConfig};config.solutions=config.solutions.split('\n').map(row=>{const cells=row.split('|');cells[1]='受控长摘要用于验证分隔线不溢出。'.repeat(20);return cells.join('|');}).join('\n');result.data={brand:{...previewBrand,templateConfig:config}};}
   await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});
  });
  for(long of [false,true]){
   await site.open('/index.html');await expect(page.locator('.main-option-name')).toHaveCount(7);
   await expect(page.locator('.skills-experience li')).toHaveCount(24);
   for(const index of [0,1,2]){
    await site.solution(index);await expect(page.locator('.main-option-name').nth(index)).toHaveAttribute('aria-selected','true');await expect(page.locator('.main-option-line:visible .skills-experience')).toBeVisible();
    const geometry=await page.locator('.main-option-line:visible .reviews').evaluate(summary=>({width:summary.getBoundingClientRect().width,parent:summary.parentElement!.getBoundingClientRect().width}));
    expect(Math.abs(geometry.width-geometry.parent)).toBeLessThanOrEqual(1);expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(1440);
   }
  }
  await site.captureSection('.main-option-line.active',info.outputPath('23-static-features.png'));
  expect(await page.locator('.chooseUs-itme-label').evaluateAll(groups=>groups.map(group=>group.children.length))).toEqual([3,3,4,5]);
  await expect(page.locator('.skills-experience a,.chooseUs-itme-label a')).toHaveCount(0);
 });
});
