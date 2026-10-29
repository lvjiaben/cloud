import {test,expect} from '../support/source-site';
import {controlledBusiness} from '../support/controlled-business';
import {WebsitePage} from '../pages/SourceWebsitePage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-32 原商品页筛选条件与选中状态',()=>{
 test('TC-32a: 直接打开、分类切换及顶部搜索保持真实查询组合',async({page})=>{
  const fixture=await controlledBusiness(page),site=new WebsitePage(page);
  await site.open('/products.html?categoryNo=cat-a&namePrefix='+encodeURIComponent('测试专用甲'));
  await expect(page.locator('[data-categories] [aria-current=true]')).toHaveText('测试分类甲');
  await expect(page.getByRole('textbox',{name:'搜索商品',exact:true})).toHaveValue('测试专用甲');
  await expect(page.locator('[data-products] > *')).toHaveCount(1);
  await site.category('测试分类乙');
  await expect(page).toHaveURL(/categoryNo=cat-b/);expect(new URL(page.url()).searchParams.get('namePrefix')).toBe('测试专用甲');
  await expect(page.locator('[data-status]')).toHaveText('暂无可订购商品。');
  await expect(page.locator('[data-categories] [aria-current=true]')).toHaveText('测试分类乙');
  await site.searchNavigation('测试专用缺货');
  expect(new URL(page.url()).searchParams.get('categoryNo')).toBe('cat-b');
  await expect(page.locator('[data-products] > *')).toHaveCount(1);await expect(page.locator('[data-products]')).toContainText('测试专用缺货');
  await expect(page.locator('#search-input')).toHaveValue('测试专用缺货');
  const selected=page.locator('[data-categories] [aria-current=true]');
  await expect(selected).toHaveCSS('background-color','rgb(224, 238, 255)');
  await site.capture('catalog-combined-filter-active');
  await site.searchNavigation('');await expect(page.locator('#search-input')).toHaveValue('');expect(new URL(page.url()).searchParams.has('namePrefix')).toBe(false);
  expect(new URL(page.url()).searchParams.get('categoryNo')).toBe('cat-b');
  await site.searchProducts('测试专用缺货');await expect(page.getByRole('textbox',{name:'搜索商品',exact:true})).toHaveValue('测试专用缺货');
  await site.searchProducts('');
  await site.category('全部商品');await expect(page.locator('[data-categories] [aria-current=true]')).toHaveText('全部商品');
  await expect(page.locator('[data-products] > *')).toHaveCount(2);
  expect(fixture.requests.every(r=>r.method==='GET')).toBe(true);
 });
 test('TC-32b: 首页带分类进入商品搜索仍使用该分类',async({page})=>{
  const fixture=await controlledBusiness(page),site=new WebsitePage(page);
  await site.open('/index.html?categoryNo=cat-b');await expect(page.locator('[data-categories] [aria-current=true]')).toHaveText('测试分类乙');
  await site.searchNavigation('测试专用缺货');expect(new URL(page.url()).searchParams.get('categoryNo')).toBe('cat-b');
  await expect(page.locator('[data-products] > *')).toHaveCount(1);await expect(page.locator('[data-products]')).toContainText('测试专用缺货');
  await site.capture('catalog-home-search-filter');expect(fixture.requests.every(r=>r.method==='GET')).toBe(true);
 });
});
