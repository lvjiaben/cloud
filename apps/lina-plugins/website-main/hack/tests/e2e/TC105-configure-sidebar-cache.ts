import {test,expect} from '../support/live-commerce';
import {ProductSidebarPage} from '../pages/ProductSidebarPage';

test.use({baseURL:'http://127.0.0.1:6680'});
test.describe('TC-105: 配置页刷新时先显示缓存的全部分类和服务器',()=>{
 const categories=[{categoryNo:'cache-cat-a',nameZhCN:'缓存华东服务器'},{categoryNo:'cache-cat-b',nameZhCN:'缓存华南服务器'}];
 const products=[
  {publicSlug:'cache-node-a',categoryNo:categories[0].categoryNo,nameZhCN:'缓存上海服务器',availability:'available'},
  {publicSlug:'cache-node-b',categoryNo:categories[1].categoryNo,nameZhCN:'缓存广州服务器',availability:'available'},
 ];

 test('配置页先显示多分类缓存，后台刷新期间商品仍可见，接口成功后分类更新',async({page},info)=>{
  await page.addInitScript(({categories,products})=>localStorage.setItem('lina.public-catalog.v1',JSON.stringify({savedAt:Date.now(),categories,products})),{categories,products});
  let releaseCatalog!:()=>void,releaseProducts!:()=>void,catalogStarted!:()=>void,secondCategoryStarted!:()=>void;
  const catalogGate=new Promise<void>(resolve=>{releaseCatalog=resolve;}),productGate=new Promise<void>(resolve=>{releaseProducts=resolve;}),started=new Promise<void>(resolve=>{catalogStarted=resolve;}),prefetched=new Promise<void>(resolve=>{secondCategoryStarted=resolve;});
  await page.route('**/x/idc-core/api/v1/public/catalog/products**',async route=>{
   const url=new URL(route.request().url());
   if(url.searchParams.get('limit')==='1'){catalogStarted();await catalogGate;await route.fulfill({json:{code:0,data:{categories:categories.map((item,index)=>({...item,nameZhCN:'已更新分类'+(index+1)})),products:[],nextCursor:''}}});return;}
   const categoryNo=url.searchParams.get('categoryNo');if(categoryNo){if(categoryNo===categories[1].categoryNo)secondCategoryStarted();await productGate;const category=categories.find(item=>item.categoryNo===categoryNo);await route.fulfill({json:{code:0,data:{categories,products:products.filter(item=>item.categoryNo===categoryNo),nextCursor:''}}});return;}
   await route.fallback();
  });
  const sidebar=new ProductSidebarPage(page);
  try{
   await sidebar.openConfiguration(products[0].publicSlug);await started;
   await prefetched;
   await sidebar.expectServer(categories[0].categoryNo,products[0].publicSlug,products[0].nameZhCN);
   await expect(page.locator('[data-categories]')).toContainText('缓存华南服务器');
   await page.screenshot({path:info.outputPath('configure-sidebar-cache-before-refresh.png')});
   await sidebar.expandCategory('缓存华南服务器');
   await sidebar.expectServer(categories[1].categoryNo,products[1].publicSlug,products[1].nameZhCN);
   await page.screenshot({path:info.outputPath('configure-sidebar-other-group-cache-before-refresh.png')});
   releaseCatalog();await expect(page.locator('[data-categories]')).toContainText('已更新分类1');
   await sidebar.expectServer(categories[0].categoryNo,products[0].publicSlug,products[0].nameZhCN);
   await sidebar.expectServer(categories[1].categoryNo,products[1].publicSlug,products[1].nameZhCN);
   releaseProducts();
   await expect(page.locator('[data-categories]')).toContainText('已更新分类2');
   await page.screenshot({path:info.outputPath('configure-sidebar-cache-after-refresh.png')});
  }finally{releaseCatalog();releaseProducts();}
 });
});
