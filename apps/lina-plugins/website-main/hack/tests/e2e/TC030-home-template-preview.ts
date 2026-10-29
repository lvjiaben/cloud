import {test,expect} from '../support/source-site';
import {previewResponse,previewBrand} from '../support/home-preview-data.mjs';
import {mountOriginalSite,originalOrigin} from '../support/original-site';
import {HomePage} from '../pages/HomePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},reducedMotion:'reduce'});
test.describe('TC-30 原首页模板配置与隔离充实预览',()=>{
 test('TC-30a: 原结构多图、分类、方案、地域、伙伴及页脚非空交互（模拟资料）',async({page},info)=>{
  const site=new HomePage(page),errors:string[]=[],requests:string[]=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/x/**',async route=>{const r=route.request();requests.push(r.method()+' '+r.url());const result=previewResponse(new URL(r.url()),r.method());await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});});
  await mountOriginalSite(page);
  await site.open(originalOrigin+'/index.html');const original=await site.capture('home-original-1440-viewport',false);await site.inspectFullPage();const originalFull=await site.capture('home-original-1440-full');
  await site.open('/index.html');await expect(page.locator('[data-brand-name]').first()).toHaveText('模拟预览 · 官网模板');
  await expect(page.locator('[data-products] > *')).toHaveCount(12);await expect(page.locator('[data-home-news] > *')).toHaveCount(6);
  await expect(page.locator('.swiper-slide')).toHaveCount(4);await expect(page.locator('.swiper-slide:visible')).toHaveCount(1);
  await expect(page.locator('.pagination-bullet')).toHaveCount(4);expect((await page.locator('.pagination-bullet').first().boundingBox())!.width).toBe(42);
  await expect(page.getByRole('button',{name:'下一张',exact:true})).toHaveCSS('background-color','rgb(255, 255, 255)');
  await expect(page.locator('.swiper-text:visible .Title')).toHaveText(previewBrand.heroTitle.zhCN);
  await site.nextHero();await expect(page.locator('.swiper-text:visible .Title')).toHaveText(/模拟轮播 2/);
  await site.previousHero();await expect(page.locator('.swiper-text:visible .Title')).toHaveText(previewBrand.heroTitle.zhCN);
  await site.heroByKeyboard(3);await expect(page.locator('.swiper-text:visible .Title')).toHaveText(/模拟轮播 4/);await site.hero(0);
  await expect(page.locator('[data-home-blocks] > *')).toHaveCount(4);
  await expect(page.locator('[data-home-product-intro]')).toBeVisible();await expect(page.locator('[data-home-product-intro]')).toContainText('产品介绍（模拟）');
  await site.category(2);await expect(page).toHaveURL(/index.html\?categoryNo=preview-category-1$/);await expect(page.locator('[data-products] > *')).toHaveCount(3);await expect(page.locator('[data-categories] .active')).toHaveText('模拟分类 2');await site.category(0);await expect(page.locator('[data-products] > *')).toHaveCount(12);
  for(const selector of ['.container_bg15','.container_bg27','.container_bg30','.sky-footer-top'])await expect(page.locator(selector)).toBeVisible();
  await expect(page.locator('.main-option-name')).toHaveCount(7);await site.solution(2);await expect(page.locator('.sidebar-profile:visible .name')).toHaveText('安全解决方案');await expect(page.locator('.main-option-line:visible')).toContainText('防火墙');await site.solution(0);
  const tabs=await page.locator('.main-option-name').evaluateAll(es=>es.map(e=>({width:e.clientWidth,scrollWidth:e.scrollWidth})));for(const tab of tabs)expect(tab.scrollWidth).toBeLessThanOrEqual(tab.width);
  await expect(page.locator('.network .feature-item')).toHaveCount(3);await expect(page.locator('.network_canvas')).toBeHidden();
  await expect(page.locator('.chooseUs-itme')).toHaveCount(4);await expect(page.locator('.chooseUs-itme').first()).toContainText('弹性计算');await expect(page.locator('.chooseUs-container')).toBeVisible();
  await expect(page.locator('.partners-item-container:visible')).toHaveCount(3);await expect(page.locator('.partners-item:visible')).toHaveCount(6);for(const repeat of await page.locator('.partners-item-page[aria-hidden=true]').all())await expect(repeat).toBeHidden();await expect(page.locator('.sky-footer-block:visible')).toHaveCount(4);
  await site.top();const viewport=await site.capture('home-preview-1440-viewport',false);await site.inspectFullPage();const full=await site.capture('home-preview-1440-full');
  expect((await site.layout()).scrollWidth).toBe(1440);expect(errors).toEqual([]);expect(requests.every(r=>r.startsWith('GET '))).toBe(true);
  await info.attach('home-pair',{body:JSON.stringify({original,originalFull,viewport,full,limitations:['模拟内容不代表真实经营资料','无原横幅的无水印授权素材','未接地图坐标或地域到商品映射','不是当前已安装包验收']}),contentType:'application/json'});
 });
 test('TC-30c: 伙伴循环只重复已配置资料，悬停暂停',async({page})=>{
  await page.emulateMedia({reducedMotion:'no-preference'});
  await page.route('**/x/**',async route=>{const r=route.request(),result=previewResponse(new URL(r.url()),r.method());await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});});
  const site=new HomePage(page);await site.open('/index.html');
  const row=page.locator('.partners-item-container').first(),first=row.locator('.partners-item-page').first(),repeat=row.locator('.partners-item-page').nth(1);
  await expect(page.locator('.partners-item-container:visible')).toHaveCount(3);await expect(first.locator('.partners-item:not([data-partner-repeat])')).toHaveCount(2);await expect(repeat).toHaveAttribute('aria-hidden','true');expect(await repeat.textContent()).toBe(await first.textContent());
  await expect(first).toHaveCSS('animation-name','scroll');await expect(first).toHaveCSS('animation-play-state','running');
  await row.hover();await expect(first).toHaveCSS('animation-play-state','paused');
  const before=await first.evaluate(e=>getComputedStyle(e).transform);await page.waitForTimeout(250);expect(await first.evaluate(e=>getComputedStyle(e).transform)).toBe(before);
  await site.capture('home-partners-hover-paused',false);await page.mouse.move(0,0);await expect(first).toHaveCSS('animation-play-state','running');
  await expect.poll(()=>first.evaluate(e=>getComputedStyle(e).transform)).not.toBe(before);
 });
 test('TC-30b: 空配置不显示模拟区块，接口失败不回退预览资料',async({page})=>{
  const site=new HomePage(page);await page.route('**/x/**',r=>r.fulfill({status:503,json:{code:50,data:null}}));
  await site.open('/index.html');await expect(page.locator('.lina-brand-error')).toContainText('网站资料暂时无法加载');await expect(page.locator('[data-products] > *')).toHaveCount(0);
  for(const selector of ['.container_bg27','.container_bg30'])await expect(page.locator(selector)).toBeHidden();
  // Explicitly authorized static footer survives a failed business/brand request.
  await expect(page.locator('.sky-footer-title')).toHaveText(['全天候品质服务','极速服务应答','客户价值为先','全方位安全保障']);
  await expect(page.locator('.sky-footer-top a[href]')).toHaveCount(0);
  await expect(page.locator('body')).not.toContainText('模拟伙伴');await site.capture('home-preview-failure-no-fallback');
 });
 test('TC-30d: 展示资料格式错误不冒充有效卡片',async({page})=>{
  await page.route('**/x/**',async route=>{const r=route.request(),url=new URL(r.url()),result=previewResponse(url,r.method());
   if(url.pathname.endsWith('/brand'))result.data={brand:{...previewBrand,templateConfig:{...previewBrand.templateConfig,benefits:'缺少分隔内容',partners:Array(16).fill('过量伙伴 | -').join('\n'),solutions:'无效方案 | 说明 | javascript:alert(1) | javascript:alert(1)'}}};
   await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});
  });
  const site=new HomePage(page);await site.open('/index.html');await expect(page.locator('.chooseUs-container')).toBeVisible();await expect(page.locator('.container_bg30')).toBeHidden();
  await expect(page.locator('.sidebar-profile:visible .name')).toHaveText('电商解决方案');await expect(page.locator('.sidebar-profile:visible .whole')).toHaveAttribute('aria-disabled','true');await expect(page.locator('.sidebar-profile:visible img')).toHaveCount(1);await site.capture('home-settings-invalid-safe');
 });
 test('TC-30e: 原三排伙伴按实际数量显示，不补造资料或重复计数',async({page})=>{
  let count=0;
  await page.route('**/x/**',async route=>{const request=route.request(),url=new URL(request.url()),result=previewResponse(url,request.method());
   if(url.pathname.endsWith('/brand'))result.data={brand:{...previewBrand,templateConfig:{...previewBrand.templateConfig,partners:Array.from({length:count},(_,i)=>'受控伙伴 '+(i+1)+' | -').join('\n')}}};
   await route.fulfill({status:result.status,json:{code:result.status===200?0:result.status,data:result.data}});
  });
  const site=new HomePage(page);
  for(count of [0,1,2,4,6,7,15,16]){
   await site.open('/index.html');await expect(page.locator('[data-brand-name]').first()).toHaveText('模拟预览 · 官网模板');
   const entityCount=count>15?0:count;
   await expect(page.locator('.partners-item-container:visible')).toHaveCount(Math.min(3,entityCount));
   const entities=page.locator('.partners-item-container:visible .partners-item-page:not([aria-hidden]) .partners-item:not([data-partner-repeat]):visible');
   await expect(entities).toHaveCount(entityCount);
   if(!entityCount)await expect(page.locator('.container_bg30')).toBeHidden();
   const names=await entities.allTextContents();
   expect(names.map(name=>name.trim())).toEqual(Array.from({length:entityCount},(_,i)=>'受控伙伴 '+(i+1)));
   expect(new Set(names.map(name=>name.trim())).size).toBe(entityCount);
   const layout=await site.layout();expect(layout.scrollWidth).toBeLessThanOrEqual(layout.width);
   if([1,4,7,15,16].includes(count))await site.capture('home-partners-count-'+count);
  }
 });
});
