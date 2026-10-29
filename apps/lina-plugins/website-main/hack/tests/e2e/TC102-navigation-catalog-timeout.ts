import {test,expect} from '../support/source-site';
import {NavigationPage} from '../pages/NavigationPage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'reduce'});
test('TC-102: 商品目录请求一直未返回时退出加载并可重试恢复',async({page},info)=>{
 let release!:()=>void;
 const pending=new Promise<void>(resolve=>{release=resolve;});
 let stalled=true;
 await page.route('**/x/idc-core/api/v1/public/catalog/products?*',async route=>{
  if(stalled)await pending;
  await route.fulfill({json:{code:0,data:{categories:[{categoryNo:'recovered-category',nameZhCN:'恢复后的分类'}],products:[{publicSlug:'recovered-product',categoryNo:'recovered-category',nameZhCN:'恢复后的服务器',summaryZhCN:'目录已恢复'}]}}});
 });
 const site=new NavigationPage(page);await site.open('/');await site.hoverTitle('产品与服务');
 try{
  await expect(site.panel()).toContainText('正在加载商品…');
  await page.screenshot({path:info.outputPath('navigation-loading.png')});
  await expect(site.panel()).toContainText('商品加载失败，请重试。',{timeout:20000});
  await page.screenshot({path:info.outputPath('navigation-timeout.png')});
 }finally{stalled=false;release();}
 await site.retryNavigation();
 const recovered=site.panel().locator('.Submenu-item').filter({hasText:'恢复后的服务器'});await expect(recovered).toHaveAttribute('href','http://localhost:6680/products.html?product=recovered-product');
 await page.screenshot({path:info.outputPath('navigation-recovered.png')});
});
