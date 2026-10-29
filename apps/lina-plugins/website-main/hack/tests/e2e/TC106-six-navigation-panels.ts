import {test,expect} from '../support/source-site';
import {NavigationPage} from '../pages/NavigationPage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'reduce',headless:true});
const menus=['产品与服务','解决方案','生态合作','应用市场','支持与服务','了解我们'];
test('TC-106: 六个顶部面板展示本方数据并让缺失目标进入本地404',async({page},info)=>{
 await page.route('**/x/idc-portal/api/v1/public/brand',route=>route.fulfill({json:{code:0,data:{brand:{navigation:[{label:'首页',href:'/'},{label:'产品与服务',href:'/products.html'},{label:'生态合作',group:true,children:[{label:'推广返利',href:'http://127.0.0.1:6032/account/referrals',external:true}]}]}}}}));
 await page.route('**/x/idc-core/api/v1/public/catalog/products?*',route=>route.fulfill({json:{code:0,data:{categories:[{categoryNo:'own',nameZhCN:'本方云服务器'}],products:[{publicSlug:'own-server',categoryNo:'own',nameZhCN:'自家演示服务器',summaryZhCN:'接口返回的配置说明',showNewBadge:true}]}}}));
 const site=new NavigationPage(page);await site.open('/');
 for(const name of menus){
  if(name==='应用市场'){await expect(page.locator('.horizontal-menu-navbar-menu>li>a').filter({hasText:name})).toHaveAttribute('href','http://localhost:6680/404.html');continue;}
  await site.hoverTitle(name);const panel=site.panel();await expect(panel).toBeVisible();await expect(panel).toContainText(name);
  if(name!=='产品与服务')await expect(panel.locator('.Submenu-menuInfo .Submenu-desc')).not.toBeEmpty();
  if(name==='产品与服务'){
   const product=panel.getByRole('link',{name:'自家演示服务器 NEW 接口返回的配置说明'});await expect(product).toHaveAttribute('href','http://localhost:6680/products.html?product=own-server');
   await expect(panel.locator('.Submenu-menuInfo .Submenu-desc')).toHaveCount(0);
   await expect(panel.getByText('本方云服务器',{exact:true})).toBeVisible();
   for(const category of ['物理机APS','裸金属高性能','服务器托管业务'])await expect(panel.getByText(category,{exact:true})).toHaveCount(0);
   await page.screenshot({path:info.outputPath('products.png')});
  }else if(name==='生态合作'){
   await expect(panel).toContainText('推广合作');await expect(panel).not.toContainText('30%');await expect(panel).not.toContainText('www.xiaobaiyun.cn');
   await page.screenshot({path:info.outputPath('ecosystem.png')});
  }else if(name==='解决方案'){
   await expect(panel).toContainText('电商解决方案');await site.selectNavigationCategory('通用解决方案');await expect(panel).toContainText('工业解决方案');await page.screenshot({path:info.outputPath('solutions.png')});
  }else if(name==='支持与服务'){
   await expect(panel).toContainText('帮助文档');await expect(panel).toContainText('宝塔面板');await page.screenshot({path:info.outputPath('support.png')});
  }else if(name==='了解我们'){
   await expect(panel).toContainText('公司简介');await expect(panel).toContainText('增值许可证查询');await page.screenshot({path:info.outputPath('about.png')});
  }else await page.screenshot({path:info.outputPath('market.png')});
  await site.escape();
 }
 await page.goto('/404.html');await expect(page.getByRole('heading',{name:'页面不存在或暂未开放'})).toBeVisible();
 const market=await page.request.get('http://127.0.0.1:6680/market.html');expect(market.status()).toBe(404);await expect(market.text()).resolves.toContain('页面不存在或暂未开放');
 await page.setViewportSize({width:390,height:844});await site.open('/');await site.toggleMenu();await site.clickTitle('支持与服务');await expect(site.panel()).toBeVisible();const panel=await site.panel().boundingBox();expect(panel!.x).toBeGreaterThanOrEqual(0);expect(panel!.x+panel!.width).toBeLessThanOrEqual(390);await page.screenshot({path:info.outputPath('mobile-support.png')});
});
