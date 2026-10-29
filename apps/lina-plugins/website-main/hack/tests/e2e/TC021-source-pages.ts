import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/SourceWebsitePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-21 迁移十页与失败状态',()=>{
 test('TC-21a: 十页加载本插件资源且没有原站业务内容和外部请求',async({page},info)=>{
  test.setTimeout(90000);const site=new WebsitePage(page);
  const external:string[]=[],errors:string[]=[],missing:string[]=[],screenshots:string[]=[];
  page.on('request',r=>{if(new URL(r.url()).origin!=='http://localhost:6680')external.push(r.url());});
  page.on('pageerror',e=>errors.push(e.message));
  page.on('response',r=>{if(r.status()>=400&&!new URL(r.url()).pathname.startsWith('/x/'))missing.push(r.status()+' '+r.url());});
  for(const name of ['index','products','configure','news','announcements','help','downloads','content','contact','about']){
   await site.open('/'+name+'.html');
   await expect(page.locator('[data-brand-name]').first()).toHaveText('本地开发联调');
   await expect(page.locator('[data-status]')).not.toHaveText(/正在加载|正在读取/);
   await expect(page.locator('body')).not.toContainText(/小白云|白猿|成都西信|xiaobaiyun\.cn|As9929/i);
   const footer=page.locator('.sky-footer-top');
   await expect(footer).toBeHidden();
   await expect(footer.locator(':scope > .sky-footer-block')).toHaveCount(4);
   await expect(footer.locator('.sky-footer-icon')).toHaveCount(4);
   await expect(footer.locator('.sky-footer-title')).toHaveCount(4);
   expect(await footer.textContent()).toMatch(/^\s*$/);
   expect(await footer.locator('[href],[src],[srcset],[title],[alt]').count()).toBe(0);
   expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(1440);
   if(name==='index'){expect((await page.locator('.systemproduct-layoutcontainer').boundingBox())!.y).toBeLessThan(400);const title=page.locator('[data-hero-title]');await expect(title).toHaveText('首页资料尚未发布');await expect(title).toBeVisible();expect((await title.boundingBox())!.y).toBeGreaterThan(62);}
   if(['products','configure'].includes(name))expect((await page.locator('.shopping-cart-page').boundingBox())!.y).toBe(112);
   if(['contact','about'].includes(name)){expect((await page.locator('.globalBanner').boundingBox())!.height).toBeGreaterThan(200);await expect(page.locator('.et-hero-tabs a')).toBeVisible();const title=page.locator('.pagebanner-title');await expect(title).toBeVisible();expect((await title.boundingBox())!.y).toBeGreaterThan(62);}
   const search=await page.locator('.horizontal-menu-navbar-search .input-label').boundingBox();expect(search!.x+search!.width).toBeLessThanOrEqual(await page.evaluate(()=>document.documentElement.clientWidth));
   if(name==='contact')await expect(page.locator('#contactInformation')).toHaveCount(1);
   screenshots.push(await site.capture('website-main-'+name));
  }
  await info.attach('screenshots',{body:JSON.stringify(screenshots),contentType:'application/json'});
  expect(external).toEqual([]);expect(errors).toEqual([]);expect(missing).toEqual([]);
 });
 test('TC-21b: 商品接口失败明确报错并清空产品，重试恢复真实后台数据',async({page})=>{
  const site=new WebsitePage(page);
  await page.route('**/x/idc-core/api/v1/public/catalog/products?*',r=>r.fulfill({status:503,contentType:'application/json',body:JSON.stringify({code:50})}));
  await site.open('/products.html');
  await expect(page.locator('[data-status]')).toHaveText(/无法|失败|不可用/);
  await expect(page.locator('[data-products] > *')).toHaveCount(0);
  await site.capture('website-main-catalog-failure');
  await page.unroute('**/x/idc-core/api/v1/public/catalog/products?*');
  await site.retry();
  await expect(page.locator('[data-products] > *')).not.toHaveCount(0);
 });
});
