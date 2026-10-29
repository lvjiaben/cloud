import {test,expect} from '../support/live-commerce';
import {CommerceNavigationPage} from '../pages/CommerceNavigationPage';

test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1670,height:950}});
test.describe('TC-96 原站五区结构与本方分组服务器菜单',()=>{
 test.describe.configure({timeout:60000});
 test('TC-96a: 一级真实分类展开，二级服务器进入卡片及配置，历史保持外壳',async({page})=>{
  const site=new CommerceNavigationPage(page),documents:string[]=[];
  page.on('request',request=>{if(request.isNavigationRequest()&&request.resourceType()==='document')documents.push(request.url());});
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=12'),{data}=await response.json();
  const first=data.products.find((product:any)=>product.availability==='available'),category=data.categories.find((item:any)=>item.categoryNo===first.categoryNo);
  const categoryResponse=await page.request.get('/x/idc-core/api/v1/public/catalog/products?'+new URLSearchParams({categoryNo:category.categoryNo,limit:'12'}));
  const expected=(await categoryResponse.json()).data.products;
  await site.open('/products.html?categoryNo='+category.categoryNo);
  await expect(page.locator('[data-catalog-group]')).toHaveCount(data.categories.length);
  await expect(site.group(category.nameZhCN)).toHaveAttribute('aria-expanded','true');
  for(const product of expected)await expect(site.server(product.publicSlug)).toHaveText(product.nameZhCN||product.nameEnUS);
  const before=page.url();await site.toggle(category.nameZhCN);await expect(site.group(category.nameZhCN)).toHaveAttribute('aria-expanded','false');expect(page.url()).toBe(before);
  await site.toggle(category.nameZhCN);await site.chooseServer(first.publicSlug);
  await expect(page.locator('[data-products] .product-card')).toHaveCount(1);await expect(page.locator('[data-product="'+first.publicSlug+'"] h5')).toHaveText(first.nameZhCN);
  await expect(site.server(first.publicSlug)).toHaveAttribute('aria-current','page');
  await expect(page.locator('.nav-top-page .return')).toHaveText('返回');await expect(page.locator('[data-catalog-title]')).toHaveText('区域（'+first.nameZhCN+'）');
  await expect(page.locator('[data-catalog-context-note]')).toHaveText(first.summaryZhCN);
  console.log('TC096 list: '+await site.capture('tc096-reference-card',false));
  await site.buy();await expect(page.locator('.allocation-header-title h1')).toHaveText(first.nameZhCN);
  await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
  await expect(site.server(first.publicSlug)).toHaveAttribute('aria-current','page');
  await expect(page.locator('[data-models] a')).toHaveCount(1);await expect(page.locator('[data-models] .basicConfigurationItems')).toBeVisible();
  await expect(page.locator('[data-refresh]')).toBeHidden();await expect(page.locator('[data-product-summary]')).toBeHidden();
  await expect(page.locator('.lina-os-controls .os-card')).toHaveCount(6);
  const boxes=await page.locator('.lina-os-controls .os-card').evaluateAll(cards=>cards.map(card=>card.getBoundingClientRect().toJSON()));
  expect(new Set(boxes.slice(0,4).map(box=>box.y)).size).toBe(1);expect(boxes[4].y).toBeGreaterThan(boxes[0].y);
  console.log('TC096 configure: '+await site.capture('tc096-reference-configure',false));
  await site.selectTerm(0);await expect(page.locator('[data-total-amount]')).toHaveText((await page.locator('[data-quote] button').first().innerText()).match(/CNY\s*[\d,.]+/)![0].replace(/^CNY /,'¥'));
  await site.toggleFees();await expect(page.locator('[data-breakdown]')).toContainText('应付合计');console.log('TC096 fees: '+await site.capture('tc096-reference-fees',false));await site.toggleFees();
  await site.browserBack();await expect(page.locator('[data-products] .product-card')).toHaveCount(1);await site.browserForward();await expect(page.locator('[data-quote] button')).toHaveCount(4);
  expect(documents).toHaveLength(1);await site.reload();await expect(site.server(first.publicSlug)).toHaveAttribute('aria-current','page');
 });
 test('TC-96b: 单开分组、二级列表分页与失败重试不影响已打开配置',async({page})=>{
  const site=new CommerceNavigationPage(page);await site.open('/configure.html?product=ecs-zhenjiang-configurable-simulation');
  await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=12'),{data}=await response.json();
  const category=data.categories.find((item:any)=>item.nameZhCN==='云服务器演示'),url='**/catalog/products?categoryNo='+category.categoryNo+'&limit=12*';
  await page.route(url,route=>route.fulfill({status:503,json:{code:1}}));await site.toggle(category.nameZhCN);
  const group=page.locator('#server-group-'+category.categoryNo);
  await expect(group.locator('.lina-server-menu-feedback')).toHaveText('服务器列表加载失败，请重试。');
  await expect(page.locator('[data-catalog-group][aria-expanded=true]')).toHaveCount(1);await expect(page.locator('[data-quote] button')).toHaveCount(4);
  console.log('TC096 failure: '+await site.capture('tc096-sidebar-failed',false));
  await page.unroute(url);await site.moreServers(category.categoryNo);
  const expectedResponse=await page.request.get('/x/idc-core/api/v1/public/catalog/products?'+new URLSearchParams({categoryNo:category.categoryNo,limit:'12'})),expected=(await expectedResponse.json()).data;
  await expect(group.locator('[data-catalog-product]')).toHaveCount(expected.products.length);
  if(expected.nextCursor){await site.moreServers(category.categoryNo);await expect.poll(()=>group.locator('[data-catalog-product]').count()).toBeGreaterThan(expected.products.length);}
  await expect(group.locator('.lina-server-menu-feedback')).toBeEmpty();
 });
 test('TC-96d: 二级服务器翻页失败后保留首屏，重试补齐且不重复',async({page})=>{
  const site=new CommerceNavigationPage(page),response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=12'),{data}=await response.json();
  const detail=await page.request.get('/x/idc-core/api/v1/public/catalog/products/ecs-zhenjiang-configurable-simulation'),current=(await detail.json()).data.product;
  const category=data.categories.find((item:any)=>item.categoryNo!==current.categoryNo);
  const product={...current,categoryNo:category.categoryNo,publicSlug:'controlled-sidebar-page-one',nameZhCN:'分页服务器甲'},second={...product,publicSlug:'controlled-sidebar-page-two',nameZhCN:'分页服务器乙'};
  let failed=true;
  await page.route('**/catalog/products?categoryNo='+category.categoryNo+'&limit=12*',route=>{
   const cursor=new URL(route.request().url()).searchParams.get('cursor');
   if(cursor&&failed)return route.fulfill({status:503,json:{code:1}});
   return route.fulfill({json:{code:0,data:{products:cursor?[product,second]:[product],nextCursor:cursor?'':'controlled-next-page'}}});
  });
  await site.open('/configure.html?product='+current.publicSlug);await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});await site.toggle(category.nameZhCN);
  const group=page.locator('#server-group-'+category.categoryNo);
  await expect(group.locator('[data-catalog-product]')).toHaveCount(1);await site.moreServers(category.categoryNo);
  await expect(group.locator('.lina-server-menu-feedback')).toHaveText('服务器列表加载失败，请重试。');await expect(group.locator('[data-catalog-product]')).toHaveCount(1);
  console.log('TC096 pagination failure: '+await site.capture('tc096-pagination-failure',false));
  failed=false;await site.moreServers(category.categoryNo);await expect(group.locator('[data-catalog-product]')).toHaveCount(2);
  await expect(site.server(second.publicSlug)).toHaveText(second.nameZhCN);await expect(group.locator('.lina-server-menu-more')).toBeHidden();await expect(page.locator('[data-quote] button')).toHaveCount(4);
 });
 test('TC-96c: 手机保留两级菜单与可操作配置和固定费用栏',async({page})=>{
  const site=new CommerceNavigationPage(page);await site.setViewport(390,900);await site.open('/configure.html?product=ecs-zhenjiang-configurable-simulation');
  await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});await expect(site.server('ecs-zhenjiang-configurable-simulation')).toHaveAttribute('aria-current','page');
  await site.revealTerms();await site.selectTerm(3);await expect(page.locator('[data-add]')).toBeEnabled();
  const box=await page.locator('.allocation-footer').boundingBox();expect(Math.abs(box!.y+box!.height-900)).toBeLessThan(2);expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(390);
  for(const button of (await site.termsLayout()).buttons)expect(button.height).toBeLessThan(65);
  console.log('TC096 mobile: '+await site.capture('tc096-reference-mobile',false));
 });
});
