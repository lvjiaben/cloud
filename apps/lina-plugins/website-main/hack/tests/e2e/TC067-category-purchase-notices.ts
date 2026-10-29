import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {CatalogPage} from '../pages/CatalogPage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1280,height:800}});
test.describe('TC-67 分类独立购买说明和温馨提示',()=>{
 test('TC-67a: 分类隔离、后台读取更新、清空、手机布局和安全纯文本',async({page})=>{
  const site=new CatalogPage(page);await controlledBusiness(page);
  const categories=[{categoryNo:'cat-a',nameZhCN:'甲类云服务',purchaseNotice:'甲类购买说明\n第二行说明',usageNotice:'甲类温馨提示'},{categoryNo:'cat-b',nameZhCN:'乙类云服务',purchaseNotice:'乙类购买说明',usageNotice:'<img src=x onerror=alert(1)>仅为文本'}];
  await page.route('**'+prefix+'/catalog/products**',route=>{
   const url=new URL(route.request().url());
   const product={publicSlug:'merchant-product',categoryNo:url.searchParams.get('categoryNo')||'cat-a',nameZhCN:'本方商品',summaryZhCN:'CPU：4核\n内存：8G\n带宽：100M',availability:'available',inventory:{mode:'finite',sellableQuantity:'4'},minimumConfigurationMonthlyPrice:{currencyCode:'CNY',amountMinor:'6800'}};
   return route.fulfill({json:{code:0,data:url.pathname.endsWith('/products')?{categories,products:[product],nextCursor:''}:{product}}});
  });
  await site.open('/products.html?categoryNo=cat-a');
  await expect(site.purchaseNotice()).toHaveText('甲类购买说明\n第二行说明');await expect(site.usageNotice()).toHaveText('甲类温馨提示');
  await expect(site.product('merchant-product').locator('.amount')).toHaveText('68.00');
  console.log(await site.capture('category-notices-desktop',false));
  await site.selectCategory('cat-b');await expect(site.purchaseNotice()).toHaveText('乙类购买说明');await expect(site.usageNotice()).toHaveText('<img src=x onerror=alert(1)>仅为文本');await expect(site.usageNotice().locator('img')).toHaveCount(0);
  categories[1].purchaseNotice='乙类已更新';categories[1].usageNotice='';await site.reloadCatalog();await expect(site.purchaseNotice()).toHaveText('乙类已更新');await expect(site.usageNotice()).toBeHidden();
  await site.setViewport(390,844);await expect.poll(async()=>(await site.layout()).scrollWidth).toBeLessThanOrEqual(390);console.log(await site.capture('category-notices-mobile',false));
  categories[1].purchaseNotice='';await site.reloadCatalog();await expect(site.purchaseNotice()).toBeHidden();
  await site.selectCategory('');await expect(site.purchaseNotice()).toBeHidden();await expect(site.usageNotice()).toBeHidden();
  await site.open('/products.html?product=merchant-product');await expect(site.purchaseNotice()).toHaveText('甲类购买说明\n第二行说明');
 });
 test('TC-67b: 分类读取失败不显示旧提示或竞品经营资料',async({page})=>{
  const site=new CatalogPage(page);await controlledBusiness(page);
  await page.route('**'+prefix+'/catalog/products**',r=>r.fulfill({status:503,json:{code:50}}));
  await site.open('/products.html?categoryNo=cat-a');await expect(page.locator('[data-status]')).toHaveText('商品加载失败，请重试。');await expect(site.purchaseNotice()).toBeHidden();await expect(site.usageNotice()).toBeHidden();
  console.log(await site.capture('category-notices-failure',false));
 });
});
