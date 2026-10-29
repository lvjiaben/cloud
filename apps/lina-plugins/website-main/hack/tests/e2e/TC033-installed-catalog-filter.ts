import {test,expect} from '@playwright/test';
import {WebsitePage as SourcePage} from '../pages/SourceWebsitePage';

test.use({viewport:{width:1440,height:900}});
test.describe('TC-33 已安装官网真实筛选保持',()=>{
 test('TC-33a: 现有商品分类和顶部搜索组合来自真实后台，清空和切换不丢条件',async({page,request})=>{
  const response=await request.get('/x/idc-core/api/v1/public/catalog/products?limit=12');expect(response.ok()).toBe(true);
  const result=await response.json();expect(result.code).toBe(0);const product=result.data.products.find((p:{categoryNo:string})=>p.categoryNo);expect(product,'仅使用后台已有商品，不造数据').toBeTruthy();
  const category=result.data.categories.find((c:{categoryNo:string})=>c.categoryNo===product.categoryNo);expect(category).toBeTruthy();
  const site=new SourcePage(page),writes:string[]=[];page.on('request',r=>{if(!['GET','HEAD'].includes(r.method()))writes.push(r.method()+' '+r.url());});
  await site.open('/products.html?'+new URLSearchParams({categoryNo:product.categoryNo,namePrefix:product.nameZhCN||product.nameEnUS}));
  await expect(page.locator('base')).toHaveAttribute('href','/x-assets/website-main/'+process.env.WEBSITE_EXPECTED_VERSION+'/');
  await expect(page.locator('[data-categories] [aria-current=true]')).toHaveText(category.nameZhCN||category.nameEnUS);
  await expect(page.locator('[data-products] h5')).toContainText([product.nameZhCN||product.nameEnUS]);
  await site.searchNavigation('');expect(new URL(page.url()).searchParams.get('categoryNo')).toBe(product.categoryNo);expect(new URL(page.url()).searchParams.has('namePrefix')).toBe(false);
  await expect(page.locator('#search-input')).toHaveValue('');
  await site.searchProducts(product.nameZhCN||product.nameEnUS);await expect(page.getByRole('textbox',{name:'搜索商品',exact:true})).toHaveValue(product.nameZhCN||product.nameEnUS);
  await site.category('全部商品');await expect(page.locator('[data-categories] [aria-current=true]')).toHaveText('全部商品');
  expect(new URL(page.url()).searchParams.get('namePrefix')).toBe(product.nameZhCN||product.nameEnUS);
  await expect(page.locator('[data-products] h5')).toContainText([product.nameZhCN||product.nameEnUS]);
  await site.capture('installed-v014-catalog-filter');expect(writes).toEqual([]);
 });
});
