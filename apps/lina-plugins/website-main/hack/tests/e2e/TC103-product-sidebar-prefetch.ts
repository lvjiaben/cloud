import {test,expect} from '../support/live-commerce';

test.use({baseURL:'http://127.0.0.1:6680'});
test.describe('TC-103: 商品目录左侧服务器快速显示',()=>{
 const category={categoryNo:'ctg-prefetch-test',nameZhCN:'测试服务器分类',nameEnUS:'Test Servers'};
 const makeProduct=(slug:string,name:string)=>({publicSlug:slug,categoryNo:category.categoryNo,nameZhCN:name,nameEnUS:name,summaryZhCN:'本地验证服务器',summaryEnUS:'Local validation server',availability:'available',currencies:['CNY'],inventory:{mode:'unlimited'},minimumConfigurationMonthlyPrice:{currencyCode:'CNY',amountMinor:'900'}});
 const products=[makeProduct('prefetch-node-a','首台测试服务器'),makeProduct('prefetch-node-b','第二台测试服务器')];
 const response={code:0,data:{categories:[category],products,nextCursor:''}};

 test('目录返回后先显示服务器，后台补齐完成后停止加载',async({page},info)=>{
  let release!:()=>void,started!:()=>void;
  const gate=new Promise<void>(resolve=>{release=resolve;}),requestStarted=new Promise<void>(resolve=>{started=resolve;});
  await page.route('**/x/idc-core/api/v1/public/catalog/products**',async route=>{
   const url=new URL(route.request().url());
   if(url.searchParams.get('categoryNo')===category.categoryNo&&url.searchParams.get('limit')==='12'){
    started();await gate;await route.fulfill({json:response});return;
   }
   if(!url.searchParams.has('categoryNo')&&url.searchParams.get('limit')==='12'){await route.fulfill({json:response});return;}
   await route.fallback();
  });
  try{
   await page.goto('/products.html');await requestStarted;
   const group=page.locator('#server-group-'+category.categoryNo);
   await expect(group.locator('.sky-cart-submenu-item')).toHaveCount(products.length);
   await expect(group).toContainText('首台测试服务器');
   await expect(group.locator('.lina-server-menu-feedback')).toBeEmpty();
   await page.screenshot({path:info.outputPath('sidebar-products-before-prefetch-finishes.png')});
  }finally{release();}
  const group=page.locator('#server-group-'+category.categoryNo);
  await expect(group.locator('.lina-server-menu-feedback')).toBeEmpty();
  await expect(group.locator('.sky-cart-submenu-item')).toHaveCount(products.length);
  await expect.poll(()=>page.evaluate(()=>JSON.parse(localStorage.getItem('lina.public-catalog.v1')||'{}').products?.length)).toBe(products.length);
  await page.screenshot({path:info.outputPath('sidebar-products-after-prefetch.png')});
 });

});
