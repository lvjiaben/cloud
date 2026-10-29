import {test,expect} from '../support/source-site';
import {WebsitePage} from '../pages/SourceWebsitePage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-35 配置页真实分类导航',()=>{
 for(const failure of [false,true])test('TC-35'+(failure?'b':'a')+': '+(failure?'分类加载失败不阻断真实报价，重试后恢复':'直接配置页保留本方分类及选中状态'),async({page})=>{
  const site=new WebsitePage(page);await site.open('/products.html');await expect(page.locator('[data-status]')).not.toHaveText(/正在加载/);
  const path=await site.availableConfigurationPath();expect(path).toBeTruthy();
  const category=await page.locator('[data-categories] a').nth(1).textContent();expect(category).toBeTruthy();
  let unavailable=failure;const categoryRequests:string[]=[];
  await page.route('**/x/idc-core/api/v1/public/catalog/products?limit=1',async route=>{categoryRequests.push(route.request().url());if(unavailable)return route.fulfill({status:503,json:{code:50,data:null}});return route.fallback();});
  await site.open(path!);await expect(page.locator('[data-quote] button').first()).toContainText('CNY');
  if(failure){await expect(page.locator('[data-categories]')).toContainText('商品分类加载失败');await site.capture('configuration-category-failed-quote-ready');unavailable=false;await site.retryCategories();}
  await expect(page.locator('[data-categories] [aria-current=true]')).toHaveText(category!);
  await site.capture('configuration-category-'+(failure?'recovered':'loaded'));
  expect(categoryRequests).toHaveLength(failure?2:1);
  await site.category(category!);await expect(page).toHaveURL(/products.html\?categoryNo=/);
  await expect(page.locator('[data-products] > *')).not.toHaveCount(0);
  await expect(page.locator('[data-brand-name]').first()).not.toHaveText('');await page.waitForLoadState('networkidle');
 });
});
