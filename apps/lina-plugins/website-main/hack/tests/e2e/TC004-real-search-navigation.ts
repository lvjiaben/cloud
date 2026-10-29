import {test,expect} from '@playwright/test';
import {WebsitePage} from '../pages/WebsitePage';

// Public template: standalone runner, actual API, no authentication or response replacement.
test.describe('TC-4 真实搜索和移动导航',()=>{
 test('TC-4a: 商品搜索同步地址、刷新回填、清除并重新读取后台',async({page})=>{
  const site=new WebsitePage(page);await site.open('/products.html');
  const response=page.waitForResponse(r=>r.url().includes('/public/catalog/products?')&&r.url().includes('namePrefix='));
  await site.searchProducts('不存在的商品');
  const result=await response;expect(result.ok()).toBeTruthy();expect((await result.json()).code).toBe(0);
  await expect(page).toHaveURL(/namePrefix=/);
  await site.open(page.url());await expect(page.locator('#search-input')).toHaveValue('不存在的商品');
  await site.searchProducts('');await expect(page).toHaveURL(/\/products\.html$/);
  await expect(page.locator('[data-status]')).not.toHaveText('正在加载商品…');await site.capture('search-cleared');
 });
 test('TC-4b: 各页面页头搜索使用商品接口，不把内容参数带入商品页',async({page})=>{
  const site=new WebsitePage(page);
  for(const path of ['index','news','announcements','help','downloads','content','contact','about','configure','products']){
   await site.open('/'+path+'.html');
   const response=page.waitForResponse(r=>r.url().includes('/public/catalog/products?')&&r.url().includes('namePrefix='));
   await site.searchNavigation('不存在的商品');
   const result=await response;expect(result.ok()).toBeTruthy();expect((await result.json()).code).toBe(0);
   await expect(page).toHaveURL(/\/products\.html\?namePrefix=/);
   await expect(page.locator('#search-input')).toHaveValue('不存在的商品');
  }
 });
 test('TC-4c: 手机导航展开关闭，并使用后台配置的用户中心地址',async({page,request})=>{
  const site=new WebsitePage(page);await site.setViewport(390,844);await site.open('/products.html');
  await expect(page.locator('[data-status]')).not.toHaveText('正在加载商品…');
  await site.toggleMenu();await expect(page.locator('.lina-menu-toggle')).toHaveAttribute('aria-expanded','true');
  const link=page.locator('.horizontal-menu-navbar-search').getByRole('link',{name:'控制台',exact:true});
  await expect(link).toBeVisible();const href=await link.getAttribute('href');expect(href).toBeTruthy();
  const response=await request.get(href!);expect(response.ok()).toBeTruthy();
  await site.capture('mobile-menu-open');await site.toggleMenu();
  await expect(page.locator('.lina-menu-toggle')).toHaveAttribute('aria-expanded','false');await expect(link).not.toBeVisible();
 });
});
