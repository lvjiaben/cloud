import {test,expect} from '../support/live-commerce';
import {CommerceNavigationPage} from '../pages/CommerceNavigationPage';

test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:900}});
test.describe('TC-84 实际官网分类下展开服务器',()=>{
 test('TC-84a: 左侧两级服务器切换卡片及配置，右侧只保留当前型号',async({page})=>{
  const site=new CommerceNavigationPage(page),slug='ecs-shanghai-configurable-simulation';
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products/'+slug);
  expect(response.ok()).toBe(true);const {product}=(await response.json()).data;
  const catalog=await page.request.get('/x/idc-core/api/v1/public/catalog/products?categoryNo='+encodeURIComponent(product.categoryNo)+'&limit=12');
  const {products}=(await catalog.json()).data,expected=products.filter((p:any)=>p.categoryNo===product.categoryNo&&p.availability==='available');
  expect(expected.length).toBeGreaterThan(1);await site.open('/configure.html?product='+slug);
  await expect(page.locator('[data-quote] button')).toHaveCount(4);
  for(const server of expected)await expect(site.server(server.publicSlug)).toHaveText(server.nameZhCN||server.nameEnUS);
  await expect(site.server(slug)).toHaveAttribute('aria-current','page');await expect(page.locator('[data-models] a')).toHaveCount(1);
  console.log('TC084 desktop: '+await site.capture('tc084-live-models-desktop',false));
  const sibling=expected.find((p:any)=>p.publicSlug!==slug);await site.chooseServer(sibling.publicSlug);await site.buy();
  await expect(page.locator('.allocation-header-title h1')).toHaveText(sibling.nameZhCN||sibling.nameEnUS);
  await expect(site.server(sibling.publicSlug)).toHaveAttribute('aria-current','page');await expect(page.locator('[data-models] a')).toHaveCount(1);
  await expect(page.locator('[data-quote] button')).toHaveCount(4);await site.setViewport(390,844);await site.revealModels();
  expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(390);console.log('TC084 mobile: '+await site.capture('tc084-live-models-mobile',false));
 });
});
