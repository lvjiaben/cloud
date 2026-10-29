import {test,expect} from '../support/source-site';
import {controlledBusiness} from '../support/controlled-business';
import {CommerceCartPage} from '../pages/CommerceCartPage';
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000}});
async function fixture(page:any,{signedIn=true,missingConfiguration=false,payment=true,redirectMethod='GET'}={}){
 await controlledBusiness(page);
 await page.route('**/lina/commerce-zh-CN.json',async(r:any)=>r.fulfill({contentType:'application/json',body:await readFile(resolve(import.meta.dirname,'../../../frontend/public/lina/commerce-zh-CN.json'),'utf8')}));
 await page.route('**/__website/session',(r:any)=>r.fulfill({json:{checked:true,accessToken:signedIn?'controlled-session':''}}));
 const revision='r'.repeat(43),requests:any[]=[],state={preparation:'ready',unknown:false,paid:false};
 const items:any[]=[{cartItemNo:'single',version:1,publicSlug:'server',productNameZhCN:'独立主机',productNameEnUS:'Private server',configurationReference:'private-ref',quantity:1,chargeTypeCode:'recurring',periodUnitCode:'month',periodCount:1,purchaseCycles:1,statusCode:'active',hasServerLogin:true,serverLoginHostname:'customer-host'},
 {cartItemNo:'auto',version:1,publicSlug:'server',productNameZhCN:'自动生成主机',productNameEnUS:'Automatic server',configurationReference:'automatic-ref',quantity:1,chargeTypeCode:'recurring',periodUnitCode:'month',periodCount:1,purchaseCycles:1,statusCode:'active'}];
 let quoteNo=0,failOrder=true,claimed=false;
 const cart:any={version:1,currencyCode:'CNY',items};
 const quote=()=>({quoteNo:'quote-'+(++quoteNo),cartVersion:cart.version,currencyCode:'CNY',expiresAt:Date.now()+3600000,totalAmountMinor:String(items.reduce((s,i)=>s+i.quantity*6900,0)-(claimed?1000:0)),subtotalMinor:String(items.reduce((s,i)=>s+i.quantity*6900,0)),discountMinor:claimed?'1000':'0',taxAmountMinor:'0',items:items.map(i=>({cartItemNo:i.cartItemNo,quoteItemNo:'quote-'+i.cartItemNo,totalAmountMinor:String(i.quantity*6900),configurationRequired:missingConfiguration&&i.cartItemNo==='single',configurationFrozen:!missingConfiguration||i.cartItemNo!=='single'}))});
 await page.route('**/x/idc-*/api/v1/account/**',async(r:any)=>{
  const request=r.request(),path=new URL(request.url()).pathname,body=request.postDataJSON();requests.push({path,method:request.method(),body,authorization:request.headers().authorization});
  const ok=(data:any)=>r.fulfill({json:{code:0,data}});
  if(path.endsWith('/commerce/guest-cart-merge'))return ok({cart});
  if(path.endsWith('/commerce/cart'))return ok({cart});
  if(path.endsWith('/commerce/quotes')){cart.latestQuote=quote();return ok({quote:cart.latestQuote});}
  if(path.endsWith('/purchase-configuration')){cart.latestQuote.items.find((i:any)=>i.quoteItemNo==='quote-single').configurationFrozen=true;return ok({});}
  if(path.includes('/commerce/cart-items/')){
   const index=items.findIndex(i=>path.endsWith('/'+i.cartItemNo));
   if(request.method()==='DELETE')items.splice(index,1);else{items[index].quantity=body.quantity;items[index].version++;}cart.version++;return ok({cart});
  }
  if(path.endsWith('/marketing/coupons/claims')){claimed=true;return ok({item:{assetNo:'coupon-controlled'}});}
  if(path.endsWith('/marketing/coupons'))return ok({items:claimed?[{assetNo:'coupon-controlled',nameZhCn:'受控优惠券',currencyCode:'CNY',status:'active',validFrom:Date.now()-1000,validUntil:Date.now()+3600000,consumedUses:0,usageLimit:1}]:[]});
  if(path.endsWith('/billing/overview'))return ok({overview:{balances:payment?[{accountTypeCode:'customer_balance',statusCode:'active',currencyCode:'CNY',availableBalanceMinor:'50000'}]:[]}});
  if(path.endsWith('/payment-channels'))return ok({channels:payment?[{channelNo:'card-controlled',displayName:'Controlled card',modes:['redirect']}]:[]});
  if(path.endsWith('/commerce/orders')){if(failOrder){failOrder=false;return r.fulfill({status:503,json:{code:503,data:null}});}return ok({checkout:{orderNo:'order-controlled',invoiceNo:'invoice-controlled'}});}
  if(path.endsWith('/payment-preparations'))return ok({preparation:{contextNo:'context-controlled',statusCode:state.preparation,ready:state.preparation==='ready'}});
  if(path.endsWith('/commerce/payment-preparations/context-controlled'))return ok({preparation:{contextNo:'context-controlled',statusCode:state.preparation,ready:state.preparation==='ready'}});
  if(path.endsWith('/account/payments')&&request.method()==='POST'){if(state.unknown)return r.abort('failed');return ok({attemptNo:'attempt-controlled',statusCode:'confirming',redirectMethod,redirectUrl:redirectMethod==='QR'?'weixin://wxpay/bizpayurl?pr=controlled':'https://payments.example/checkout',redirectFields:{signed:'controlled-signature'},expiresAt:Date.now()+3600000});}
  if(path.endsWith('/account/payments/attempt-controlled'))return ok({attemptNo:'attempt-controlled',statusCode:state.paid?'succeeded':'confirming'});
  if(path.endsWith('/billing/invoices/invoice-controlled'))return ok({detail:{invoice:{canPayFromBalance:!state.paid,statusCode:state.paid?'paid':'pending_payment',outstandingAmountMinor:state.paid?'0':'13800'}}});
  if(path.endsWith('/commerce/orders/order-controlled'))return ok({order:{summary:{statusCode:state.paid?'processing':'pending_payment'}}});
  if(path.endsWith('/billing/invoices/invoice-controlled/balance-payments')){state.paid=true;return ok({payment:{statusCode:'paid',outstandingAmountMinor:'0'}});}
  return r.fulfill({status:403,json:{code:403,data:null}});
 });
 await page.route('http://localhost:6032/auth/website-session**',(r:any)=>r.fulfill({contentType:'text/html',body:'<h1>Controlled sign in</h1><script>const q=new URLSearchParams(location.search);if(q.get("nonce"))parent.postMessage({type:"lina-website-session",nonce:q.get("nonce"),accessToken:""},q.get("websiteOrigin"));</script>'}));
 await page.route('http://localhost:6032/account/billing/invoices/**',(r:any)=>r.fulfill({contentType:'text/html',body:'<h1>Controlled invoice</h1>'}));
 await page.addInitScript(({revision})=>{localStorage.setItem('lina.website.locale','zh-CN');sessionStorage.setItem('website-main-purchase-drafts',JSON.stringify({'private-ref':{publicSlug:'server',configurationRevision:revision,values:[],serverLoginToken:'opaque-private-token',locale:'zh-CN'}}));},{revision});
 return {requests,cart,state};
}
test.describe('TC-156 官网购物车与订单确认',()=>{
 test('TC-156a: 单台凭据、数量报价、冻结token、幂等重试及官网付款',async({page},info)=>{
  const {requests}=await fixture(page,{missingConfiguration:true});const site=new CommerceCartPage(page);await site.open();
  await expect(site.total).toHaveText('CNY 138.00');await expect(site.item('single').locator('input[type=number]')).toBeDisabled();
  await expect(site.workspace).toHaveClass('allocation-card');await expect(site.workspace.locator('.allocation-header')).toHaveCount(1);await expect(site.workspace.locator('.allocation-mian')).toHaveCount(1);await expect(site.workspace.locator('.allocation-footer')).toHaveCount(1);await expect(page.locator('link[href*="cart-commerce.css"]')).toHaveCount(0);
  await expect(site.item('single')).toContainText('customer-host');await expect(site.item('single').getByRole('link',{name:/编辑配置/})).toHaveAttribute('href',/\/configure.html\?product=server&cartItemNo=single/);
  const frozen=requests.find(r=>r.path.endsWith('/quote-items/quote-single/purchase-configuration'));expect(frozen.body.serverLoginToken).toBe('opaque-private-token');expect(frozen.body).not.toHaveProperty('password');
  await site.screenshot(info.outputPath(Date.now()+'-cart-loaded.png'));
  await site.quantity('auto','2');await expect(site.total).toHaveText('CNY 207.00');
  const updated=requests.find(r=>r.method==='PUT');expect(updated.body).toEqual({expectedCartVersion:1,expectedItemVersion:1,quantity:2});expect(updated.authorization).toBe('Bearer controlled-session');
  await site.next();await site.payment('Controlled card');await expect(site.workspace.locator('.cart-details')).toContainText('customer-host');await expect(site.workspace.locator('.cart-details')).toContainText('CNY 207.00');
  await site.screenshot(info.outputPath(Date.now()+'-cart-confirm.png'));
  await site.submit();await expect(site.status).toContainText('操作未完成');await site.screenshot(info.outputPath(Date.now()+'-cart-order-failed.png'));
  await site.submit();await expect(page).toHaveURL(/\/cart.html$/);await expect(site.workspace.getByRole('link',{name:'打开支付页面',exact:true})).toHaveAttribute('href','https://payments.example/checkout');
  const orders=requests.filter(r=>r.path.endsWith('/commerce/orders'));expect(orders).toHaveLength(2);expect(orders[0].body).toEqual(orders[1].body);expect(orders[0].body.quoteNo).toBe('quote-2');expect(orders[0].body.idempotencyKey).toBeTruthy();
  expect(requests.filter(r=>r.path.endsWith('/account/payments')).map(r=>r.body)).toEqual([{paymentContextNo:'context-controlled',channelNo:'card-controlled',modeCode:'redirect',balanceAmountMinor:'0'}]);
 });
 test('TC-156b: 英文真实文案、移除项目、窄屏结算可达',async({page},info)=>{
  const {requests}=await fixture(page);const site=new CommerceCartPage(page);await site.open();await expect(site.total).toHaveText('CNY 138.00');await site.english();
  await expect(site.workspace.getByRole('heading',{name:'Shopping cart checkout',exact:true})).toBeVisible();await expect(site.item('single')).toContainText('Private server');await expect(site.item('single')).toContainText('Hostname');await expect(site.workspace.getByRole('link',{name:'← Continue shopping',exact:true})).toBeVisible();
  await site.item('auto').getByRole('button',{name:'Remove',exact:true}).click();await expect(site.item('auto')).toHaveCount(0);await expect(site.total).toHaveText('CNY 69.00');expect(requests.find(r=>r.method==='DELETE').path).toContain('/cart-items/auto');
  await page.setViewportSize({width:390,height:844});await site.showSteps();await expect(site.workspace.locator('.cart-steps li').last()).toBeInViewport();await site.screenshot(info.outputPath(Date.now()+'-cart-mobile-en-items.png'));
  await site.showCheckout();await expect(site.workspace.getByRole('button',{name:'Proceed to checkout',exact:true})).toBeInViewport();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await site.screenshot(info.outputPath(Date.now()+'-cart-mobile-en.png'));
 });
 test('TC-156c: 未登录不访问账户接口、客户中心登录回跳',async({page},info)=>{
  const {requests}=await fixture(page,{signedIn:false});const site=new CommerceCartPage(page);await site.open();await expect(site.status).toContainText('登录后即可');expect(requests).toHaveLength(0);
  await site.screenshot(info.outputPath(Date.now()+'-cart-signed-out.png'));await site.status.getByRole('button',{name:'登录',exact:true}).click();
  await expect(page).toHaveURL(/http:\/\/localhost:6032\/auth\/website-session\?login=1&returnTo=%2Fcart.html&websiteOrigin=/);
 });
 test('TC-156d: 优惠码失败与成功、官网选择并创建渠道付款',async({page},info)=>{
  const {requests}=await fixture(page,{payment:true});const site=new CommerceCartPage(page);await site.open();await expect(site.total).toHaveText('CNY 138.00');
  await site.coupon('bad');await expect(site.status).toContainText('请输入有效优惠码');expect(requests.filter(r=>r.path.endsWith('/coupons/claims'))).toHaveLength(0);
  await site.coupon('VALID123');await expect(site.total).toHaveText('CNY 128.00');await expect(site.status).toContainText('优惠已应用');
  expect(requests.find(r=>r.path.endsWith('/coupons/claims')).body.code).toBe('VALID123');expect(requests.filter(r=>r.path.endsWith('/commerce/quotes')).at(-1).body.assetNumbers).toEqual(['coupon-controlled']);
  await site.next();await site.payment('Controlled card');await expect(site.workspace.locator('.cart-payment-handoff')).toContainText('请在此选择支付方式');await site.screenshot(info.outputPath(Date.now()+'-cart-payment-preference.png'));
  await site.submit();await expect(site.status).toContainText('操作未完成');await site.submit();await expect(page).toHaveURL(/\/cart.html$/);await expect(site.workspace.getByRole('link',{name:'打开支付页面',exact:true})).toHaveAttribute('href','https://payments.example/checkout');
  expect(requests.filter(r=>r.method==='POST'&&r.path.endsWith('/account/payments'))).toHaveLength(1);
 });

 for(const method of ['QR','POST','GET'])test('TC-156'+({QR:'e',POST:'f',GET:'j'}[method])+': 官网付款 '+method,async({page},info)=>{
  const {state,requests}=await fixture(page,{redirectMethod:method});const site=new CommerceCartPage(page);await site.open();await expect(site.total).toHaveText('CNY 138.00');await site.next();await site.payment('Controlled card');await site.submit();await expect(site.status).toContainText('操作未完成');await site.submit();
  if(method==='QR'){await expect(site.workspace.getByRole('img',{name:'付款二维码'})).toBeVisible();expect(await site.workspace.getByRole('img',{name:'付款二维码'}).evaluate((image:HTMLImageElement)=>image.naturalWidth)).toBeGreaterThan(20);}
  else if(method==='POST'){const form=site.workspace.locator('.cart-payment-options form');await expect(form).toHaveAttribute('method','POST');await expect(form).toHaveAttribute('action','https://payments.example/checkout');await expect(form.locator('input[name=signed]')).toHaveValue('controlled-signature');}
  if(method!=='QR'){
   const transfers:any[]=[];await page.context().route('https://payments.example/**',route=>{transfers.push({method:route.request().method(),body:route.request().postData()});return route.fulfill({contentType:'text/html',body:'<h1>Controlled payment handoff</h1>'});});
   const popup=await site.openPayment();await expect(popup).toHaveURL('https://payments.example/checkout');expect(transfers[0].method).toBe(method);if(method==='POST')expect(transfers[0].body).toBe('signed=controlled-signature');await popup.close();
  }
  await site.screenshot(info.outputPath(Date.now()+'-website-payment-'+method+'.png'));state.paid=true;await site.checkPayment();await expect(site.status).toContainText('付款已确认');expect(requests.filter(r=>r.path.endsWith('/account/payments'))).toHaveLength(1);
 });
 test('TC-156g: 纯余额在官网直接付账单',async({page},info)=>{
  const {requests}=await fixture(page);const site=new CommerceCartPage(page);await site.open();await expect(site.total).toHaveText('CNY 138.00');await site.next();await site.payment('账户余额 可用余额 CNY 500.00');await site.submit();await expect(site.status).toContainText('操作未完成');await site.submit();await expect(site.status).toContainText('付款已确认');
  expect(requests.filter(r=>r.path.endsWith('/balance-payments'))).toHaveLength(1);expect(requests.find(r=>r.path.endsWith('/balance-payments')).body.idempotencyKey).toBeTruthy();expect(requests.filter(r=>r.path.endsWith('/account/payments'))).toHaveLength(0);await site.screenshot(info.outputPath(Date.now()+'-website-balance-paid.png'));
 });
 test('TC-156h: 未知付款提交刷新只查状态，不重发POST',async({page},info)=>{
  const {state,requests}=await fixture(page);state.unknown=true;const site=new CommerceCartPage(page);await site.open();await expect(site.total).toHaveText('CNY 138.00');await site.next();await site.payment('Controlled card');await site.submit();await expect(site.status).toContainText('操作未完成');await site.submit();await expect(site.workspace.locator('.cart-payment-handoff')).toContainText('付款提交结果暂未确认');
  await site.checkPayment();await expect(site.workspace.getByRole('button',{name:'继续付款',exact:true})).toHaveCount(0);await site.open();await expect(site.workspace.locator('.cart-payment-handoff')).toContainText('付款提交结果暂未确认');expect(requests.filter(r=>r.path.endsWith('/account/payments'))).toHaveLength(1);await site.screenshot(info.outputPath(Date.now()+'-website-payment-unknown.png'));await site.english();await expect(site.workspace.locator('.cart-payment-handoff')).toContainText('The payment submission result is unknown. Check status before taking any further action.');await expect(site.workspace.getByRole('button',{name:'Check payment status',exact:true})).toBeVisible();await site.screenshot(info.outputPath(Date.now()+'-website-payment-unknown-en.png'));
 });
 test('TC-156i: 认证和审核属于业务待办，禁止创建付款与误跳登录',async({page},info)=>{
  const {state,requests}=await fixture(page);state.preparation='awaiting_identity';const site=new CommerceCartPage(page);await site.open();await expect(site.total).toHaveText('CNY 138.00');await site.next();await site.payment('Controlled card');await site.submit();await expect(site.status).toContainText('操作未完成');await site.submit();await expect(site.workspace.locator('.cart-payment-handoff')).toContainText('实名认证');expect(requests.filter(r=>r.path.endsWith('/account/payments'))).toHaveLength(0);await expect(page).toHaveURL(/\/cart.html$/);
  state.preparation='awaiting_order_review';await site.continuePayment();await expect(site.workspace.locator('.cart-payment-handoff')).toContainText('订单正在审核');expect(requests.filter(r=>r.path.endsWith('/account/payments'))).toHaveLength(0);await site.screenshot(info.outputPath(Date.now()+'-website-payment-review.png'));
 });

});
