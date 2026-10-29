import {test,expect} from '@playwright/test';
import {WebsitePage} from '../pages/WebsitePage';
// Requires an explicitly selected, already published real local product; no fixtures or API replacement.
test.describe('TC-2 真实商品配置与加车',()=>{
 test('TC-2a: 服务端配置与报价一致，真实加车后交接现有中心',async({page})=>{
  test.skip(!process.env.WEBSITE_REAL_PRODUCT,'未提供本地真实商品标识，不能冒充业务验收');
  const site=new WebsitePage(page);const slug=process.env.WEBSITE_REAL_PRODUCT!;
  await site.open('/configure.html?product='+encodeURIComponent(slug));
  await expect(page.locator('[data-quote] button').first()).toBeVisible();await site.chooseTerm();await expect(page.locator('[data-add]')).toBeEnabled();await site.capture('real-quote');
  // Read and forward the actual backend response unchanged before navigation.
  // No fixture, replacement data or forced successful status is supplied.
  let body:{code:number;data:{cart:{reference:string}}}|undefined;
  await page.route('**/public/commerce/cart-items',async route=>{const response=await route.fetch();body=await response.json();await route.fulfill({response});});
  await site.submit();await expect.poll(()=>body?.code).toBe(0);expect(body?.data.cart.reference).toBeTruthy();
  await expect(page).toHaveURL(/\/account\/cart/);await site.capture('real-customer-cart-handoff');
  // Login merge, item ownership and order totals require authenticated live assertions; this test does not claim them.
 });
});
