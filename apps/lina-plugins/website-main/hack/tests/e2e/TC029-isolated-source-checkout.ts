import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {WebsitePage} from '../pages/SourceWebsitePage';
import {CustomerCheckoutPage} from '../pages/CustomerCheckoutPage';
const backend='http://127.0.0.1:16630',center='http://localhost:16632';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},customerOrigin:center,apiOrigin:backend,locale:'zh-CN'});
test.describe('TC-29 当前源码与既有隔离后台交易（不付款）',()=>{
 test('TC-29a: 复用隔离客户及商品，官网真实加车登录归并核价到待付款订单',async({page,playwright},info)=>{
  test.skip(process.env.WEBSITE_LOCAL_CHECKOUT!=='1','仅明确授权隔离环境执行');test.setTimeout(60000);
  const account=JSON.parse(await readFile(process.env.WEBSITE_QA_ACCOUNT_FILE!,'utf8'));
  // Preflight checks the exact existing QA account without clearing its cart.
  const preflight=await playwright.request.newContext();
  try{
   const auth=await(await preflight.post(backend+'/api/v1/auth/login',{data:{username:account.email,password:account.password,clientType:'web'}})).json();expect(auth.code).toBe(0);
   const cart=await(await preflight.get(backend+'/x/idc-core/api/v1/account/commerce/cart',{headers:{Authorization:'Bearer '+auth.data.accessToken}})).json();expect(cart.code).toBe(0);
   expect(cart.data.cart?.items?.length||0,'既有客户车非空时停止，不删除或购买其旧商品').toBe(0);
  }finally{await preflight.dispose();}
  const allowed:string[]=[],denied:string[]=[];
  await page.route('http://localhost:6680/x/idc-core/api/v1/public/commerce/**',async route=>{
   const r=route.request(),url=new URL(r.url());
   if(r.method()!=='POST'||!['/x/idc-core/api/v1/public/commerce/carts','/x/idc-core/api/v1/public/commerce/cart-items'].includes(url.pathname)){denied.push(r.method()+' '+url.pathname);return route.abort();}
   allowed.push(r.method()+' '+url.pathname);return route.fulfill({response:await route.fetch({url:backend+url.pathname})});
  });
  await page.route(center+'/**',route=>{
   const r=route.request(),path=new URL(r.url()).pathname;
   if(['GET','HEAD'].includes(r.method()))return route.continue();
   if(r.method()==='POST'&&['/api/v1/auth/login','/x/idc-core/api/v1/account/commerce/guest-cart-merge','/x/idc-core/api/v1/account/commerce/quotes','/x/idc-core/api/v1/account/commerce/orders'].includes(path)){allowed.push(r.method()+' '+path);return route.continue();}
   denied.push(r.method()+' '+path);return route.abort('blockedbyclient');
  });
  const site=new WebsitePage(page),customer=new CustomerCheckoutPage(page);
  await site.open('/configure.html?product=isolated-integration-only');await expect(page.locator('[data-quote] button')).toHaveText('1月：CNY 1.00');
  await site.setPurchaseCount('quantity','2');await site.setPurchaseCount('cycles','3');await site.chooseTerm();await site.submit();
  await expect(page.getByPlaceholder('请输入用户名')).toBeVisible();
  const merged=page.waitForResponse(r=>r.url().endsWith('/account/commerce/guest-cart-merge')&&r.request().method()==='POST');
  try{await customer.login(account.email,account.password);}finally{await customer.clearLoginPassword();}
  expect((await(await merged).json()).code).toBe(0);
  await expect(page.getByText('2 件商品 · 3 个计费周期',{exact:true})).toBeVisible();await customer.reprice();
  await expect(page.getByText('CNY 6.00',{exact:true}).first()).toBeVisible();await expect(page.getByRole('button',{name:'确认并提交订单',exact:true})).toBeEnabled();
  await site.capture('isolated-current-source-cart');
  const ordered=page.waitForResponse(r=>r.url().endsWith('/account/commerce/orders')&&r.request().method()==='POST');await customer.checkout();
  const result=await(await ordered).json();expect(result.code).toBe(0);const checkout=result.data.checkout;expect(checkout).toMatchObject({currencyCode:'CNY',totalAmountMinor:'600'});
  await expect(page.getByRole('heading',{name:'订单已创建',exact:true})).toBeVisible();await customer.openOrder(checkout.orderNo);
  await expect(page.getByText('待付款',{exact:true})).toBeVisible();await expect(page.getByText('2 件商品 · 3 个计费周期',{exact:true})).toBeVisible();await expect(page.getByText(checkout.invoiceNo,{exact:true})).toBeVisible();
  const screenshot=await site.capture('isolated-current-source-unpaid-order');
  const paymentPreparation='POST /x/idc-core/api/v1/account/commerce/orders/'+checkout.orderNo+'/payment-preparations';
  // The existing order view automatically prepares payment; this test must
  // reject that request rather than calling a payment provider.
  expect(denied.filter(path=>path!==paymentPreparation)).toEqual([]);
  expect(denied).toContain(paymentPreparation);
  await info.attach('isolated-checkout-evidence',{body:JSON.stringify({backend,center,source:'current local website-main source',checkout,allowed,denied,screenshot,retained:'unpaid QA order and invoice; no payment or provisioning'},null,2),contentType:'application/json'});
 });
});
