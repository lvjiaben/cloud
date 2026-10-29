import {test,expect} from '@playwright/test';
import {WebsitePage} from '../pages/WebsitePage';

test.describe('TC-10 桌面既有用户中心认证入口',()=>{
 test('TC-10a: 官网购物车入口到真实登录页，保留目标购物车路由',async({page})=>{
  const site=new WebsitePage(page);
  // Real UI and routes only. No credentials, login, cart or order writes.
  await site.open('/configure.html');await site.openCustomerCart();
  await expect(page).toHaveURL(/\/auth\/login\?redirect=/);
  const url=new URL(page.url());expect(url.hostname).toBe('localhost');expect(url.port).toBe('6032');
  expect(decodeURIComponent(url.searchParams.get('redirect')||'')).toBe('/account/cart');
  await expect(page.locator('input[name=username]')).toBeVisible();await expect(page.locator('input[name=password]')).toHaveAttribute('type','password');
  await expect(page.getByRole('button',{name:'login',exact:true})).toBeVisible();
  await site.capture('desktop-real-customer-login-target');
  // Preserved target is verified; successful authentication/merge/ownership
  // remains unverified until a real local product and customer are available.
 });
});
