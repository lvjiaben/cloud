import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/WebsitePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},customerOrigin:'http://localhost:6032',apiOrigin:'http://127.0.0.1:6030'});
test.describe('TC-1 原站数据隔离与明确失败',()=>{
 test('TC-1a: 商品接口失败后不显示原站商品、价格或品牌',async({page})=>{
  const site=new WebsitePage(page);const remote:string[]=[];
  page.on('request',r=>{if(!new URL(r.url()).hostname.match(/^(127\.0\.0\.1|localhost)$/))remote.push(r.url());});
  await page.route('**/x/**',route=>route.abort('failed'));
  await site.open('/products.html');await expect(page.locator('[data-status]')).toHaveText('商品加载失败，请刷新重试。');
  await expect(page.locator('[data-products]')).toBeEmpty();await expect(page.locator('body')).not.toContainText(/\u5c0f\u767d\u4e91|白猿|As9929|61\.00|成都西信/);
  expect(remote).toEqual([]);await site.capture('products-failure-desktop');
  await site.setViewport(390,844);await site.capture('products-failure-mobile');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBeTruthy();
 });
 test('TC-1b: 配置及内容接口失败时不能加车或显示原内容',async({page})=>{
  const site=new WebsitePage(page);await page.route('**/x/**',route=>route.abort('failed'));
  await site.open('/configure.html?product=not-a-real-product');await expect(page.locator('[data-status]')).toHaveText('无法读取商品配置，请重试。');await expect(page.locator('[data-add]')).toBeDisabled();await expect(page.locator('[data-fields]')).toBeEmpty();await site.capture('configure-failure');
  await site.open('/news.html');await expect(page.locator('[data-status]')).toHaveText('内容加载失败，请重试。');await expect(page.locator('[data-content-list]')).toBeEmpty();await site.capture('news-failure');
 });
});

