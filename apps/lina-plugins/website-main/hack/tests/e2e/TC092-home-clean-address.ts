import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';

test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:900}});
test.describe('TC-92 首页地址不带商品分类',()=>{
 test.describe.configure({timeout:60000});
 test.beforeEach(async({page})=>{
  if(process.env.WEBSITE_HOME_URL_VERIFY_LIVE!=='1'){
   const body=await readFile(resolve(import.meta.dirname,'../../../frontend/public/lina/site.js'),'utf8');
   await page.route('**/lina/site.js',route=>route.fulfill({contentType:'application/javascript',body}));
  }
 });
 test('TC-92a: 首页默认分类和切换分类不改地址，刷新继续正常加载',async({page},info)=>{
  await page.goto('/');
  const tabs=page.locator('.systemproduct-category');
  await expect(tabs.first()).toHaveAttribute('aria-current','true',{timeout:20000});
  await expect(page.locator('[data-products] > a').first()).toBeVisible();
  await expect(page).toHaveURL('http://127.0.0.1:6680/');
  expect(await tabs.count()).toBeGreaterThan(1);
  const next=tabs.nth(1),categoryNo=new URL((await next.getAttribute('href'))!,'http://127.0.0.1:6680').searchParams.get('categoryNo');
  const changed=page.waitForResponse(r=>r.url().includes('/catalog/products?')&&new URL(r.url()).searchParams.get('categoryNo')===categoryNo);
  await next.click();expect((await changed).ok()).toBe(true);
  await expect(tabs.nth(1)).toHaveAttribute('aria-current','true',{timeout:20000});
  await expect(page).toHaveURL('http://127.0.0.1:6680/');
  await page.screenshot({path:info.outputPath('home-category-clean-address.png'),animations:'disabled'});
  await page.reload();await expect(tabs.first()).toHaveAttribute('aria-current','true',{timeout:20000});
  await expect(page).toHaveURL('http://127.0.0.1:6680/');
 });
 test('TC-92b: 商品目录返回首页为根地址，目录自身保留分类',async({page},info)=>{
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products?limit=1');
  expect(response.ok()).toBe(true);
  const result=await response.json(),category=result.data.products[0].categoryNo;
  await page.goto('/products.html?categoryNo='+category);
  await expect(page.locator('[data-products] .product-card').first()).toBeVisible({timeout:20000});
  expect(new URL(page.url()).searchParams.get('categoryNo')).toBe(category);
  await page.locator('[data-products] .product-card .buy-now-button[href]').first().click();
  await expect(page.locator('body')).toHaveAttribute('data-page','configure');
  await page.locator('.horizontal-menu-navbar-logo a').click();
  await expect(page.locator('.systemproduct-category').first()).toHaveAttribute('aria-current','true',{timeout:20000});
  await expect(page).toHaveURL('http://127.0.0.1:6680/');
  await page.screenshot({path:info.outputPath('returned-home-clean-address.png'),animations:'disabled'});
 });
 test('TC-92c: 旧首页分类链接清理分类参数，保留其他参数及锚点',async({page})=>{
  await page.goto('/index.html?categoryNo=unknown-category&utm_source=home-check#products');
  await expect(page.locator('.systemproduct-category').first()).toHaveAttribute('aria-current','true',{timeout:20000});
  await expect(page).toHaveURL('http://127.0.0.1:6680/?utm_source=home-check#products');
 });
});
