import {test,expect} from '../support/source-site';
import {HomeDetailPage} from '../pages/HomeDetailPage';
import {previewProducts,previewResponse} from '../support/home-preview-data.mjs';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000},reducedMotion:'reduce'});
test.describe('TC-64 分类左栏联动与原卡片边框',()=>{
 test('TC-64a: 少量商品保持原左栏尺寸，分类名称与商品同步',async({page},info)=>{
  const categories=[{categoryNo:'our-cloud',nameZhCN:'本方弹性云'},{categoryNo:'our-dedicated',nameZhCN:'本方独立服务器'}];
  const products=previewProducts.slice(0,2).map((p,i)=>({...p,categoryNo:categories[i].categoryNo}));
  await page.route('**/x/**',async route=>{const q=route.request(),u=new URL(q.url()),r=previewResponse(u,q.method());if(u.pathname.endsWith('/catalog/products'))r.data={categories,products:products.filter(p=>!u.searchParams.get('categoryNo')||p.categoryNo===u.searchParams.get('categoryNo')),nextCursor:''};await route.fulfill({status:r.status,json:{code:0,data:r.data}});});
  const site=new HomeDetailPage(page);await site.open('/');await expect(page.locator('[data-products] .systemproduct-item')).toHaveCount(2);
  await site.focusSection('.class-systemproducts');
  const top=page.locator('.systemproduct-left-top'),bottom=page.locator('.systemproduct-right-bottom');
  const a=await top.boundingBox(),b=await bottom.boundingBox();expect(a!.width).toBe(294);expect(a!.height).toBe(282);expect(b!.height).toBe(173);expect(b!.y-a!.y-a!.height).toBe(10);
  await expect(top).toHaveCSS('border-top-width','1px');await expect(top).toHaveCSS('border-top-color','rgb(255, 255, 255)');await expect(top).toHaveCSS('border-radius','4px');
  await page.locator('[data-categories]').getByRole('link',{name:'本方弹性云',exact:true}).click();await expect(page.locator('.systemproduct-left-title')).toHaveText('本方弹性云');await expect(page.locator('[data-products] .systemproduct-name')).toHaveText(products[0].nameZhCN);
  await page.locator('[data-categories]').getByRole('link',{name:'本方独立服务器',exact:true}).click();await expect(page.locator('.systemproduct-left-title')).toHaveText('本方独立服务器');await expect(page.locator('[data-products] .systemproduct-name')).toHaveText(products[1].nameZhCN);
  await expect(page.locator('.systemproduct-left-desc')).toBeHidden();await expect(page.locator('.systemproduct-left-desc')).toHaveText('');await expect(page.locator('.class-systemproducts')).not.toContainText('各项配置均衡');
  await expect(page.locator('.systemproduct-right-bottom a')).toHaveAttribute('href','/contact.html');
  await site.captureSection('.class-systemproducts',info.outputPath('category-selected.png'));
  await page.locator('[data-categories]').getByRole('link',{name:'全部商品',exact:true}).click();await expect(page.locator('.systemproduct-left-title')).toHaveText('全部商品');await expect(page.locator('[data-products] .systemproduct-item')).toHaveCount(2);
 });
});
