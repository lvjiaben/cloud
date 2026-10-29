import {test,expect} from '../support/live-commerce';
import {CommerceNavigationPage} from '../pages/CommerceNavigationPage';
test.use({baseURL:'http://127.0.0.1:6680',headless:true});
test.describe('TC-146 选配提示在入口及商品切换后保持一致',()=>{
 for(const mobile of [false,true])test(`TC-146${mobile?'b':'a'}: ${mobile?'手机兼容入口':'桌面直接选配'}切换商品再返回及重新读取均保留唯一提示`,async({page},info)=>{
  await page.setViewportSize({width:mobile?390:1440,height:900});
  const site=new CommerceNavigationPage(page);
  const slug='ecs-zhenjiang-balanced-draft-20261020';
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=12');
  const products=(await response.json()).data.products;
  const other=products.find((p:any)=>p.publicSlug!==slug&&p.availability==='available');expect(other).toBeTruthy();
  await site.open(mobile?'/configure.html?product='+slug:'/products.html?product='+slug+'&action=configureproduct');
  for(const [index,current] of [slug,other.publicSlug,slug].entries()){
   if(index>0){await site.chooseServer(current);await expect(page.locator('[data-products] h5')).toHaveCount(1);await site.buy();}
   await expect(page.locator('[data-quote] button')).not.toHaveCount(0,{timeout:20000});
   await expect(page.locator('.allocation-header-title h1')).toHaveText(products.find((p:any)=>p.publicSlug===current).nameZhCN);
   await expect(page.locator('[data-catalog-usage-notice]')).toHaveCount(1);
   await expect(page.locator('[data-catalog-usage-notice]')).toBeVisible();
   await expect(page.locator('[data-catalog-usage-notice]')).toContainText('温馨提示：服务器禁止涉及色情、赌博、政治、诈骗等违法违规业务');
  }
  await site.refreshConfiguration();
  await expect(page.locator('[data-quote] button')).not.toHaveCount(0,{timeout:20000});
  await expect(page.locator('[data-catalog-usage-notice]')).toBeVisible();
  await page.locator('[data-catalog-usage-notice]').scrollIntoViewIfNeeded();
  await page.screenshot({path:info.outputPath('configuration-notice.png')});
 });
});
