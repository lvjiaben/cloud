import {test,expect} from '@playwright/test';
import {WebsitePage} from '../pages/WebsitePage';

// Standalone public-template tests need no host authentication fixture.
// Read actual empty collections; never replace catalog/content with fixtures.
test.describe('TC-3 真实空数据与移动页脚',()=>{
 test('TC-3b: 首页空资料仍显示首屏，迁移区块不拆散商品容器',async({page,request})=>{
  const response=await request.get('/x/idc-portal/api/v1/public/brand');expect(response.ok()).toBeTruthy();
  const envelope=await response.json();expect(envelope.code).toBe(0);
  const brand=envelope.data.brand;
  test.skip(!!(brand.heroImage||brand.heroTitle?.zhCN||brand.heroTitle?.enUS||brand.heroDescription?.zhCN||brand.heroDescription?.enUS),'已有真实首屏资料，不覆盖后台配置');
  const site=new WebsitePage(page);
  for(const size of [{width:1440,height:900},{width:390,height:844}]){
   await site.setViewport(size.width,size.height);await site.open('/index.html');
   await expect(page.locator('[data-hero-title]')).toHaveText('首页资料尚未发布');
   await expect(page.locator('[data-hero-title]')).toBeVisible();
   await expect(page.locator('.systemproduct-container .systemproducts [data-products]')).toHaveCount(1);
   await expect(page.locator('[data-status]')).not.toHaveText('正在加载商品…');
   await expect(page.locator('[data-status]')).toBeInViewport();
   await expect(page.locator('.systemproducts')).toHaveCSS('opacity','1');
   const layout=await site.layout();expect(layout.width).toBe(size.width);expect(layout.scrollWidth).toBeLessThanOrEqual(size.width);
   await site.capture('home-empty-'+size.width);
  }
 });
 test('TC-3a: 空商品可见且内容页脚不引入横向溢出',async({page,request})=>{
  const catalog=await request.get('/x/idc-core/api/v1/public/catalog/products?limit=12');
  const news=await request.get('/x/idc-core/api/v1/public/content/items?type=news&limit=20');
  expect(catalog.ok()).toBeTruthy();expect(news.ok()).toBeTruthy();
  const c=await catalog.json(),n=await news.json();
  expect(c.code).toBe(0);expect(n.code).toBe(0);
  expect(Array.isArray(c.data.products)).toBeTruthy();expect(Array.isArray(n.data.items)).toBeTruthy();
  test.skip(c.data.products.length!==0||n.data.items.length!==0,'此用例仅验证真实空集合，禁止清库或伪造响应');
  const site=new WebsitePage(page);await site.setViewport(390,844);
  for(const path of ['/products.html','/news.html','/contact.html','/about.html']){
   await site.open(path);await expect(page.locator('[data-status]')).toHaveText(/暂无可订购商品。|暂无已发布内容。|暂未公布联系方式。|公司介绍、资质和服务承诺尚无公开资料。/);
   const layout=await site.layout();expect(layout.scrollWidth).toBeLessThanOrEqual(layout.width);
   if(path==='/products.html'){expect(layout.status?.top).toBeGreaterThanOrEqual(0);expect(layout.status?.bottom).toBeLessThanOrEqual(844);}
   await site.capture('empty-'+path.slice(1));
  }
 });
});
