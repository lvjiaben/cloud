import { test, expect } from '@host-tests/fixtures/auth';
import { createServer } from 'node:http';
import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { mkdir } from 'node:fs/promises';
import { ProductPurchasePage } from '../pages/ProductPurchasePage';

const screenshotDirectory=resolve(fileURLToPath(new URL('../../../../../../',import.meta.url)),'temp',new Date().toISOString().slice(0,10).replaceAll('-',''));
const templateID=process.env.E2E_WEBSITE_TEMPLATE||'website-default';
if(!['website-default','website-minimal'].includes(templateID))throw new Error('Unsupported website fixture');
const pluginRoot = resolve(fileURLToPath(new URL('../../../../', import.meta.url)),templateID);
const product={publicSlug:'11',nameZhCN:'配置购买测试服务器',nameEnUS:'Purchase test server',summaryZhCN:'配置与下单验证',summaryEnUS:'Configuration and checkout',categoryNameZhCN:'云服务器',categoryNameEnUS:'Cloud servers',currencies:['CNY'],availability:'available'};
let origin='';
let renderer: ChildProcess;
let publicCampaigns:any[]=[];
let campaignRequestCount=0;
let campaignResponseFails=false;
let campaignExposureCount=0;
let publicPrimaryColor='#1677ff';
const api=createServer((req,res)=>{
  const path=new URL(req.url!,'http://local').pathname;
  let data: unknown;
  if(path.endsWith('/public/brand')) data={brand:{primaryColor:publicPrimaryColor,siteName:{zhCN:'云服务平台',enUS:'Cloud services'},footer:{zhCN:'服务支持',enUS:'Support'},navigation:[{label:{zhCN:'商品',enUS:'Products'},href:'/products'}],footerNavigation:[]}};
  else if(path.endsWith('/campaigns')) {campaignRequestCount++;if(campaignResponseFails){res.statusCode=503;res.end(JSON.stringify({code:50}));return;}data={campaigns:publicCampaigns};}
  else if(path.endsWith('/campaign-exposures')) {campaignExposureCount++;data={recorded:true};}
  else if(path.endsWith('/products/11')) data={product};
  else if(path.endsWith('/products')) data={products:[product],categories:[],availableCurrencies:['CNY'],nextCursor:''};
  else {res.statusCode=404;res.end(JSON.stringify({code:44}));return;}
  res.setHeader('content-type','application/json');res.end(JSON.stringify({code:0,data}));
});
test.beforeAll(async()=>{
  api.listen(0,'127.0.0.1');await once(api,'listening');
  const apiOrigin=`http://127.0.0.1:${(api.address() as any).port}`;
  const portProbe=createServer();portProbe.listen(0,'127.0.0.1');await once(portProbe,'listening');const port=(portProbe.address() as any).port;await new Promise<void>(done=>portProbe.close(()=>done()));
  origin=`http://127.0.0.1:${port}`;
  renderer=spawn(process.execPath,[resolve(pluginRoot,'frontend/site/.output/server/index.mjs')],{env:{...process.env,PORT:String(port),HOST:'127.0.0.1',NUXT_API_ORIGIN:apiOrigin,NUXT_PUBLIC_CUSTOMER_URL:'http://localhost:6032'},stdio:'ignore'});
  await expect.poll(async()=>{try{return(await fetch(origin+'/__site/health')).status;}catch{return 0;}},{timeout:20000}).toBe(200);
  await mkdir(screenshotDirectory,{recursive:true});
});
test.afterAll(async()=>{renderer?.kill();api.closeAllConnections();await new Promise<void>(done=>api.close(()=>done()));});

test.beforeEach(async({page})=>{
 product.currencies=['CNY'];publicCampaigns=[];campaignRequestCount=0;campaignResponseFails=false;campaignExposureCount=0;
 await page.route('**/__website/configuration',route=>route.fulfill({json:{origin,customerOrigin:origin}}));
 await page.context().route('**/auth/website-session?*',route=>{
  const url=new URL(route.request().url()),nonce=url.searchParams.get('nonce');
  const body=url.searchParams.get('login')==='1'
   ?`<script>localStorage.setItem('fixture.customer.signedIn','1');location.replace(${JSON.stringify(origin)}+${JSON.stringify(url.searchParams.get('returnTo')||'/cart')});</script>`
   :`<script>parent.postMessage({type:'lina-website-session',nonce:${JSON.stringify(nonce)},accessToken:localStorage.getItem('fixture.customer.signedIn')==='1'?'fixture-website-session':''},location.origin);</script>`;
  return route.fulfill({contentType:'text/html; charset=utf-8',body});
 });
});

