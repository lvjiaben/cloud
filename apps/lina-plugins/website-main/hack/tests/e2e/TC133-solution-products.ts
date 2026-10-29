import {test,expect} from '../support/source-site';
import {SolutionProductsPage} from '../pages/SolutionProductsPage';
import type {Page} from '@playwright/test';

test.use({baseURL:process.env.SOLUTION_PRODUCTS_LIVE?'http://127.0.0.1:6680':'http://localhost:6680',websiteGateway:!!process.env.SOLUTION_PRODUCTS_LIVE,headless:true});
const solutions=[['electricity','electricity'],['web','web'],['game','game'],['Goingtosea','goingsea'],['financial','financial'],['government','government'],['safety','safety']];
const product=(slug:string,available=true)=>({publicSlug:slug,nameZhCN:'本方商品 '+slug,nameEnUS:'Our product '+slug,summaryZhCN:'本方后台简介',summaryEnUS:'Our catalog description',availability:available?'available':'unavailable'});
async function setup(page:Page,config:Record<string,string>,read:()=>object=()=>({products:[product('second',false),product('first'),product('unrelated')]})){
 await page.route('**/x/**',route=>route.fulfill({json:{code:0,data:{items:[],categories:[],products:[]}}}));
 await page.route('**/x/idc-portal/api/v1/public/brand',route=>route.fulfill({json:{code:0,data:{brand:{siteName:{zhCN:'本方站点',enUS:'Our site'},navigation:[],templateConfig:config}}}}));
 await page.route('**/x/idc-core/api/v1/public/catalog/products?**',route=>{const result=read();return route.fulfill({json:{code:0,data:result}});});
}
test.describe('TC-133 方案明确关联本方商品',()=>{
 test('TC-133a: 七处分别读取配置、顺序去重、剔除无关和下架、后台价格状态及英文',async({page},info)=>{
  const config=Object.fromEntries(solutions.map(([,key])=>['relatedproducts'+key,'/products.html?product=first\n/products.html?product=second\nfirst\nremoved']));
  await setup(page,config);const site=new SolutionProductsPage(page);
  for(const [path] of solutions){
   const requests:string[][]=[];page.on('request',r=>{const u=new URL(r.url());if(u.pathname.endsWith('/catalog/products')&&u.searchParams.has('publicSlugs[]'))requests.push(u.searchParams.getAll('publicSlugs[]'));});
   await site.open('/plan/'+path+'.html');await expect(site.names).toHaveText(['本方商品 first','本方商品 second']);
   expect(requests[0]).toEqual(['first','second','removed']);await expect(site.slot.locator('.web-product-price')).toHaveCount(0);
   await expect(site.slot).toContainText('暂不可售');await expect(site.slot.locator('a').first()).toHaveAttribute('href','/products.html?product=first');
  }
  await site.screenshot(info.outputPath('related-products-desktop.png'));await site.english();await expect(site.names).toHaveText(['Our product first','Our product second']);await expect(site.slot).toContainText('Currently unavailable');await expect(site.slot.locator('a').first()).toHaveText('View product');
  await site.mobile();await site.screenshot(info.outputPath('related-products-mobile-en.png'));expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 });
 test('TC-133b: 无关联不取目录，配置互相独立',async({page},info)=>{
  let reads=0;await setup(page,{relatedproductselectricity:'first'},()=>{reads++;return {products:[product('first')]};});const site=new SolutionProductsPage(page);
  await site.open('/plan/web.html');await expect(site.slot).toHaveText('暂无关联商品。');expect(reads).toBe(0);
  await site.screenshot(info.outputPath('related-products-empty.png'));await site.open('/plan/electricity.html');await expect(site.names).toHaveText(['本方商品 first']);
 });
 test('TC-133c: 加载失败清空旧数据并可重试，全部下架为空态',async({page},info)=>{
  let mode='error';await setup(page,{relatedproductsweb:'first'},()=>mode==='empty'?{products:[]}:{products:[product('first')]});
  await page.route('**/x/idc-core/api/v1/public/catalog/products?**',route=>mode==='error'?route.fulfill({status:503,json:{code:50}}):route.fallback());
  const site=new SolutionProductsPage(page);await site.open('/plan/web.html');await expect(site.slot).toContainText('商品加载失败，请重试。');await site.screenshot(info.outputPath('related-products-error.png'));
  mode='ready';await site.retry();await expect(site.names).toHaveText(['本方商品 first']);mode='empty';await page.reload();await expect(site.slot).toHaveText('暂无关联商品。');
 });
 test('TC-133d: 外站商品链接和超限配置不发起推荐查询',async({page})=>{
  let reads=0;const config={relatedproductsweb:'https://other.example/products.html?product=first'};await setup(page,config,()=>{reads++;return {products:[]};});const site=new SolutionProductsPage(page);
  await site.open('/plan/web.html');await expect(site.slot).toContainText('关联商品配置有误');expect(reads).toBe(0);
  config.relatedproductsweb=Array.from({length:25},(_,i)=>'product-'+i).join('\n');await page.reload();await expect(site.slot).toContainText('关联商品配置有误');expect(reads).toBe(0);
 });
});
