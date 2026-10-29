import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {CatalogLayoutPage} from '../pages/CatalogLayoutPage';

test.use({baseURL:'http://localhost:6680',websiteGateway:true,headless:true});
test.describe('TC-142 商品主体占位与无页脚布局',()=>{
 test('TC-142c: 购买配置的新旧入口在接口失败时也不显示公共页脚',async({page})=>{
  await controlledBusiness(page);const site=new CatalogLayoutPage(page);
  for(const width of [1440,390]){
   await site.setViewport(width,900);
   for(const path of ['/configure.html?product=controlled-alpha','/products.html?product=controlled-alpha&action=configureproduct']){
    await site.open(path);await expect(page.locator('body')).toHaveAttribute('data-page','configure');await expect(page.locator('[data-status]')).toHaveText('无法读取商品配置，请重试。');await expect(page.locator('.sky-footer-container')).toBeHidden();
   }
  }
 });
 for(const width of [1440,390])test(`TC-142${width===1440?'a':'b'}: ${width}px加载、失败恢复、空数据和长内容`,async({page},info)=>{
  await page.setViewportSize({width,height:900});await controlledBusiness(page);
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
  let mode='loading',release!:()=>void;const gate=new Promise<void>(resolve=>release=resolve);
  await page.route('**'+prefix+'/catalog/products**',async route=>{
   if(mode==='loading')await gate;
   if(mode==='failure')return route.fulfill({status:503,json:{code:50}});
   const product={publicSlug:'layout-long',categoryNo:'cat-a',nameZhCN:'布局测试商品',availability:'available',summaryZhCN:Array.from({length:65},(_,i)=>'长内容测试条目'+i).join('\n')};
   return route.fulfill({json:{code:0,data:{products:mode==='empty'?[]:[product],categories:mode==='empty'?[]:[{categoryNo:'cat-a',nameZhCN:'测试分类'}],nextCursor:''}}});
  });
  const site=new CatalogLayoutPage(page);
  const check=async()=>{await expect(page.locator('.sky-footer-container')).toBeHidden();const g=await site.geometry();expect(g.footerHeight).toBe(0);expect(g.mainTop).toBeGreaterThanOrEqual(112);expect(g.mainBottom).toBeGreaterThanOrEqual(g.viewport-1);expect(g.scrollWidth).toBeLessThanOrEqual(width);expect(g.background).toBe('rgba(0, 0, 0, 0)');};
  try{
   await site.open('/products.html?categoryNo=cat-a');await expect(page.locator('[data-status]')).toHaveText('正在加载商品…');await check();
   mode='failure';release();await expect(page.locator('[data-status]')).toHaveText('商品加载失败，请重试。');await check();await site.screenshot(info.outputPath('failure.png'));
   mode='empty';await site.retryCatalog();await expect(page.locator('[data-status]')).toHaveText('暂无可订购商品。');await check();
   mode='long';await site.reloadCatalog();await expect(site.product('layout-long')).toBeVisible();await check();
   if(width===1440){const scroll=await site.scrollProducts();expect(scroll.top).toBeGreaterThan(0);expect(scroll.page).toBe(0);}
   else{await expect(site.product('layout-long').locator('.buy-now-button')).toBeVisible();}
   await site.screenshot(info.outputPath('content.png'));await site.bottom();await expect(page.locator('.sky-footer-container')).toBeHidden();await site.screenshot(info.outputPath('bottom.png'));await site.top();await check();expect(errors).toEqual([]);
   await site.open('/');await expect(page.locator('.sky-footer-container')).toBeVisible();
  }finally{release();}
 });
});
