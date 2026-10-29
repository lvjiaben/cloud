import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {CatalogPage} from '../pages/CatalogPage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-66 本方分类导航与最低配置月价（隔离数据）',()=>{
 test('TC-66a: 分类一级标题、同类多商品、准确库存、月起价和配置目标',async({page})=>{
  const site=new CatalogPage(page);await controlledBusiness(page);
  const products=[{publicSlug:'merchant-one',categoryNo:'merchant-cloud',nameZhCN:'本方弹性云甲',availability:'available',summaryZhCN:'CPU：4核；内存：8G；硬盘：100G；月付69元；安装费0元',inventory:{mode:'finite',sellableQuantity:'4'},monthlyStartingPrice:{currencyCode:'CNY',amountMinor:'1',setupFeeMinor:'0'},defaultConfigurationMonthlyPrice:{currencyCode:'CNY',amountMinor:'9900'},minimumConfigurationMonthlyPrice:{currencyCode:'CNY',amountMinor:'6800'}},{publicSlug:'merchant-two',categoryNo:'merchant-cloud',nameZhCN:'本方弹性云乙',availability:'unavailable',inventory:{mode:'finite',sellableQuantity:'0'},monthlyStartingPrice:{currencyCode:'CNY',amountMinor:'1200',setupFeeMinor:'0'},defaultConfigurationMonthlyPrice:{currencyCode:'CNY',amountMinor:'9900'}},{publicSlug:'merchant-other',categoryNo:'merchant-other-category',nameZhCN:'本方其他分类商品',availability:'available'}];
  await page.route('**'+prefix+'/catalog/products**',route=>{
   const url=new URL(route.request().url()),slug=url.pathname.split('/').at(-1);
   return route.fulfill({json:{code:0,data:slug==='products'?{products:products.filter(p=>!url.searchParams.get('categoryNo')||p.categoryNo===url.searchParams.get('categoryNo')),categories:[{categoryNo:'merchant-cloud',nameZhCN:'本方弹性云'}],nextCursor:''}:{product:products.find(p=>p.publicSlug===slug)}}});
  });
  await site.open('/products.html');
  await expect(site.menu()).toHaveText(['全部商品','本方弹性云']);
  const first=site.product('merchant-one');await expect(first.locator('.stock-info')).toHaveText('库存4');await expect(first.locator('.amount')).toHaveText('68.00');await expect(first.locator('.period')).toHaveText('起/月');await expect(first.locator('.price')).toHaveAttribute('title','最低可购配置的月付价，含适用税费，不含一次性安装费');await expect(first.locator('.lina-catalog-setup')).toHaveCount(0);await expect(first.locator('.product-description li')).toHaveText(['CPU：4核','内存：8G','硬盘：100G']);await expect(first.locator('.product-description')).not.toContainText('月付69元');
  await expect(first.getByRole('link',{name:'立即购买'})).toHaveAttribute('href','/configure.html?product=merchant-one');
  await expect(page.locator('[data-product=merchant-two] .stock-info')).toHaveText('库存0');await expect(page.locator('[data-product=merchant-two] .price')).toHaveText('价格待配置');await expect(page.locator('[data-product=merchant-two] .buy-now-button')).not.toHaveAttribute('href');
  await site.captureCatalog('catalog-menu');
  await site.selectCategory('merchant-cloud');await expect(page).toHaveURL(/categoryNo=merchant-cloud/);await expect(page.locator('[data-products] .product-card')).toHaveCount(2);await expect(site.product('merchant-other')).toHaveCount(0);await expect(page.locator('[data-catalog-title]')).toHaveText('本方弹性云');await expect(site.category('merchant-cloud')).toHaveAttribute('aria-current','page');
  await site.captureCatalog('catalog-selected');
  await site.open('/products.html?product=merchant-one');await expect(site.menu()).toHaveText(['全部商品','本方弹性云']);await expect(site.product('merchant-one')).toBeVisible();await expect(page.locator('[data-products] .product-card')).toHaveCount(1);
 });
 test('TC-66b: 空目录、失败和恢复不复活旧商品',async({page})=>{
  const site=new CatalogPage(page);await controlledBusiness(page);let fail=true;
  await page.route('**'+prefix+'/catalog/products*',r=>fail?r.fulfill({status:503,json:{code:50}}):r.fulfill({json:{code:0,data:{products:[],categories:[],nextCursor:''}}}));
  await site.open('/products.html');await expect(page.locator('[data-status]')).toHaveText('商品加载失败，请重试。');await expect(page.locator('[data-products]')).toBeEmpty();
  await site.captureCatalog('catalog-failure');fail=false;await site.retryCatalog();await expect(page.locator('[data-status]')).toHaveText('暂无可订购商品。');await expect(page.locator('[data-products]')).toBeEmpty();
 });
});