test('TC-1a: 服务端商品详情接管后保持可用，公开配置进入购物车',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('fixture.customer.signedIn','1'));
 await page.route('**/account/commerce/guest-cart-merge',route=>route.fulfill({json:{code:0,data:{merged:true}}}));
 const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));page.on('console',m=>{if(m.type()==='error')errors.push(m.text());});
 const view={configurationRevision:'revision-current',fields:[],values:[]};let additions=0;
 await page.route('**/purchase-configuration*',route=>route.fulfill({json:{code:0,data:{...view,valid:true,issues:[]}}}));
 await page.route('**/purchase-configuration/quote',route=>route.fulfill({json:{code:0,data:{...view,currencyCode:'CNY',terms:[{billingTermToken:'monthly-token',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',subtotalAmount:'10000',setupAmount:'0',taxAmount:'0',totalAmount:'10000'}]}}}));
 await page.route('**/purchase-configuration/evaluate',route=>route.fulfill({json:{code:0,data:{...view,valid:true,issues:[]}}}));
 await page.route('**/account/commerce/carts',route=>route.fulfill({json:{code:0,data:{cart:{reference:'opaque-cookie-cart'}}}}));
 await page.route('**/account/commerce/cart-items',route=>{const body=route.request().postDataJSON();expect(body.publicSlug).toBe('11');expect(body.billingTermToken).toBe('monthly-token');additions++;return route.fulfill({json:{code:0,data:{cart:{reference:'opaque-cookie-cart'}}}});});
 await page.route('**/account/commerce/cart',route=>route.fulfill({json:{code:0,data:{cart:{reference:'cart-test',currencyCode:'CNY',version:1+additions,items:additions?[{cartItemNo:'item-test',configurationReference:'config-reference',productNameZhCN:'配置购买测试服务器',publicSlug:'11',quantity:1,purchaseCycles:1,version:1}]:[]}}}}));
 const ui=new ProductPurchasePage(page,origin);
 await ui.open();await ui.assertDetail();await page.reload();await ui.assertDetail();
 expect(errors).toEqual([]);
 await expect(page.getByTestId('public-cart')).toHaveAttribute('href','/cart');
 await ui.screenshot('detail');
 await page.getByRole('button',{name:'查看准确价格',exact:true}).click();
 await expect(page.getByTestId('public-add-to-cart')).toBeVisible();
 await page.getByTestId('public-add-to-cart').click();
 await expect(page.getByTestId('website-cart-item-item-test')).toContainText('配置购买测试服务器');await expect(page.getByRole('button',{name:'核对配置和价格',exact:true})).toBeVisible();expect(additions).toBe(1);
});

test('TC-1b: 无效商品展示错误与重试，不清空页面',async({page})=>{
 const ui=new ProductPurchasePage(page,origin);await ui.open('missing');
 await expect(page.getByText('商品暂时加载失败，请稍后重试。',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'重新加载',exact:true}).click();
 await expect(page.getByText('商品暂时加载失败，请稍后重试。',{exact:true})).toBeVisible();
 await ui.screenshot('missing');
});


test('TC-1c: 需要客户身份的报价先登录，原配置保留并重新核价',async({page})=>{
 const view={configurationRevision:'revision-current',fields:[],values:[]};
 await page.route('**/purchase-configuration*',route=>route.fulfill({json:{code:0,data:{...view,valid:true,issues:[]}}}));
 await page.route('**/purchase-configuration/evaluate',route=>route.fulfill({json:{code:0,data:{...view,valid:true,issues:[]}}}));
 await page.route('**/purchase-configuration/quote',route=>route.fulfill({json:{code:0,data:{...view,currencyCode:'CNY',terms:[{billingTermToken:'identity-term',labelZhCN:'每月',minimumPurchaseCycles:1,status:route.request().headers().authorization?'final':'requires_customer_context',totalAmount:'10000'}]}}}));
 const ui=new ProductPurchasePage(page,origin);await ui.open();await ui.assertDetail();
 await page.getByRole('button',{name:'查看准确价格',exact:true}).click();
 await expect(page.getByRole('button',{name:'登录并继续购买',exact:true})).toBeVisible();
 await expect(page.getByTestId('public-add-to-cart')).toHaveCount(0);
 await page.getByRole('button',{name:'登录并继续购买',exact:true}).click();
 await expect(page.getByTestId('public-add-to-cart')).toBeVisible();
 await expect(page).toHaveURL(origin+'/products/11');
});

test('TC-1d: 官网未登录结算提示登录，确认失败保留报价，只有最终付款进入账单',async({page})=>{
 const cart={cartNo:'cart-checkout',currencyCode:'CNY',version:2,items:[{cartItemNo:'cart-item',productNameZhCN:'结算测试服务器',publicSlug:'11',quantity:1,purchaseCycles:1,version:1,configurationRequired:true,configurationFrozen:true}]};
 const orders:any[]=[];let fail=true;let merges=0;
 await page.route('**/x/idc-core/api/v1/**',async route=>{
  const url=new URL(route.request().url());let data:unknown;
  if(url.pathname.includes('/account/'))expect(route.request().headers().authorization).toBe('Bearer fixture-website-session');
  if(url.pathname.endsWith('/guest-cart-merge')){merges++;data={merged:true,cart};}
  else if(url.pathname.endsWith('/commerce/cart'))data={cart};
  else if(url.pathname.endsWith('/commerce/quotes'))data={quote:{quoteNo:'quote-checkout',cartVersion:2,currencyCode:'CNY',subtotalMinor:'110000',discountMinor:'0',taxAmountMinor:'0',totalAmountMinor:'110000',expiresAt:Date.now()+60000,items:cart.items.map(item=>({...item,quoteItemNo:'quote-item'}))}};
  else if(url.pathname.endsWith('/commerce/orders')){
   orders.push(route.request().postDataJSON());
   if(fail){await route.fulfill({status:503,json:{code:50}});return;}
   data={checkout:{orderNo:'order-checkout',invoiceNo:'invoice-checkout'}};
  }else throw new Error('Unexpected checkout path '+url.pathname);
  await route.fulfill({json:{code:0,data}});
 });
 await page.route('**/account/billing/invoices/invoice-checkout*',route=>route.fulfill({contentType:'text/html; charset=utf-8',body:'<h1>用户中心账单付款</h1>'}));
 await page.goto(origin+'/cart');
 await expect(page.getByTestId('website-cart-item-cart-item')).toContainText('结算测试服务器');
 await expect(page.getByText('请先登录后结算',{exact:true})).toBeVisible();expect(orders).toHaveLength(0);
 await page.getByRole('button',{name:'登录并继续结算',exact:true}).last().click();
 await expect(page).toHaveURL(origin+'/checkout');
 await expect(page.getByTestId('website-final-quote')).toContainText('CNY 1100.00');expect(merges).toBeGreaterThan(0);
 await page.screenshot({path:resolve(screenshotDirectory,'192000-website-confirm-order.png'),fullPage:true});
 await page.getByTestId('website-submit-order').click();
 await expect(page.getByText('操作暂时无法完成，请刷新购物车后重试。',{exact:true})).toBeVisible();
 await expect(page.getByTestId('website-final-quote')).toContainText('CNY 1100.00');
 await page.screenshot({path:resolve(screenshotDirectory,'192000-website-order-error.png'),fullPage:true});
 fail=false;await page.getByTestId('website-submit-order').click();
 await expect(page.getByRole('heading',{name:'用户中心账单付款'})).toBeVisible();
 expect(orders).toHaveLength(2);expect(orders[0]).toEqual(orders[1]);expect(orders[1].quoteNo).toBe('quote-checkout');expect(orders[1].idempotencyKey).toBeTruthy();
});


test('TC-1e: 官网购物车英文文案与移动端布局正确',async({page})=>{
 await page.route('**/public/commerce/cart',route=>route.fulfill({json:{code:0,data:{cart:{reference:'english-cart',currencyCode:'CNY',version:1,items:[{cartItemNo:'english-item',productNameZhCN:'测试服务器',productNameEnUS:'Cloud server',publicSlug:'11',quantity:1,purchaseCycles:1,version:1}]}}}}));
 await page.setViewportSize({width:390,height:844});await page.goto(origin+'/en-US/cart');
 await expect(page.getByRole('heading',{name:'Cart',exact:true})).toBeVisible();await expect(page.getByText('Sign in before checking out',{exact:true})).toBeVisible();
 await expect(page.getByTestId('website-cart-item-english-item')).toContainText('Cloud server');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.screenshot({path:resolve(screenshotDirectory,'204500-website-mobile-en.png'),fullPage:true});
});

test('TC-1f: 移除失败保留商品，重试后剩余商品与刷新一致',async({page})=>{
 let items=[1,2].map(n=>({cartItemNo:'remove-'+n,productNameZhCN:'移除测试商品'+n,publicSlug:'11',quantity:1,purchaseCycles:1,version:1}));let version=3,fail=true;
 await page.route('**/public/commerce/cart',route=>route.fulfill({json:{code:0,data:{cart:{reference:'remove-cart',currencyCode:'CNY',version,items}}}}));
 await page.route('**/public/commerce/cart-items/*?*',async route=>{
  const url=new URL(route.request().url());expect(route.request().method()).toBe('DELETE');expect(url.searchParams.get('expectedCartVersion')).toBe(String(version));
  if(fail)return route.fulfill({status:409,json:{code:409}});
  items=items.filter(item=>item.cartItemNo!==url.pathname.split('/').at(-1));version++;
  return route.fulfill({json:{code:0,data:{cart:{reference:'remove-cart',currencyCode:'CNY',version,items}}}});
 });
 await page.goto(origin+'/cart');
 const item=page.getByTestId('website-cart-item-remove-1');await item.getByRole('button',{name:/移\s*除/}).click();
 await expect(page.getByText('操作暂时无法完成，请刷新购物车后重试。',{exact:true})).toBeVisible();await expect(item).toBeVisible();
 await page.screenshot({path:resolve(screenshotDirectory,'222000-cart-remove-error.png'),fullPage:true});
 fail=false;await item.getByRole('button',{name:/移\s*除/}).click();await expect(item).toHaveCount(0);await expect(page.getByTestId('website-cart-item-remove-2')).toContainText('移除测试商品2');
 await page.reload();await expect(item).toHaveCount(0);await expect(page.getByTestId('website-cart-item-remove-2')).toBeVisible();
 await page.screenshot({path:resolve(screenshotDirectory,'222000-cart-remove-success.png'),fullPage:true});
});

test('TC-1g: 已登录客户直接结算并提交已选配置，无登录窗口',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('fixture.customer.signedIn','1'));
 const item={cartItemNo:'already-signed-item',configurationReference:'retained-config-reference',productNameZhCN:'已选系统服务器',publicSlug:'11',quantity:1,purchaseCycles:1,version:1,configurationRequired:true,configurationFrozen:false};
 const final={quoteNo:'selected-quote',cartVersion:3,currencyCode:'CNY',subtotalMinor:'10000',discountMinor:'0',taxAmountMinor:'0',totalAmountMinor:'10000',expiresAt:Date.now()+60000,items:[{...item,quoteItemNo:'selected-quote-item',totalAmountMinor:'10000'}]};
 const cart={cartNo:'signed-cart',currencyCode:'CNY',version:2,items:[item],latestQuote:undefined as unknown};let freezes=0,quotes=0;
 await page.addInitScript(()=>sessionStorage.setItem('lina.website.purchase-drafts',JSON.stringify({'retained-config-reference':{publicSlug:'11',configurationRevision:'original-revision',values:[{fieldKey:'os',stringValue:'Debian'}],summary:[{labelZhCN:'操作系统',labelEnUS:'Operating system',valueZhCN:'Debian',valueEnUS:'Debian'}],preview:{currencyCode:'CNY',totalAmountMinor:'1100',termZhCN:'每月',termEnUS:'Monthly'}}})));
 await page.route('**/x/idc-core/api/v1/account/commerce/**',async route=>{
  expect(route.request().headers().authorization).toBe('Bearer fixture-website-session');const url=new URL(route.request().url());
  if(url.pathname.endsWith('/guest-cart-merge'))return route.fulfill({json:{code:0,data:{merged:false}}});
  if(url.pathname.endsWith('/commerce/cart'))return route.fulfill({json:{code:0,data:{cart}}});
  if(url.pathname.endsWith('/commerce/quotes')){quotes++;cart.latestQuote=final;cart.version=3;return route.fulfill({json:{code:0,data:{quote:final}}});}
  if(url.pathname.endsWith('/purchase-configuration')){
   expect(route.request().postDataJSON()).toEqual({publicSlug:'11',configurationRevision:'original-revision',values:[{fieldKey:'os',stringValue:'Debian'}]});freezes++;final.items[0]!.configurationFrozen=true;
   return route.fulfill({json:{code:0,data:{configurationFrozen:true}}});
  }
  throw new Error('Unexpected '+url.pathname);
 });
 const popups:string[]=[];page.on('popup',()=>popups.push('popup'));
 await page.goto(origin+'/cart');await expect(page.getByRole('button',{name:'核对配置和价格',exact:true})).toBeVisible();
 await expect(page.getByText('请先登录后结算',{exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'核对配置和价格',exact:true}).click();await expect(page).toHaveURL(origin+'/checkout');
 await expect(page.getByTestId('website-submit-order')).toBeEnabled();expect(freezes).toBe(1);expect(quotes).toBe(1);expect(popups).toEqual([]);
 await expect(page.getByTestId('website-final-quote')).toContainText('CNY 100.00');
 await expect(page.getByTestId('website-cart-configuration')).toContainText('Debian');await expect(page.getByTestId('website-cart-preview')).toContainText('CNY 11.00');await expect(page.getByTestId('website-cart-final-item')).toContainText('CNY 100.00');await expect(page.getByText('本项最终金额与加入购物车时不同，请核对配置和费用后再确认。',{exact:true})).toBeVisible();
 await page.screenshot({path:resolve(screenshotDirectory,'222000-already-signed-confirm.png'),fullPage:true});
});

test('TC-1h: 旧购物车在官网逐项补全配置，校验失败不得下单',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('fixture.customer.signedIn','1'));
 const items=[1,2].map(n=>({cartItemNo:'old-item-'+n,productNameZhCN:'待补全商品'+n,publicSlug:'11',quantity:1,purchaseCycles:1,version:1,configurationRequired:true,configurationFrozen:false,quoteItemNo:'old-quote-item-'+n}));
 const quote={quoteNo:'old-quote',cartVersion:3,currencyCode:'CNY',subtotalMinor:'20000',discountMinor:'0',taxAmountMinor:'0',totalAmountMinor:'20000',expiresAt:Date.now()+60000,items};let loaded=0,fail=true;
 const cart={cartNo:'old-cart',currencyCode:'CNY',version:2,items,latestQuote:undefined as unknown};
 await page.route('**/x/idc-core/api/v1/account/commerce/**',route=>{
  const url=new URL(route.request().url());
  if(url.pathname.endsWith('/guest-cart-merge'))return route.fulfill({json:{code:0,data:{merged:false}}});
  if(url.pathname.endsWith('/commerce/cart'))return route.fulfill({json:{code:0,data:{cart}}});
  if(url.pathname.endsWith('/commerce/quotes')){cart.latestQuote=quote;return route.fulfill({json:{code:0,data:{quote}}});}
  if(url.pathname.endsWith('/purchase-configuration')){
   if(fail)return route.fulfill({status:409,json:{code:409}});
   const number=url.pathname.split('/').at(-2);items.find(item=>item.quoteItemNo===number)!.configurationFrozen=true;
   return route.fulfill({json:{code:0,data:{configurationFrozen:true}}});
  }
  throw new Error('Unexpected '+url.pathname);
 });
 await page.route('**/public/catalog/products/11/purchase-configuration',route=>{loaded++;return route.fulfill({json:{code:0,data:{configurationRevision:'original-revision',fields:[],values:[]}}});});
 await page.route('**/purchase-configuration/evaluate',route=>route.fulfill({json:{code:0,data:{valid:true,issues:[],fields:[],values:[]}}}));
 await page.goto(origin+'/checkout');await expect(page.getByRole('button',{name:'补全配置',exact:true})).toHaveCount(2);expect(loaded).toBe(0);
 await expect(page.getByTestId('website-submit-order')).toBeDisabled();
 await page.getByTestId('website-cart-item-old-item-1').getByRole('button',{name:'补全配置',exact:true}).click();
 await page.getByRole('button',{name:'保存配置并继续',exact:true}).click();
 await expect(page.getByText('配置与当前报价不一致或已失效，请返回商品重新选购。',{exact:true})).toBeVisible();await expect(page.getByTestId('website-submit-order')).toBeDisabled();expect(loaded).toBe(1);
 await page.screenshot({path:resolve(screenshotDirectory,'231000-old-cart-configuration-error.png'),fullPage:true});
 fail=false;await page.getByRole('button',{name:'保存配置并继续',exact:true}).click();await expect(page.getByRole('button',{name:'补全配置',exact:true})).toHaveCount(1);
 await page.getByTestId('website-cart-item-old-item-2').getByRole('button',{name:'补全配置',exact:true}).click();await page.getByRole('button',{name:'保存配置并继续',exact:true}).click();
 await expect(page.getByRole('button',{name:'补全配置',exact:true})).toHaveCount(0);await expect(page.getByTestId('website-submit-order')).toBeEnabled();expect(loaded).toBe(2);
 await page.screenshot({path:resolve(screenshotDirectory,'231000-old-cart-configuration-complete.png'),fullPage:true});
});

test('TC-1i: 浏览器隔离隐藏会话后自动在当前标签交接并恢复购物车',async({page})=>{
 const customerOrigin=origin.replace('127.0.0.1','localhost');let handedOff=false,posts=0,probes=0;
 await page.route('**/__website/configuration',route=>route.fulfill({json:{origin,customerOrigin}}));
 await page.route('**/__website/session',async route=>{
  if(route.request().method()==='POST'){
   expect(route.request().headers().origin).toBe(customerOrigin);const form=new URLSearchParams(route.request().postData()||'');expect(form.get('accessToken')).toBe('fixture-website-session');posts++;handedOff=true;
   return route.fulfill({status:303,headers:{location:origin+'/cart'}});
  }
  return route.fulfill({json:{checked:handedOff,accessToken:handedOff?'fixture-website-session':''}});
 });
 await page.context().route('**/auth/website-session?*',route=>{
  const url=new URL(route.request().url());let body:string;
  if(url.searchParams.get('probe')==='1'){
   probes++;expect(url.searchParams.get('websiteOrigin')).toBe(origin);
   body=`<form method="post" action="${origin}/__website/session"><input name="accessToken" value="fixture-website-session"><input name="returnTo" value="/cart"></form><script>document.forms[0].submit()</script>`;
  }else body=`<script>parent.postMessage({type:'lina-website-session',nonce:${JSON.stringify(url.searchParams.get('nonce'))},accessToken:''},${JSON.stringify(origin)});</script>`;
  return route.fulfill({contentType:'text/html; charset=utf-8',body});
 });
 await page.route('**/x/idc-core/api/v1/**',route=>{
  const url=new URL(route.request().url());
  if(url.pathname.includes('/account/'))expect(route.request().headers().authorization).toBe('Bearer fixture-website-session');
  if(url.pathname.endsWith('/guest-cart-merge'))return route.fulfill({json:{code:0,data:{merged:false}}});
  if(url.pathname.endsWith('/commerce/cart'))return route.fulfill({json:{code:0,data:{cart:{cartNo:'partitioned-cart',currencyCode:'CNY',version:1,items:[{cartItemNo:'partitioned-item',productNameZhCN:'会话恢复商品',publicSlug:'11',quantity:1,purchaseCycles:1,version:1}]}}}});
  return route.continue();
 });
 await page.goto(origin+'/cart');await expect(page.getByRole('button',{name:'核对配置和价格',exact:true})).toBeVisible();
 await expect(page.getByTestId('website-cart-item-partitioned-item')).toContainText('会话恢复商品');await expect(page.getByText('请先登录后结算',{exact:true})).toHaveCount(0);
 expect(posts).toBe(1);expect(probes).toBe(1);expect(page.context().pages()).toHaveLength(1);await expect(page).toHaveURL(origin+'/cart');
 await page.screenshot({path:resolve(screenshotDirectory,'235900-partitioned-session-restored.png'),fullPage:true});
});

test('TC-1j: 晚到校验不覆盖新输入，加购固定点击时选配与核价',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('fixture.customer.signedIn','1'));
 await page.route('**/account/commerce/guest-cart-merge',route=>route.fulfill({json:{code:0,data:{merged:true}}}));
 const field={fieldToken:'memory',type:'integer',labelZhCN:'内存（GiB）',labelEnUS:'Memory (GiB)',descriptionZhCN:'1 GiB = 1024 MiB',descriptionEnUS:'1 GiB = 1024 MiB',minimumInteger:'1',maximumInteger:'1024',visible:true,disabled:false,required:true};
 const view={configurationRevision:'memory-revision',fields:[field],values:[{fieldToken:'memory',type:'integer',integerValue:'1'}]};
 await page.route('**/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
 let releaseOld!:()=>void,oldSeen!:()=>void;
 const oldRequest=new Promise<void>(resolve=>oldSeen=resolve),oldResponse=new Promise<void>(resolve=>releaseOld=resolve);
 await page.route('**/purchase-configuration/evaluate',async r=>{
  const values=r.request().postDataJSON().values;
  if(values[0]?.integerValue==='2'){oldSeen();await oldResponse;}
  await r.fulfill({json:{code:0,data:{...view,values,valid:true,issues:[]}}});
 });
 await page.route('**/purchase-configuration/quote',r=>{
  expect(r.request().postDataJSON().values[0].integerValue).toBe('3');
  return r.fulfill({json:{code:0,data:{configurationRevision:view.configurationRevision,currencyCode:'CNY',terms:[{billingTermToken:'memory-three',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',setupAmount:'1100',subtotalAmount:'1500',taxAmount:'0',totalAmount:'2600'}]}}});
 });
 let releaseCart!:()=>void,cartSeen!:()=>void;
 const cartRequest=new Promise<void>(resolve=>cartSeen=resolve),cartResponse=new Promise<void>(resolve=>releaseCart=resolve);
 let added=false;
 await page.route('**/account/commerce/cart-items',r=>{expect(r.request().postDataJSON().values).toEqual([{fieldToken:'memory',type:'integer',integerValue:'3'}]);added=true;return r.fulfill({json:{code:0,data:{cart:{reference:'memory-cart'}}}});});
 let cartReads=0;await page.route('**/account/commerce/cart',async r=>{if(++cartReads===1){cartSeen();await cartResponse;}return r.fulfill({json:{code:0,data:{cart:{reference:'memory-cart',currencyCode:'CNY',version:added?2:1,items:added?[{cartItemNo:'memory-item',configurationReference:'memory-config',productNameZhCN:'内存核价商品',publicSlug:'11',quantity:1,purchaseCycles:1,version:1}]:[]}}}});});
 await page.goto(origin+'/products/11');
 const form=page.getByTestId('public-product-configurator');await expect(form).toContainText('内存（GiB）');const input=form.getByRole('spinbutton');await expect(input).toHaveValue('1');
 await input.fill('2');await oldRequest;await input.fill('3');releaseOld();
 const quote=page.getByRole('button',{name:'查看准确价格',exact:true});await expect(quote).toBeEnabled();await expect(input).toHaveValue('3');
 await quote.click();await expect(page.getByTestId('public-add-to-cart')).toBeVisible();
 await page.getByTestId('public-add-to-cart').click();await cartRequest;await input.fill('4');releaseCart();
 const item=page.getByTestId('website-cart-item-memory-item');await expect(item).toBeVisible();
 await expect(item.getByTestId('website-cart-configuration')).toContainText('内存（GiB）');await expect(item.getByTestId('website-cart-configuration')).toContainText('3');
 await expect(item.getByTestId('website-cart-preview')).toContainText('CNY 26.00');
 await page.screenshot({path:resolve(screenshotDirectory,'120000-late-evaluation-cart-price.png'),fullPage:true});
});

test('TC-1k: 本地滑块下拉单选保持数值类型，首次不重复校验，失败保留输入',async({page})=>{
 const numeric=(token:string,label:string,displayControl:string,extra:any={})=>({fieldToken:token,type:'integer',labelZhCN:label,labelEnUS:label,descriptionZhCN:'',descriptionEnUS:'',required:true,visible:true,disabled:false,minimumInteger:'2',maximumInteger:'16',quantityStep:'2',displayControl,...extra});
 const fields=[numeric('cpu','处理器核心数','slider',{maximumInteger:'15'}),numeric('memory','内存（GiB）','select',{numericChoices:['2','4','8','16']}),numeric('bandwidth','带宽（Mbps）','radio',{numericChoices:['2','4','8']})];
 const initial={configurationRevision:'local-controls',fields,values:[],ready:false};
 let evaluations=0;let rejectNext=true;let lastValues:any[]=[];
 let initialReads=0;await page.route('**/purchase-configuration',route=>{initialReads++;return initialReads===1?route.fulfill({status:503,json:{code:50}}):route.fulfill({json:{code:0,data:initial}});});
 await page.route('**/purchase-configuration/evaluate',async route=>{
  evaluations++;lastValues=route.request().postDataJSON().values;
  if(rejectNext){rejectNext=false;await route.fulfill({status:503,json:{code:50}});return;}
  await route.fulfill({json:{code:0,data:{...initial,values:lastValues,valid:lastValues.length===3,issues:[]}}});
 });
 let quoted:any[]=[];await page.route('**/purchase-configuration/quote',route=>{quoted=route.request().postDataJSON().values;return route.fulfill({json:{code:0,data:{configurationRevision:'local-controls',currencyCode:'CNY',terms:[{billingTermToken:'local-control-term',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',subtotalAmount:'1100',setupAmount:'0',taxAmount:'0',totalAmount:'1100'}]}}});});
 const ui=new ProductPurchasePage(page,origin);await ui.open();await ui.assertDetail();
 await expect(page.getByRole('slider')).toBeVisible();await page.waitForTimeout(400);expect(evaluations).toBe(0);expect(initialReads).toBe(2);
 await page.getByRole('slider').focus();await page.keyboard.press('ArrowRight');
 await expect(page.getByTestId('configuration-evaluation-error')).toBeVisible();
 await expect(page.getByRole('spinbutton')).toHaveValue('4');
 await page.screenshot({path:resolve(screenshotDirectory,'145500-control-error.png'),fullPage:true});
 await page.getByTestId('configuration-evaluation-error').getByRole('button',{name:'重新加载',exact:true}).click();
 await expect(page.getByTestId('configuration-evaluation-error')).toBeHidden();
 await page.getByRole('slider').focus();await page.keyboard.press('End');await expect(page.getByRole('spinbutton')).toHaveValue('14');await page.keyboard.press('Home');await page.keyboard.press('ArrowRight');
 const form=page.getByTestId('public-product-configurator');await form.locator('.ant-select').first().click();await page.locator('.ant-select-dropdown:visible').getByText('8',{exact:true}).click();
 await form.locator('.ant-radio-button-wrapper').filter({hasText:'4'}).click();
 await expect(page.getByRole('button',{name:'查看准确价格',exact:true})).toBeEnabled();
 await page.getByRole('button',{name:'查看准确价格',exact:true}).click();
 await expect(page.getByTestId('public-add-to-cart')).toBeVisible();
 expect(quoted).toEqual(expect.arrayContaining([{fieldToken:'cpu',type:'integer',integerValue:'4'},{fieldToken:'memory',type:'integer',integerValue:'8'},{fieldToken:'bandwidth',type:'integer',integerValue:'4'}]));
 expect(quoted.every(value=>!value.optionTokens)).toBe(true);
 await page.evaluate(()=>window.scrollTo(0,0));await page.waitForTimeout(100);
 await page.screenshot({path:resolve(screenshotDirectory,'145500-local-controls-quote.png'),fullPage:true});
});

test('TC-1l: 客户中心会话探测未响应时官网配置仍立即显示',async({page})=>{
 test.setTimeout(10000);
 await page.route('**/auth/website-session?*',route=>route.fulfill({contentType:'text/html',body:'<!doctype html><title>Session probe delayed</title>'}));
 let reads=0;await page.route('**/purchase-configuration',route=>{reads++;return route.fulfill({json:{code:0,data:{configurationRevision:'public-without-session',ready:true,fields:[],values:[]}}});});
 const ui=new ProductPurchasePage(page,origin);await ui.open();await ui.assertDetail();
 await expect(page.getByText('这个商品不需要客户填写额外配置。',{exact:true})).toBeVisible({timeout:3000});
 expect(reads).toBe(1);await expect(page.getByRole('button',{name:'查看准确价格',exact:true})).toBeEnabled();
 await page.screenshot({path:resolve(screenshotDirectory,'163600-delayed-session-configuration.png'),fullPage:true});
});

test('TC-1m: 本地分组、规格名称和高级文本配置保留类型后报价',async({page})=>{
 test.setTimeout(30000);
 const view={configurationRevision:'local-custom-fields',ready:true,fields:[{fieldToken:'memory',type:'integer',labelZhCN:'内存',labelEnUS:'Memory',descriptionZhCN:'选择内存规格',descriptionEnUS:'Choose memory',required:true,visible:true,disabled:false,displayControl:'select',quantityStep:'1',minimumInteger:'2',maximumInteger:'4',numericChoices:['2','4'],numericLabels:{'2':{zhCN:'2 GiB',enUS:'2 GiB'},'4':{zhCN:'4 GiB',enUS:'4 GiB'}},groupZhCN:'硬件规格',groupEnUS:'Hardware',unit:'GiB'},{fieldToken:'note',type:'multi_line',labelZhCN:'工单备注',labelEnUS:'Order note',descriptionZhCN:'本地记录',descriptionEnUS:'Local record',visible:true,required:false,disabled:false,advanced:true,groupZhCN:'服务设置',groupEnUS:'Service settings'}],values:[{fieldToken:'memory',type:'integer',integerValue:'2'}]};
 await page.route('**/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
 await page.route('**/purchase-configuration/evaluate',r=>r.fulfill({json:{code:0,data:{...view,values:r.request().postDataJSON().values,valid:true,issues:[]}}}));
 let quoted:any[]=[];await page.route('**/purchase-configuration/quote',r=>{quoted=r.request().postDataJSON().values;return r.fulfill({json:{code:0,data:{configurationRevision:view.configurationRevision,currencyCode:'CNY',terms:[{billingTermToken:'custom-field-term',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',setupAmount:'0',subtotalAmount:'500',taxAmount:'0',totalAmount:'500'}]}}});});
 const ui=new ProductPurchasePage(page,origin);await ui.open();await ui.assertDetail();const form=page.getByTestId('public-product-configurator');await expect(form.getByRole('heading',{name:'硬件规格',exact:true})).toBeVisible();await expect(form).toContainText('2 GiB');await expect(form.locator('.ant-form-item').filter({has:page.getByText('工单备注',{exact:true})}).getByRole('textbox')).toBeHidden();
 await page.getByTestId('purchase-advanced-toggle').click();await expect(form.getByRole('heading',{name:'服务设置',exact:true})).toBeVisible();await form.locator('.ant-form-item').filter({has:page.getByText('工单备注',{exact:true})}).getByRole('textbox').fill('需要中文说明书');await page.getByRole('button',{name:'查看准确价格',exact:true}).click();
 await expect.poll(()=>quoted).toEqual(expect.arrayContaining([{fieldToken:'memory',type:'integer',integerValue:'2'},{fieldToken:'note',type:'multi_line',textValue:'需要中文说明书'}]));await page.screenshot({path:resolve(screenshotDirectory,'181700-local-groups-advanced.png'),fullPage:true});
});

test('TC-1n: 实际本地官网网关加载新版分组及高级配置资源',async({page})=>{
 const live=process.env.E2E_WEBSITE_LIVE_ORIGIN;test.skip(!live,'Set the explicit local website gateway for deployment smoke verification');
 const target=new URL(live!);expect(['localhost','127.0.0.1']).toContain(target.hostname);test.setTimeout(15000);
 await page.route('**/purchase-configuration',r=>r.fulfill({json:{code:0,data:{configurationRevision:'local-live-widget-smoke',ready:true,fields:[{fieldToken:'memory',type:'integer',labelZhCN:'内存',labelEnUS:'Memory',descriptionZhCN:'',descriptionEnUS:'',visible:true,required:true,disabled:false,displayControl:'select',minimumInteger:'2',maximumInteger:'2',numericChoices:['2'],numericLabels:{'2':{zhCN:'2 GiB',enUS:'2 GiB'}},groupZhCN:'本地规格分组',groupEnUS:'Local specifications',unit:'GiB'},{fieldToken:'note',type:'multi_line',labelZhCN:'本地备注',labelEnUS:'Local note',descriptionZhCN:'',descriptionEnUS:'',visible:true,required:false,disabled:false,advanced:true}],values:[{fieldToken:'memory',type:'integer',integerValue:'2'}]}}}));
 await page.goto(target.origin+'/products/11');const form=page.getByTestId('public-product-configurator');await expect(form.getByRole('heading',{name:'本地规格分组',exact:true})).toBeVisible({timeout:5000});await expect(form).toContainText('2 GiB');await page.getByTestId('purchase-advanced-toggle').click();await expect(form.locator('.ant-form-item').filter({has:page.getByText('本地备注',{exact:true})}).getByRole('textbox')).toBeVisible();await page.screenshot({path:resolve(screenshotDirectory,'183600-live-website-custom-fields.png'),fullPage:true});
});

test('TC-1o: 配置摘要保留规格名称和单位，改配置立即撤销旧报价，窄屏仍能核价', async ({page}) => {
 const field={fieldToken:'memory-summary',type:'integer',labelZhCN:'内存',labelEnUS:'Memory',descriptionZhCN:'选择内存',descriptionEnUS:'Choose memory',visible:true,required:true,disabled:false,displayControl:'radio',minimumInteger:'2',maximumInteger:'4',numericChoices:['2','4'],numericLabels:{'2':{zhCN:'2 GiB',enUS:'2 GiB'},'4':{zhCN:'4 GiB',enUS:'4 GiB'}},unit:'GiB'};
 const view={configurationRevision:'summary-revision',ready:true,fields:[field],values:[{fieldToken:field.fieldToken,type:'integer',integerValue:'2'}]};
 await page.route('**/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
 await page.route('**/purchase-configuration/evaluate',r=>r.fulfill({json:{code:0,data:{...view,values:r.request().postDataJSON().values,valid:true,issues:[]}}}));
 await page.route('**/purchase-configuration/quote',r=>r.fulfill({json:{code:0,data:{configurationRevision:view.configurationRevision,currencyCode:'CNY',terms:[{billingTermToken:'summary-term',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',totalAmount:'2000'}]}}}));
 const ui=new ProductPurchasePage(page,origin);await ui.open();await ui.assertDetail();
 await expect(page.getByTestId('purchase-selected-configuration')).toContainText('2 GiB');
 await expect(page.getByTestId('purchase-selected-configuration')).not.toContainText('2 GiB GiB');
 await page.getByRole('button',{name:'查看准确价格',exact:true}).click();await expect(page.getByTestId('public-add-to-cart')).toBeVisible();
 await ui.selectSpecification('4 GiB');
 await expect(page.getByTestId('public-add-to-cart')).toHaveCount(0);
 await expect(page.getByTestId('purchase-selected-configuration')).toContainText('4 GiB');
 await expect(page.getByRole('button',{name:'查看准确价格',exact:true})).toBeEnabled();
 await ui.screenshot('configuration-summary-desktop');
 await page.setViewportSize({width:390,height:844});
 await expect(page.getByRole('button',{name:'查看准确价格',exact:true})).toBeEnabled();
 await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.getByRole('button',{name:'查看准确价格',exact:true}).click();await expect(page.getByTestId('public-add-to-cart')).toBeVisible();
 await ui.screenshot('configuration-summary-mobile');
});

const quoteCases=[
 {id:'p',currency:'CNY',subtotal:'100',setup:'1000',tax:'11',total:'1100',shown:['CNY 1.00','CNY 10.00','CNY 0.11','CNY 11.00'],english:false},
 {id:'q',currency:'JPY',subtotal:'900719925474099301',setup:'7',tax:'0',total:'900719925474099308',shown:['JPY 900719925474099301','JPY 7','JPY 0','JPY 900719925474099308'],english:false},
 {id:'r',currency:'KWD',subtotal:'12345',setup:'1000',tax:'667',total:'14012',shown:['KWD 12.345','KWD 1.000','KWD 0.667','KWD 14.012'],english:true},
];
for(const item of quoteCases) test(`TC-1${item.id}: ${item.currency}权威报价明细精确显示，一次费与本次合计分开`,async({page})=>{
 product.currencies=[item.currency];
 const view={configurationRevision:'fees-current',fields:[],values:[]};
 await page.route('**/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
 await page.route('**/purchase-configuration/evaluate',r=>r.fulfill({json:{code:0,data:{...view,valid:true,issues:[]}}}));
 await page.route('**/purchase-configuration/quote',r=>r.fulfill({json:{code:0,data:{...view,currencyCode:item.currency,terms:[{billingTermToken:'fees-term',labelZhCN:'1月',labelEnUS:'1 month',minimumPurchaseCycles:item.english?3:1,status:'final',subtotalAmount:item.subtotal,setupAmount:item.setup,taxAmount:item.tax,totalAmount:item.total}]}}}));
 let writes=0;
 await page.route('**/commerce/guest-carts/**',r=>{writes++;return r.fulfill({status:400,json:{code:40}});});
 const ui=new ProductPurchasePage(page,origin);
 await page.goto(origin+(item.english?'/en-US':'')+'/products/11');
 await page.getByRole('button',{name:item.english?'Check exact price':'查看准确价格',exact:true}).click();
 const breakdown=page.getByTestId('public-quote-breakdown');
 await expect(breakdown).toContainText(item.english?'Product and configuration':'商品与配置');
 await expect(breakdown).toContainText(item.english?'One-time fee':'一次性费用');
 await expect(breakdown).toContainText(item.english?'Tax included in total':'税费（合计已包含）');
 await expect(page.getByTestId('public-quote-subtotal')).toHaveText(item.shown[0]!);
 await expect(page.getByTestId('public-quote-setup')).toHaveText(item.shown[1]!);
 await expect(page.getByTestId('public-quote-tax')).toHaveText(item.shown[2]!);
 await expect(page.getByTestId('public-quote-total')).toContainText(item.english?'Total for this purchase':'本次合计');
 await expect(page.getByTestId('public-quote-total')).toContainText(item.shown[3]!);
 await expect(page.getByTestId('public-add-to-cart')).toHaveText(item.english?'Add to cart':'加入购物车');
 if(item.english) await expect(page.getByTestId('public-quote-purchase-cycles')).toHaveText('This purchase includes 3 billing periods');
 await ui.screenshot(templateID+'-quote-details-'+item.currency);
 await page.setViewportSize({width:390,height:844});
 await expect.poll(()=>page.locator('body').evaluate(el=>el.scrollWidth<=window.innerWidth+1)).toBe(true);
 await expect.poll(()=>breakdown.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
 if(item.currency!=='JPY') await expect.poll(()=>page.getByTestId('public-quote-subtotal').evaluate(el=>el.getBoundingClientRect().height<=Number.parseFloat(getComputedStyle(el).lineHeight)+1)).toBe(true);
 expect(writes).toBe(0);await ui.screenshot(templateID+'-quote-details-mobile-'+item.currency);
});

test('TC-1s: 需要客户身份的未确认金额不显示零元报价或费用明细',async({page})=>{
 const view={configurationRevision:'identity-fees',fields:[],values:[]};
 await page.route('**/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
 await page.route('**/purchase-configuration/evaluate',r=>r.fulfill({json:{code:0,data:{...view,valid:true,issues:[]}}}));
 await page.route('**/purchase-configuration/quote',r=>r.fulfill({json:{code:0,data:{...view,currencyCode:'CNY',terms:[{billingTermToken:'identity-term',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'requires_customer_context',subtotalAmount:'0',setupAmount:'0',taxAmount:'0',totalAmount:'0'}]}}}));
 const ui=new ProductPurchasePage(page,origin);await ui.open();await ui.assertDetail();
 await page.getByRole('button',{name:'查看准确价格',exact:true}).click();
 await expect(page.getByRole('button',{name:'登录并继续购买',exact:true})).toBeVisible();
 await expect(page.getByText('登录并完成客户资料核验后才能形成最终价格。',{exact:true})).toBeVisible();
 await expect(page.getByTestId('public-quote-total')).toHaveCount(0);
 await expect(page.getByTestId('public-quote-breakdown')).toHaveCount(0);
 await expect(page.getByTestId('public-add-to-cart')).toHaveCount(0);
 await ui.screenshot(templateID+'-unconfirmed-price-requires-identity');
});


test('TC-1t: 禁用脚本的SSR首屏已请求并展示公开活动，不依靠浏览器修复错误',async({browser})=>{
 publicCampaigns=[{campaignNo:'cmp_11111111111111111111111111111111',campaignType:'discount',nameZhCN:'服务器首屏活动',nameEnUS:'Server-rendered campaign',descriptionZhCN:'当前商品公开活动',descriptionEnUS:'Public product campaign',startsAt:1_792_454_400_000,endsAt:1_823_990_400_000}];
 const context=await browser.newContext({javaScriptEnabled:false});
 try {
  const page=await context.newPage();await page.goto(origin+'/products/11');
  await expect(page.getByRole('heading',{name:'配置购买测试服务器',exact:true})).toBeVisible();
  await expect(page.getByRole('heading',{name:'服务器首屏活动',exact:true})).toBeVisible();
  await expect(page.getByText('活动暂时加载失败，不影响浏览商品。',{exact:true})).toHaveCount(0);
  expect(campaignRequestCount).toBe(1);expect(campaignExposureCount).toBe(0);
  await new ProductPurchasePage(page,origin).screenshot(templateID+'-campaign-ssr-without-script');
  await page.goto(origin+'/en-US/products/11');
  await expect(page.getByRole('heading',{name:'Server-rendered campaign',exact:true})).toBeVisible();
  await expect(page.getByText('Public product campaign',{exact:true})).toBeVisible();
  expect(campaignRequestCount).toBe(2);expect(campaignExposureCount).toBe(0);
 } finally {await context.close();}
});

test('TC-1u: 活动真实读取失败保留商品内容，SSR不伪装为空活动',async({browser})=>{
 campaignResponseFails=true;
 const context=await browser.newContext({javaScriptEnabled:false});
 try {
  const page=await context.newPage();await page.goto(origin+'/products/11');
  await expect(page.getByRole('heading',{name:'配置购买测试服务器',exact:true})).toBeVisible();
  await expect(page.getByText('活动暂时加载失败，不影响浏览商品。',{exact:true})).toBeVisible();
  await expect(page.getByText('当前币种暂无适用于此商品的公开活动',{exact:true})).toHaveCount(0);
  expect(campaignRequestCount).toBe(1);expect(campaignExposureCount).toBe(0);
  await new ProductPurchasePage(page,origin).screenshot(templateID+'-campaign-ssr-genuine-failure');
 } finally {await context.close();}
});

test('TC-1v: 禁用脚本首屏保留组件样式，按钮尺寸与边框不依赖水合', async ({browser}) => {
 const context=await browser.newContext({javaScriptEnabled:false,viewport:{width:1280,height:900}});
 try {
  const page=await context.newPage();const response=await page.goto(origin+'/products/11');
  await expect(page.getByRole('heading',{name:'配置购买测试服务器',exact:true})).toBeVisible();
  await new ProductPurchasePage(page,origin).screenshot(templateID+'-component-styles-ssr-without-script');
  const html=await response!.text();
  expect(html).toMatch(/<style[^>]+data-css-hash=/);
  expect(await page.locator('head style[data-css-hash]').count()).toBeGreaterThan(0);
  const button=page.getByTestId('public-sign-in');
  await expect(button).toBeVisible();
  const style=await button.evaluate(element=>{const css=getComputedStyle(element);return {height:css.height,display:css.display,border:css.borderTopStyle};});
  expect(parseFloat(style.height)).toBeGreaterThanOrEqual(32);
  expect(style.border).toBe('solid');
 } finally {await context.close();}
});


test('TC-1w: SSR样式按请求隔离，两个品牌色和语言不会串用', async ({browser}) => {
 const context=await browser.newContext({javaScriptEnabled:false,viewport:{width:390,height:844}});
 try {
  const page=await context.newPage();
  publicPrimaryColor='#1a2b3c';
  let response=await page.goto(origin+'/products/11');
  let css=Array.from((await response!.text()).matchAll(/<style[^>]*>([\s\S]*?)<\/style>/gu), item=>item[1]).join('');
  expect(css).toContain('#1a2b3c');expect(css).not.toContain('#8b1c2d');
  await expect(page.getByRole('heading',{name:'配置购买测试服务器',exact:true})).toBeVisible();
  publicPrimaryColor='#8b1c2d';
  response=await page.goto(origin+'/en-US/products/11');
  css=Array.from((await response!.text()).matchAll(/<style[^>]*>([\s\S]*?)<\/style>/gu), item=>item[1]).join('');
  expect(css).toContain('#8b1c2d');expect(css).not.toContain('#1a2b3c');
  await expect(page.getByRole('heading',{name:'Purchase test server',exact:true})).toBeVisible();
  await expect(page.getByTestId('public-sign-in')).toHaveText('Sign in');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await new ProductPurchasePage(page,origin).screenshot(templateID+'-component-styles-ssr-mobile-english');
 } finally {publicPrimaryColor='#1677ff';await context.close();}
});

for (const english of [false,true]) test(`TC-1${english?'y':'x'}: ${english?'英文':'中文'}数量清空后由服务端恢复首值，保留零值和合法选择`,async({page})=>{
 const field=(token:string,type:string,required:boolean)=>({fieldToken:token,type,required,visible:true,disabled:false,labelZhCN:token==='optional'?'可选数量':token==='decimal'?'可选小数':'必填数量',labelEnUS:token==='optional'?'Optional quantity':token==='decimal'?'Optional decimal':'Required quantity',descriptionZhCN:'',descriptionEnUS:'',...(type==='integer'?{minimumInteger:required?'1':'0',maximumInteger:'8',quantityStep:'1'}:{minimumDecimal:'0',maximumDecimal:'8'}),...(token==='optional'?{displayControl:'slider'}:{})});
 const defaults:any[]=[{fieldToken:'optional',type:'integer',integerValue:'0'},{fieldToken:'decimal',type:'decimal',decimalValue:'0'},{fieldToken:'required',type:'integer',integerValue:'1'},{fieldToken:'required-decimal',type:'decimal',decimalValue:'0'}];
 const initial=[defaults[0],defaults[1],{fieldToken:'required',type:'integer',integerValue:'2'},{fieldToken:'required-decimal',type:'decimal',decimalValue:'2.5'}];
 const view={configurationRevision:'first-choice-values',fields:[field('optional','integer',false),field('decimal','decimal',false),field('required','integer',true),{...field('required-decimal','decimal',true),labelZhCN:'必填小数',labelEnUS:'Required decimal'},{...field('no-minimum','decimal',false),minimumDecimal:undefined,labelZhCN:'无最小值小数',labelEnUS:'Decimal without minimum'}],values:initial,ready:true};
 let evaluated:any[]=[];let quoted:any[]=[];let pauseNext=false;let releaseEvaluation:(()=>void)|undefined;
 await page.route('**/purchase-configuration',route=>route.fulfill({json:{code:0,data:view}}));
 await page.route('**/purchase-configuration/evaluate',async route=>{
  evaluated=route.request().postDataJSON().values;
  if(pauseNext){pauseNext=false;await new Promise<void>(done=>{releaseEvaluation=done;});}
  const restored=defaults.map(fallback=>evaluated.find(row=>row.fieldToken===fallback.fieldToken)||fallback);
  return route.fulfill({json:{code:0,data:{...view,values:restored,valid:true,issues:[]}}});
 });
 await page.route('**/purchase-configuration/quote',route=>{quoted=route.request().postDataJSON().values;return route.fulfill({json:{code:0,data:{configurationRevision:view.configurationRevision,currencyCode:'CNY',terms:[{billingTermToken:'first-choice-term',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',subtotalAmount:'100',setupAmount:'0',taxAmount:'0',totalAmount:'100'}]}}});});
 await page.goto(origin+(english?'/en-US':'')+'/products/11');
 const form=page.getByTestId('public-product-configurator');
 const optional=form.getByRole('spinbutton',{name:english?'Optional quantity':'可选数量',exact:true}),decimal=form.getByRole('spinbutton',{name:english?'Optional decimal':'可选小数',exact:true}),required=form.getByRole('spinbutton',{name:english?'Required quantity':'必填数量',exact:true});
 const requiredDecimal=form.getByRole('spinbutton',{name:english?'Required decimal':'必填小数',exact:true}),noMinimum=form.getByRole('spinbutton',{name:english?'Decimal without minimum':'无最小值小数',exact:true});
 await expect(form).toContainText(english?'Choose configuration':'选择配置');
 await expect(form.getByTestId('purchase-plan-default')).toHaveCount(0);await expect(form).not.toContainText(english?'Use plan defaults':'使用套餐默认配置');
 await expect(optional).not.toHaveAttribute('placeholder',/套餐默认|plan defaults/);await expect(decimal).not.toHaveAttribute('placeholder',/套餐默认|plan defaults/);
 await expect(optional).toHaveValue('0');await expect(decimal).toHaveValue('0');await expect(required).toHaveValue('2');await expect(requiredDecimal).toHaveValue('2.5');await expect(noMinimum).toHaveValue('');
 await new ProductPurchasePage(page,origin).screenshot(templateID+'-first-choices-initial-'+(english?'english':'chinese'));
 await optional.fill('4');await decimal.fill('0');
 await expect.poll(()=>evaluated).toEqual(expect.arrayContaining([{fieldToken:'optional',type:'integer',integerValue:'4'},{fieldToken:'decimal',type:'decimal',decimalValue:'0'}]));
 const quote=page.getByRole('button',{name:english?/Check exact price/:/查看准确价格/});await expect(quote).toBeEnabled();await quote.click();
 expect(quoted).toEqual(expect.arrayContaining([{fieldToken:'optional',type:'integer',integerValue:'4'},{fieldToken:'decimal',type:'decimal',decimalValue:'0'}]));
 await optional.fill('0');await expect.poll(()=>evaluated).toContainEqual({fieldToken:'optional',type:'integer',integerValue:'0'});await expect(optional).toHaveValue('0');
 await optional.fill('');await decimal.fill('');
 await expect.poll(()=>evaluated.some(value=>value.fieldToken==='decimal')).toBe(false);
 await expect(optional).toHaveValue('0');await expect(decimal).toHaveValue('0');await expect(required).toHaveValue('2');
 await expect(quote).toBeEnabled();await quote.click();expect(quoted).toEqual(initial);
 pauseNext=true;await required.fill('');await requiredDecimal.fill('');await expect(quote).toBeDisabled();
 await expect.poll(()=>!!releaseEvaluation).toBe(true);expect(evaluated.some(value=>['required','required-decimal'].includes(value.fieldToken))).toBe(false);releaseEvaluation!();
 await expect(required).toHaveValue('1');await expect(requiredDecimal).toHaveValue('0');await expect(noMinimum).toHaveValue('');await expect(quote).toBeEnabled();
 await quote.click();expect(quoted).toEqual(defaults);
 await expect(form.getByTestId('purchase-plan-default')).toHaveCount(0);
 await page.setViewportSize({width:390,height:844});await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await new ProductPurchasePage(page,origin).screenshot(templateID+'-first-choices-'+(english?'english':'chinese'));
});

test('TC-1z: 上游选项尚未同步时三秒内显示本地表单，后台刷新后保留已填配置',async({page})=>{
 test.setTimeout(25000);
 const fields:any[]=[{fieldToken:'host',type:'single_line',labelZhCN:'主机名称',labelEnUS:'Hostname',visible:true,required:true,disabled:false},{fieldToken:'os',type:'single_select',labelZhCN:'操作系统',labelEnUS:'Operating system',displayControl:'select',visible:true,required:true,disabled:false,options:[]}];
 let evaluations=0;let seenHost='';
 await page.route('**/public/catalog/products/11/purchase-configuration',r=>r.fulfill({json:{code:0,data:{ready:false,configurationRevision:'option-projection',fields,values:[]}}}));
 await page.route('**/purchase-configuration/evaluate',r=>{
  evaluations++;const input=r.request().postDataJSON();seenHost=input.values.find((v:any)=>v.fieldToken==='host')?.textValue||'';
  const available=evaluations>=2;const next=fields.map(field=>field.fieldToken==='os'?{...field,options:available?[{optionToken:'actual-os',labelZhCN:'Debian',labelEnUS:'Debian'}]:[]}:field);
  return r.fulfill({json:{code:0,data:{configurationRevision:'option-projection',fields:next,values:input.values,valid:available&&input.values.some((v:any)=>v.fieldToken==='os'&&v.optionTokens?.length),issues:[]}}});
 });
 const ui=new ProductPurchasePage(page,origin);const start=Date.now();await ui.open();await expect(page.getByRole('textbox',{name:'主机名称',exact:true})).toBeVisible();expect(Date.now()-start).toBeLessThan(3000);
 await expect(page.getByTestId('configuration-options-pending')).toContainText('上游可选内容尚未就绪。可以继续填写其他配置，稍后重新读取。');
 await ui.fillHost('keep-host');await ui.screenshot('local-form-options-pending');
 await expect(page.getByTestId('configuration-options-pending')).toHaveCount(0,{timeout:9000});await expect(page.getByRole('textbox',{name:'主机名称',exact:true})).toHaveValue('keep-host');expect(seenHost).toBe('keep-host');
 await ui.chooseOS('Debian');await expect.poll(()=>evaluations).toBeGreaterThan(2);await ui.screenshot('local-options-recovered');
});
