import {test,expect} from '../support/source-site';
import {NavigationPage} from '../pages/NavigationPage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'reduce'});
test.describe('TC-73 本方导航与后台一致',()=>{
 test('TC-73a: 自有分组、描述和商品接口，不复活固定广告',async({page},info)=>{
  await page.route('**/x/idc-portal/api/v1/public/brand',r=>r.fulfill({json:{code:0,data:{brand:{navigation:[{label:'支持与服务',group:true,children:[{label:'本方栏目',group:true,children:[{label:'帮助文档',href:'/help.html'},{label:'联系我们',href:'/contact.html'}]}]},{label:'生态合作',group:true,children:[{label:'推广返利',href:'http://localhost:6032/account/referrals',external:true}]},{label:'产品与服务',href:'/products.html'}]}}}}));
  await page.route('**/x/idc-core/api/v1/public/catalog/products?*',r=>r.fulfill({json:{code:0,data:{categories:[{categoryNo:'own',nameZhCN:'本方分类'}],products:[{publicSlug:'own-product',nameZhCN:'本方服务器',summaryZhCN:'规格说明',categoryNo:'own'}]}}}));
  const site=new NavigationPage(page);await site.open('/');await site.hoverTitle('支持与服务');await expect(site.panel().getByRole('button',{name:'本方栏目',exact:true})).toBeVisible();await site.selectNavigationCategory('本方栏目');await expect(site.panel().getByRole('link',{name:'联系我们',exact:true})).toHaveAttribute('href','http://localhost:6680/contact.html');await site.hoverTitle('生态合作');await expect(site.panel().getByRole('link',{name:/^推广返利/})).toHaveAttribute('href','http://localhost:6032/account/referrals');await expect(site.panel()).not.toContainText('30%');await expect(site.panel().locator('.lina-nav-promotion')).toContainText('推广合作');await page.screenshot({path:info.outputPath('own-navigation.png')});
  await site.hoverTitle('产品与服务');await expect(site.panel().locator('.Submenu-item').first()).toBeVisible();await expect(site.panel().locator('a[href*="products.html?product="]').first()).toHaveAttribute('href',/localhost:6680/);
 });
 test('TC-73c: 合作广告复用原插画与叠字布局，点击目标由本方导航决定',async({page},info)=>{
  await page.route('**/x/idc-portal/api/v1/public/brand',route=>route.fulfill({json:{code:0,data:{brand:{navigation:[{label:'生态合作',group:true,children:[{label:'推广返利',href:'http://localhost:6032/account/referrals',customer:true}]}]}}}}));
  const site=new NavigationPage(page);await site.open('/');await site.hoverTitle('生态合作');
  const ad=site.panel().getByRole('link',{name:'了解推广计划',exact:true}),image=ad.locator('img');
  await expect(ad).toHaveAttribute('href','http://localhost:6032/account/referrals');
  await expect(ad).toContainText('推广合作');await expect(ad).not.toContainText('30%');
  await expect(image).toHaveAttribute('src','/static/picture/local6624afb6bf492.png');
  await expect.poll(()=>image.evaluate((img:HTMLImageElement)=>img.complete&&img.naturalWidth===1020&&img.naturalHeight===522)).toBe(true);
  const layout=await ad.evaluate(el=>({width:el.getBoundingClientRect().width,height:el.getBoundingClientRect().height,padding:getComputedStyle(el).padding,position:getComputedStyle(el.querySelector('.advertisement-position')!).position,titleSize:getComputedStyle(el.querySelector('.advertisement-title')!).fontSize}));
  expect(layout.width).toBeCloseTo(256,0);expect(layout.height).toBeCloseTo(131,0);expect(layout.padding).toBe('0px');expect(layout.position).toBe('absolute');expect(layout.titleSize).toBe('16px');
  await ad.hover();await expect(ad.locator('.advertisement-title')).toHaveCSS('color','rgb(0, 82, 217)');
  await page.screenshot({path:info.outputPath('ecosystem-advertisement.png'),animations:'disabled'});
  await site.escape();await page.setViewportSize({width:390,height:844});await site.toggleMenu();await site.clickTitle('生态合作');
  await expect(ad).toBeVisible();const bounds=await ad.boundingBox();expect(bounds!.x).toBeGreaterThanOrEqual(0);expect(bounds!.x+bounds!.width).toBeLessThanOrEqual(390);
  await ad.screenshot({path:info.outputPath('ecosystem-advertisement-mobile.png'),animations:'disabled'});
 });
 test('TC-73b: 商品请求失败不展示参考站内容',async({page},info)=>{
  await page.route('**/x/idc-core/api/v1/public/catalog/products?*',r=>r.fulfill({status:503,json:{code:50}}));const site=new NavigationPage(page);await site.open('/');await site.hoverTitle('产品与服务');await expect(site.panel()).toContainText('商品加载失败，请重试。');await expect(site.panel().locator('a[href*="configure.html"]')).toHaveCount(0);await page.screenshot({path:info.outputPath('catalog-failure.png')});
 });
});
