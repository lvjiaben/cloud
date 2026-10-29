import {test,expect} from '../support/source-site';
import {readFile} from 'node:fs/promises';
import {WebsitePage} from '../pages/SourceWebsitePage';
import {CustomerCheckoutPage} from '../pages/CustomerCheckoutPage';

const customerOrigin=process.env.WEBSITE_SOURCE_CUSTOMER_ORIGIN||'http://localhost:6032';
const slug=process.env.WEBSITE_REAL_PRODUCT||'local-integration-only';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900},customerOrigin,apiOrigin:process.env.WEBSITE_SOURCE_API_ORIGIN||'http://127.0.0.1:6030',locale:'zh-CN'});

test.describe('TC-25 当前商品登录后可重新配置',()=>{
 test('TC-25a: 真实商品入口只传当前商品和币种，商品读取失败后隐藏入口',async({page})=>{
  const site=new WebsitePage(page);
  await site.open('/configure.html?product='+encodeURIComponent(slug));
  const link=page.locator('[data-customer-configure]');await expect(link).toBeVisible();
  await expect(link).toHaveAttribute('href',customerOrigin+'/account/products/'+encodeURIComponent(slug)+'/configure?currency=CNY');
  await site.capture('minimum-customer-configuration-entry');
  await page.evaluate(()=>{(window as any).__LINA_TEMPLATE__.customerUrl='https://other.invalid';});
  await site.refreshConfiguration();await expect(link).toBeHidden();expect(await link.getAttribute('href')).toBeNull();
  await page.evaluate(origin=>{(window as any).__LINA_TEMPLATE__.customerUrl=origin;},customerOrigin);
  await site.refreshConfiguration();await expect(link).toBeVisible();
  await page.route('**/x/idc-core/api/v1/public/catalog/products/'+encodeURIComponent(slug),r=>r.fulfill({status:503,body:'{}'}));
  await site.refreshConfiguration();await expect(page.locator('[data-status]')).toHaveText('无法读取商品配置，请重试。');
  await expect(link).toBeHidden();expect(await link.getAttribute('href')).toBeNull();
 });
 test('TC-25b: 真实客户在新窗口登录后进入同一商品并由现有后台校验报价',async({page,context},info)=>{
  test.skip(!process.env.WEBSITE_QA_ACCOUNT_FILE&&!(process.env.WEBSITE_QA_LOGIN_USER&&process.env.WEBSITE_QA_LOGIN_PASSWORD),'需要既有本地验收账号；不创建账号，不替换认证或业务响应');
  test.setTimeout(60000);
  const account=process.env.WEBSITE_QA_ACCOUNT_FILE?JSON.parse(await readFile(process.env.WEBSITE_QA_ACCOUNT_FILE,'utf8')) as {email:string;password:string}:{email:process.env.WEBSITE_QA_LOGIN_USER!,password:process.env.WEBSITE_QA_LOGIN_PASSWORD!};
  const blocked:string[]=[];
  await context.route(customerOrigin+'/**',route=>{
   const r=route.request(),path=new URL(r.url()).pathname;
   if(['GET','HEAD'].includes(r.method())||r.method()==='POST'&&(path==='/api/v1/auth/login'||/\/purchase-configuration\/(evaluate|quote)$/.test(path)))return route.continue();
   blocked.push(r.method()+' '+path);return route.abort('blockedbyclient');
  });
  const site=new WebsitePage(page);
  await site.open('/configure.html?product='+encodeURIComponent(slug));
  await expect(page.locator('[data-customer-configure]')).toBeVisible();
  const [popup]=await Promise.all([page.waitForEvent('popup'),site.openCustomerConfiguration()]);
  const customer=new CustomerCheckoutPage(popup),popupSite=new WebsitePage(popup);
  await expect(page).toHaveURL(/configure.html\?product=/);
  expect(await popup.evaluate(()=>window.opener)).toBeNull();
  await expect(popup.getByPlaceholder('请输入用户名')).toBeVisible();
  const loginURL=new URL(popup.url());expect(loginURL.pathname).toBe('/auth/login');
  expect(decodeURIComponent(loginURL.searchParams.get('redirect')||'')).toBe('/account/products/'+encodeURIComponent(slug)+'/configure?currency=CNY');
  const login=popup.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1/auth/login'&&r.request().method()==='POST');
  let auth:any;
  try{await customer.login(account.email,account.password);auth=await(await login).json();}finally{await customer.clearLoginPassword();}
  if(auth.code!==0){await customer.clearLoginPassword();const screenshot=await popupSite.capture('minimum-real-login-rejected',false);await info.attach('login-result',{body:JSON.stringify({code:auth.code,message:auth.message,errorCode:auth.errorCode,screenshot}),contentType:'application/json'});}
  expect(auth.code).toBe(0);
  await expect(popup).toHaveURL(customerOrigin+'/account/products/'+encodeURIComponent(slug)+'/configure?currency=CNY');
  await expect(popup.getByRole('button',{name:'校验并查看价格',exact:true})).toBeEnabled();
  const quote=popup.waitForResponse(r=>new URL(r.url()).pathname.endsWith('/products/'+encodeURIComponent(slug)+'/purchase-configuration/quote'));
  await customer.quoteConfiguration();const response=await quote,result=await response.json();expect(result.code).toBe(0);expect(result.data.currencyCode).toBe('CNY');
  expect(result.data.terms.some((t:{status:string;totalAmount:string})=>t.status==='final'&&t.totalAmount==='100')).toBeTruthy();
  await expect(popup.getByTestId('purchase-configuration-quote')).toContainText('1.00');
  const screenshot=await popupSite.capture('minimum-real-customer-quote',false);await info.attach('screenshot',{path:screenshot,contentType:'image/png'});
  expect(blocked,'不允许购物车、订单或其他业务写入').toEqual([]);
 });
});


