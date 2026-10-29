import {test,expect} from '@playwright/test';
import {readFile} from 'node:fs/promises';
import {randomBytes,randomUUID} from 'node:crypto';
import {WebsitePage} from '../pages/WebsitePage';
import {CustomerCheckoutPage} from '../pages/CustomerCheckoutPage';

// Explicit local opt-in. Uses a previously provisioned QA account and published
// test product. No fixtures, route interception, payment or external provisioning.
test.describe('TC-12 真实登录、归并与未付款订单',()=>{
 test.use({locale:'zh-CN'});
 test('TC-12a: 官网数量周期经登录归并进入同一客户订单，金额与账单一致，另一客户不可读取',async({page,playwright})=>{
  test.skip(process.env.WEBSITE_LOCAL_CHECKOUT!=='1','只在明确授权的本地隔离环境运行');
  const base=process.env.WEBSITE_TEST_URL!;
  expect(['localhost','127.0.0.1']).toContain(new URL(base).hostname);
  const account=JSON.parse(await readFile(process.env.WEBSITE_QA_ACCOUNT_FILE!,'utf8')) as {email:string;password:string};
  const slug=process.env.WEBSITE_REAL_PRODUCT!;
  expect(slug).toBeTruthy();
  const site=new WebsitePage(page);const customer=new CustomerCheckoutPage(page);
  await site.open('/configure.html?product='+encodeURIComponent(slug));
  await expect(page.locator('[data-quote] button').first()).toHaveText('1月：CNY 1.00');
  await site.setPurchaseCount('quantity','2');await site.setPurchaseCount('cycles','3');await site.chooseTerm();
  await site.submit();await expect(page.getByPlaceholder('请输入用户名')).toBeVisible();
  const merged=page.waitForResponse(r=>r.url().endsWith('/account/commerce/guest-cart-merge')&&r.request().method()==='POST');
  await customer.login(account.email,account.password);
  expect((await (await merged).json()).code).toBe(0);
  await expect(page.getByText('2 件商品 · 3 个计费周期',{exact:true})).toBeVisible();
  await expect(page.getByRole('heading',{name:'隔离联调商品（测试专用）',exact:true})).toBeVisible();
  await customer.reprice();
  await expect(page.getByRole('button',{name:'确认并提交订单',exact:true})).toBeEnabled();
  await expect(page.getByText('CNY 6.00',{exact:true}).first()).toBeVisible();
  await site.capture('real-login-merged-quote',false);
  const created=page.waitForResponse(r=>r.url().endsWith('/account/commerce/orders')&&r.request().method()==='POST');
  await customer.checkout();
  const result=await (await created).json();expect(result.code).toBe(0);
  const checkout=result.data.checkout;
  expect(checkout).toMatchObject({currencyCode:'CNY',totalAmountMinor:'600'});
  expect(checkout.orderNo).toMatch(/^ORD_/);
  await expect(page.getByRole('heading',{name:'订单已创建',exact:true})).toBeVisible();
  await customer.openOrder(checkout.orderNo);
  await expect(page.getByText('待付款',{exact:true})).toBeVisible();
  await expect(page.getByText('2 件商品 · 3 个计费周期',{exact:true})).toBeVisible();
  await expect(page.getByRole('heading',{name:'隔离联调商品（测试专用）',exact:true})).toBeVisible();
  await expect(page.getByText('CNY 6.00',{exact:true}).first()).toBeVisible();
  await expect(page.getByText(checkout.invoiceNo,{exact:true})).toBeVisible();
  await expect(page.getByText('等待实名认证',{exact:true})).toBeVisible();
  await site.capture('real-owned-order',false);
  const backend=process.env.WEBSITE_API_ORIGIN!;
  expect(['16630','16634']).toContain(new URL(backend).port);
  const admin=await (await page.request.post(backend+'/api/v1/auth/login',{data:{username:process.env.WEBSITE_QA_ADMIN_USER,password:process.env.WEBSITE_QA_ADMIN_PASSWORD,clientType:'web'}})).json();expect(admin.code).toBe(0);
  const email='website-boundary-'+randomBytes(8).toString('hex')+'@project1.test';const password=randomBytes(24).toString('base64url')+'aA1!';
  const createdCustomer=await (await page.request.post(backend+'/x/idc-core/api/v1/admin/customers/direct-accounts',{headers:{Authorization:'Bearer '+admin.data.accessToken},data:{customerType:'individual',displayName:'隔离订单归属验收账号',countryCode:'CN',preferredLocale:'zh-CN',email,initialPassword:password,reason:'仅验证独立本地数据库中订单不可跨客户访问，不通知、不付款。',idempotencyKey:randomUUID()}})).json();expect(createdCustomer.code).toBe(0);
  const other=await playwright.request.newContext();
  try{
   const loggedIn=await (await other.post(backend+'/api/v1/auth/login',{data:{username:email,password,clientType:'web'}})).json();expect(loggedIn.code).toBe(0);
   const headers={Authorization:'Bearer '+loggedIn.data.accessToken};const orders=backend+'/x/idc-core/api/v1/account/commerce/orders';
   const denied=await (await other.get(orders+'/'+checkout.orderNo,{headers})).json();
   const missing=await (await other.get(orders+'/ORD_'+randomBytes(16).toString('hex'),{headers})).json();
   expect(denied.code).not.toBe(0);expect(denied.code).toBe(missing.code);expect(denied.errorCode).toBe(missing.errorCode);expect(denied.data).toBeNull();
   const ownOrders=await (await other.get(orders,{headers})).json();expect(ownOrders.code).toBe(0);expect(JSON.stringify(ownOrders.data)).not.toContain(checkout.orderNo);
  }finally{await other.dispose();}
  // Retain the unpaid order as an audit artifact in the isolated database.
  // No payment, compliance bypass, delivery or unrelated cart cleanup occurs.
 });
});
