import {test,expect} from '../support/live-commerce';
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {CommerceNavigationPage} from '../pages/CommerceNavigationPage';

test.use({baseURL:'http://127.0.0.1:6680'});
test.describe('TC-95 配置表单布局与共用用户中心地址',()=>{
 test.describe.configure({timeout:60000});
 test.beforeEach(async({page})=>{
  if(process.env.WEBSITE_RECOVERY_CANDIDATE){
   for(const file of ['site.js','site.css','customer-config.js']){
    const body=await readFile(resolve(import.meta.dirname,'../../../frontend/public/lina',file),'utf8');
    await page.route('**/lina/'+file,route=>route.fulfill({contentType:file.endsWith('.css')?'text/css':'application/javascript',body}));
   }
   await page.route('**/lina/runtime-config.json',route=>route.fulfill({json:{customerUrl:'http://127.0.0.1:6032'}}));
  }
 });
 for(const width of [390,1440]){
  test(`TC-95${width===390?'a':'b'}: ${width}像素下型号及周期可读并保持权威报价`,async({page})=>{
   const site=new CommerceNavigationPage(page);await site.setViewport(width,900);
   await site.open('/products.html?product=ecs-zhenjiang-configurable-simulation&action=configureproduct');
   const terms=page.locator('[data-quote] button');await expect(terms).toHaveCount(4,{timeout:25000});
   await site.revealModels();
   await expect(page.locator('[data-models] .sky-form-label')).toHaveText('产品型号');
   await expect(page.locator('[data-models] a[aria-current=page]')).toHaveText(await page.locator('.allocation-header-title h1').innerText());
   await expect(page.locator('[data-customer="/account/cart"]')).toHaveAttribute('href','http://127.0.0.1:6032/account/cart');
   await expect(page.locator('[data-catalog-login]')).toHaveAttribute('href',/^http:\/\/127\.0\.0\.1:6032\/auth\/login/);
   console.log('TC095 models: '+await site.capture(`tc095-${width}-models`,false));
   await site.revealTerms();const layout=await site.termsLayout();
   expect(layout.width).toBeGreaterThan(width===390?200:500);
   expect(layout.scrollWidth).toBeLessThanOrEqual(Math.ceil(layout.width));
   for(const button of layout.buttons){expect(button.height).toBeLessThanOrEqual(64);expect(button.width).toBeGreaterThan(100);}
   await site.selectTerm(3);await expect(terms.last()).toHaveAttribute('aria-pressed','true');
   await expect(page.locator('[data-total-amount]')).toHaveText((await terms.last().innerText()).match(/CNY\s*[\d,.]+/)![0].replace(/^CNY /,'¥'));
   expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(width);
   console.log('TC095 terms: '+await site.capture(`tc095-${width}-terms`,false));
   await site.reload();await expect(page.locator('[data-customer="/account/cart"]')).toHaveAttribute('href','http://127.0.0.1:6032/account/cart');
  });
 }
 test('TC-95c: 型号读取失败保留当前配置并可重试切换',async({page})=>{
  const site=new CommerceNavigationPage(page);
  await page.route('**/catalog/products?*',route=>new URL(route.request().url()).searchParams.get('limit')==='12'?route.fulfill({status:503,json:{code:1}}):route.continue());
  await site.open('/products.html?product=ecs-zhenjiang-configurable-simulation&action=configureproduct');
  await expect(page.locator('.lina-server-menu-feedback').filter({hasText:'服务器列表加载失败，请重试。'})).toHaveCount(1,{timeout:25000});
  await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
  await site.revealModels();console.log('TC095 retry: '+await site.capture('tc095-model-failure',false));
  const current=await page.locator('[data-catalog-product][aria-current=page]').getAttribute('data-catalog-product'),category=await page.locator('[data-catalog-group][aria-expanded=true]').getAttribute('data-catalog-group');
  await page.unroute('**/catalog/products?*');await site.moreServers(category!);
  const alternative=page.locator('[data-catalog-product]:not([aria-current=page])').first();await expect(alternative).toBeVisible();
  const slug=(await alternative.getAttribute('data-catalog-product'))!,name=await alternative.innerText();expect(slug).not.toBe(current);
  await site.chooseServer(slug);await expect(page.locator('[data-products] .product-card')).toHaveCount(1);await site.buy();await expect(page.locator('.allocation-header-title h1')).toHaveText(name);
  await expect(page.locator('[data-quote] button')).not.toHaveCount(0);
 });
 test('TC-95d: 部署配置缺失或非法时用户中心入口明确不可用',async({page})=>{
  const site=new CommerceNavigationPage(page);
  for(const config of [null,{customerUrl:'javascript:alert(1)'},{customerUrl:'https://user:secret@example.invalid'}]){
   await page.route('**/lina/runtime-config.json',route=>config?route.fulfill({json:config}):route.fulfill({status:404,body:'missing'}));
   await site.open('/configure.html?product=ecs-zhenjiang-configurable-simulation');
   await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
   await expect(page.locator('[data-customer="/account/cart"]')).toHaveAttribute('aria-disabled','true');
   await expect(page.locator('[data-customer="/account/cart"]')).not.toHaveAttribute('href');
   await page.unroute('**/lina/runtime-config.json');
  }
 });
});
