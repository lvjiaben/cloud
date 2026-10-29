import { test, expect } from '@host-tests/fixtures/auth';
import { createServer } from 'node:http';
import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { ProductPurchasePage } from '../pages/ProductPurchasePage';
const templateID=process.env.E2E_WEBSITE_TEMPLATE||'website-default';
if(!['website-default','website-minimal'].includes(templateID))throw new Error('Unsupported website fixture');
const pluginRoot=resolve(fileURLToPath(new URL('../../../../',import.meta.url)),templateID);
let origin='',renderer:ChildProcess;
const product={publicSlug:'11',nameZhCN:'配置购买测试服务器',nameEnUS:'Purchase test server',summaryZhCN:'配置测试',summaryEnUS:'Configuration test',categoryNameZhCN:'云服务器',categoryNameEnUS:'Cloud servers',currencies:['CNY'],availability:'available'};
const api=createServer((req,res)=>{const path=new URL(req.url!,'http://local').pathname;let data:unknown;if(path.endsWith('/public/brand'))data={brand:{primaryColor:'#1677ff',siteName:{zhCN:'云服务平台',enUS:'Cloud services'},footer:{zhCN:'服务支持',enUS:'Support'},navigation:[],footerNavigation:[]}};else if(path.endsWith('/campaigns'))data={campaigns:[]};else if(path.endsWith('/products/11'))data={product};else if(path.endsWith('/products'))data={products:[product],categories:[],availableCurrencies:['CNY'],nextCursor:''};else{res.statusCode=404;res.end('{}');return;}res.setHeader('content-type','application/json');res.end(JSON.stringify({code:0,data}));});
test.beforeAll(async()=>{api.listen(0,'127.0.0.1');await once(api,'listening');const probe=createServer();probe.listen(0,'127.0.0.1');await once(probe,'listening');const port=(probe.address() as any).port;await new Promise<void>(done=>probe.close(()=>done()));origin=`http://127.0.0.1:${port}`;renderer=spawn(process.execPath,[resolve(pluginRoot,'frontend/site/.output/server/index.mjs')],{env:{...process.env,PORT:String(port),HOST:'127.0.0.1',NUXT_API_ORIGIN:`http://127.0.0.1:${(api.address() as any).port}`},stdio:'ignore'});await expect.poll(async()=>{try{return(await fetch(origin+'/__site/health')).status;}catch{return 0;}},{timeout:20000}).toBe(200);});
test.afterAll(async()=>{renderer?.kill();api.closeAllConnections();await new Promise<void>(done=>api.close(()=>done()));});
test.beforeEach(async({page})=>{await page.route('**/__website/configuration',route=>route.fulfill({json:{origin,customerOrigin:origin}}));await page.context().route('**/auth/website-session?*',route=>{const nonce=new URL(route.request().url()).searchParams.get('nonce');return route.fulfill({contentType:'text/html',body:`<script>parent.postMessage({type:'lina-website-session',nonce:${JSON.stringify(nonce)},accessToken:'fixture-session'},location.origin)</script>`});});});
const option=(optionToken:string,label:string)=>({optionToken,labelZhCN:label,labelEnUS:label});
const field=(fieldToken:string,zh:string,en:string,options:any[],presentation?:string)=>({fieldToken,type:'single_select',labelZhCN:zh,labelEnUS:en,descriptionZhCN:'',descriptionEnUS:'',visible:true,required:true,disabled:false,options,displayControl:'select',presentation});
const fields=[field('opaque.os','操作系统','Operating system',[option('token.os.a','Ubuntu 22.04'),option('token.os.b','Ubuntu 24.04'),option('token.os.c','Windows Server 2022'),option('token.os.d','Custom appliance'),option('token.os.tencent','TencentOS Server 3.2'),option('token.os.centos-dash','CentOS-7'),option('token.os.centos-digit','CentOS7')],'operating_system'),field('opaque.region','区域','Region',[option('token.region.a','香港一区'),option('token.region.b','Tokyo 02')],'region'),{fieldToken:'opaque.disk',type:'integer',labelZhCN:'数据盘',labelEnUS:'Data disk',descriptionZhCN:'',descriptionEnUS:'',visible:true,required:true,disabled:false,minimumInteger:'10',maximumInteger:'65',quantityStep:'10',unit:'GiB',displayControl:'auto'},field('opaque.unknown','线路','Network',[option('line.a','Premium'),option('line.b','Standard')],'unknown')];
const initial=[{fieldToken:'opaque.os',type:'single_select',optionTokens:['token.os.a']},{fieldToken:'opaque.region',type:'single_select',optionTokens:['token.region.a']},{fieldToken:'opaque.disk',type:'integer',integerValue:'10'},{fieldToken:'opaque.unknown',type:'single_select',optionTokens:['line.a']}];
test.describe('TC-6 官网专用配置与主机登录',()=>{
 for(const english of [false,true])test(`TC-6${english?'b':'a'}: ${english?'英文':'中文'}系统按家族选择版本、区域图标、数量步长同步及报价保留精确令牌`,async({page})=>{
  let seen:any[]=[],fail=false;
  const view={configurationRevision:'special-controls',fields,values:initial,ready:true};
  await page.route('**/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
  await page.route('**/purchase-configuration/evaluate',r=>{seen=r.request().postDataJSON().values;return fail?r.fulfill({status:503,json:{code:50}}):r.fulfill({json:{code:0,data:{...view,values:seen,valid:true,issues:[]}}});});
  let quoted:any[]=[];await page.route('**/purchase-configuration/quote',r=>{quoted=r.request().postDataJSON().values;return r.fulfill({json:{code:0,data:{configurationRevision:view.configurationRevision,currencyCode:'CNY',terms:[{billingTermToken:'monthly',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',subtotalAmount:'100',setupAmount:'0',taxAmount:'0',totalAmount:'100'}]}}});});
  const ui=new ProductPurchasePage(page,origin);await ui.open('11',english);
  await expect(page.getByTestId('purchase-os-family-ubuntu')).toHaveAttribute('aria-pressed','true');await ui.screenshot(templateID+'-special-controls-initial');
  await expect(page.getByTestId('purchase-os-family-other')).toContainText(english?'Other':'其他系统');
  await expect(page.locator('.ant-select').filter({has:page.locator('#purchase-opaque\\.unknown')})).toBeVisible();
  await expect(page.locator('.os-families button').first()).toHaveAttribute('data-testid','purchase-os-family-ubuntu');await ui.chooseVersion('Ubuntu 24.04',english);await ui.chooseFamily('ubuntu');await ui.chooseRegion('Tokyo 02');await ui.changeQuantity(english?'Data disk':'数据盘','40');await ui.increaseQuantity(english?'Data disk':'数据盘',english);
  await expect(page.getByRole('spinbutton',{name:english?'Data disk':'数据盘',exact:true})).toHaveValue('50');
  await expect.poll(()=>seen).toEqual(expect.arrayContaining([{fieldToken:'opaque.os',type:'single_select',optionTokens:['token.os.b']},{fieldToken:'opaque.region',type:'single_select',optionTokens:['token.region.b']},{fieldToken:'opaque.disk',type:'integer',integerValue:'50'}]));
  await expect(page.getByTestId('purchase-slider-opaque.disk')).toContainText('60 GiB');
  await ui.checkPrice(english);await expect(page.getByTestId('public-add-to-cart')).toBeVisible();expect(quoted).toEqual(seen);
  await ui.screenshot(templateID+'-special-controls-'+(english?'en':'zh'));
  await page.setViewportSize({width:390,height:844});await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await ui.screenshot(templateID+'-special-controls-mobile-'+(english?'en':'zh'));
  fail=true;await ui.chooseFamily('windows');await expect(page.getByTestId('configuration-evaluation-error')).toContainText(english?'Product configuration is temporarily unavailable. Please try again.':'商品配置暂时不可用，请稍后重试。');
  await expect(page.getByTestId('purchase-os-family-windows')).toHaveAttribute('aria-pressed','true');await expect(page.getByTestId('public-add-to-cart')).toHaveCount(0);await ui.screenshot(templateID+'-special-controls-failure');
 });
 test('TC-6c: 主机名与密码只走登录凭据接口，不进入选配草稿或报价并在加购后清除',async({page})=>{
  const policy={hostnameEnabled:true,passwordEnabled:true,hostname:{length:12,characters:['lowercase','digits']},password:{length:12,characters:['lowercase','uppercase','digits']}};
  const view={configurationRevision:'credentials',fields:[],values:[],ready:true,serverLoginPolicy:policy};let saved:any,added:any,additions=0,rejectLogin=true;
  await page.route('**/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
  await page.route('**/purchase-configuration/quote',r=>{expect(JSON.stringify(r.request().postDataJSON())).not.toContain('PrivateAbc123');return r.fulfill({json:{code:0,data:{configurationRevision:'credentials',currencyCode:'CNY',terms:[{billingTermToken:'monthly',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',subtotalAmount:'100',setupAmount:'0',taxAmount:'0',totalAmount:'100'}]}}});});
  await page.route('**/purchase-configuration/server-login',r=>{expect(new URL(r.request().url()).pathname).toBe('/x/idc-core/api/v1/account/catalog/products/11/purchase-configuration/server-login');saved=r.request().postDataJSON();if(rejectLogin){rejectLogin=false;return r.fulfill({status:503,json:{code:50}});}return r.fulfill({json:{code:0,data:{serverLoginToken:'opaque-login-token',hostname:saved.hostname,passwordConfigured:true}}});});
  await page.route('**/account/commerce/guest-cart-merge',r=>r.fulfill({json:{code:0,data:{merged:true}}}));
  await page.route('**/account/commerce/cart-items',r=>{added=r.request().postDataJSON();additions++;return r.fulfill({json:{code:0,data:{cart:{}}}});});
  await page.route('**/account/commerce/cart',r=>r.fulfill({json:{code:0,data:{cart:{cartNo:'cart',currencyCode:'CNY',version:1+additions,items:additions?[{cartItemNo:'item',configurationReference:'reference',productNameZhCN:'配置购买测试服务器',publicSlug:'11',quantity:1,purchaseCycles:1,version:1}]:[]}}}}));
  const ui=new ProductPurchasePage(page,origin);await ui.open();await expect(page.getByTestId('purchase-server-login')).toContainText('允许自行填写，留空时自动生成。');await ui.fillServerLogin('my-host','PrivateAbc123');await ui.checkPrice();await ui.addToCart();
  await expect(page.getByText('商品没有加入购物车，请保留当前配置后重试。',{exact:true})).toBeVisible();expect(additions).toBe(0);await expect(page.getByTestId('purchase-server-login').getByLabel('主机密码',{exact:true})).toHaveValue('PrivateAbc123');await ui.screenshot(templateID+'-credential-save-failed');await ui.addToCart();
  await expect(page.getByTestId('website-cart-item-item')).toBeVisible();expect(saved).toEqual({configurationRevision:'credentials',hostname:'my-host',password:'PrivateAbc123'});expect(added.serverLoginToken).toBe('opaque-login-token');expect(added.values).toEqual([]);expect(JSON.stringify(added)).not.toContain('PrivateAbc123');
  const storage=await page.evaluate(()=>JSON.stringify({local:{...localStorage},session:{...sessionStorage}}));expect(storage).not.toContain('PrivateAbc123');expect(storage).not.toContain('opaque-login-token');
 });
 test('TC-6d: 登录恢复主机名与选配，密码要求重填且不会自动加购',async({page})=>{
  await page.context().route('**/auth/website-session?*',route=>{const url=new URL(route.request().url());const body=url.searchParams.get('login')==='1'?`<script>localStorage.setItem('fixture.signedIn','1');location.replace(${JSON.stringify(origin)}+${JSON.stringify(url.searchParams.get('returnTo'))})</script>`:`<script>parent.postMessage({type:'lina-website-session',nonce:${JSON.stringify(url.searchParams.get('nonce'))},accessToken:localStorage.getItem('fixture.signedIn')==='1'?'fixture-session':''},location.origin)</script>`;return route.fulfill({contentType:'text/html',body});});
  const view={configurationRevision:'login-recovery',fields:[],values:[],ready:true,serverLoginPolicy:{hostnameEnabled:true,passwordEnabled:true}};
  await page.route('**/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
  await page.route('**/purchase-configuration/evaluate',r=>r.fulfill({json:{code:0,data:{...view,valid:true,issues:[]}}}));
  await page.route('**/purchase-configuration/quote',r=>r.fulfill({json:{code:0,data:{configurationRevision:view.configurationRevision,currencyCode:'CNY',terms:[{billingTermToken:'monthly',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',subtotalAmount:'100',setupAmount:'0',taxAmount:'0',totalAmount:'100'}]}}}));
  let additions=0;await page.route('**/commerce/cart-items',r=>{additions++;return r.fulfill({status:500});});
  const ui=new ProductPurchasePage(page,origin);await ui.open();await ui.fillServerLogin('keep-host','PrivateAbc123');await ui.checkPrice();await ui.addToCart();
  await expect(page.getByText('登录后请重新填写主机密码，或留空自动生成。',{exact:true})).toBeVisible();
  await expect(page.getByTestId('purchase-server-login').getByRole('textbox',{name:'主机名',exact:true})).toHaveValue('keep-host');
  await expect(page.getByTestId('purchase-server-login').getByLabel('主机密码',{exact:true})).toHaveValue('');
  expect(additions).toBe(0);expect(await page.evaluate(()=>JSON.stringify({...localStorage,...sessionStorage}))).not.toContain('PrivateAbc123');
  await ui.screenshot(templateID+'-login-password-reentry');
 });
 for(const change of [false,true])test(`TC-6${change?'f':'e'}: 编辑已有主机登录${change?'修改主机名时继承旧密码':'不改动时继承原令牌'}`,async({page})=>{
  let frozen=false,prepared:any,submitted:any;
  const quote=()=>({quoteNo:'quote',cartVersion:2,currencyCode:'CNY',subtotalMinor:'100',discountMinor:'0',taxAmountMinor:'0',totalAmountMinor:'100',expiresAt:Date.now()+60000,items:[{cartItemNo:'item',quoteItemNo:'quote-item',publicSlug:'11',quantity:1,purchaseCycles:1,configurationRequired:true,configurationFrozen:frozen,totalAmountMinor:'100'}]});
  const cart=()=>({cartNo:'cart',version:2,currencyCode:'CNY',items:[{cartItemNo:'item',configurationReference:'no-local-draft',publicSlug:'11',productNameZhCN:'配置购买测试服务器',quantity:1,purchaseCycles:1,version:1,hasServerLogin:true,serverLoginToken:'previous-token',serverLoginHostname:'old-host',serverLoginPasswordConfigured:true}],latestQuote:quote()});
  await page.route('**/account/commerce/guest-cart-merge',r=>r.fulfill({json:{code:0,data:{merged:true}}}));await page.route('**/account/commerce/cart',r=>r.fulfill({json:{code:0,data:{cart:cart()}}}));await page.route('**/account/commerce/quotes',r=>r.fulfill({json:{code:0,data:{quote:quote()}}}));
  const view={configurationRevision:'edit-policy',fields:[],values:[],ready:true,serverLoginPolicy:{hostnameEnabled:true,passwordEnabled:true}};
  await page.route('**/products/11/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
  await page.route('**/purchase-configuration/server-login',r=>{prepared=r.request().postDataJSON();return r.fulfill({json:{code:0,data:{serverLoginToken:'replacement-token',hostname:'new-host',passwordConfigured:true}}});});
  await page.route('**/quote-items/quote-item/purchase-configuration',r=>{submitted=r.request().postDataJSON();frozen=true;return r.fulfill({json:{code:0,data:{}}});});
  const ui=new ProductPurchasePage(page,origin);await ui.openCheckout();await ui.completeConfiguration();await expect(page.getByTestId('purchase-server-login')).toContainText('主机名可修改，密码留空保留已设置的密码。');
  await expect(page.getByTestId('purchase-server-login').getByRole('textbox',{name:'主机名',exact:true})).toHaveValue('old-host');if(change)await ui.fillHostname('new-host');await ui.saveConfiguration();await expect(page.getByTestId('purchase-server-login')).toHaveCount(0);
  if(change){expect(prepared).toEqual({configurationRevision:'edit-policy',previousServerLoginToken:'previous-token',hostname:'new-host'});expect(submitted.serverLoginToken).toBe('replacement-token');}else{expect(prepared).toBeUndefined();expect(submitted.serverLoginToken).toBeUndefined();}
 });

 test('TC-6g: 凭据写入成功后加购失败，清空密码并复用同一令牌重试',async({page})=>{
  const view={configurationRevision:'credential-retry',fields:[],values:[],ready:true,serverLoginPolicy:{hostnameEnabled:true,passwordEnabled:true}};let prepared=0,attempts=0;const tokens:string[]=[];
  await page.route('**/products/11/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
  await page.route('**/purchase-configuration/quote',r=>r.fulfill({json:{code:0,data:{configurationRevision:view.configurationRevision,currencyCode:'CNY',terms:[{billingTermToken:'monthly',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',subtotalAmount:'100',setupAmount:'0',taxAmount:'0',totalAmount:'100'}]}}}));
  await page.route('**/purchase-configuration/server-login',r=>{prepared++;return r.fulfill({json:{code:0,data:{serverLoginToken:'retry-same-token',hostname:'retry-host',passwordConfigured:true}}});});
  await page.route('**/account/commerce/guest-cart-merge',r=>r.fulfill({json:{code:0,data:{merged:true}}}));
  await page.route('**/account/commerce/cart',r=>r.fulfill({json:{code:0,data:{cart:{cartNo:'cart',version:attempts>=2?2:1,currencyCode:'CNY',items:attempts>=2?[{cartItemNo:'retry-item',configurationReference:'retry-ref',productNameZhCN:'配置购买测试服务器',publicSlug:'11',quantity:1,purchaseCycles:1,version:1}]:[]}}}}));
  await page.route('**/account/commerce/cart-items',r=>{tokens.push(r.request().postDataJSON().serverLoginToken);attempts++;return attempts===1?r.fulfill({status:503,json:{code:50}}):r.fulfill({json:{code:0,data:{}}});});
  const ui=new ProductPurchasePage(page,origin);await ui.open();await ui.fillServerLogin('retry-host','PrivateAbc123');await ui.checkPrice();await ui.addToCart();
  await expect(page.getByText('商品没有加入购物车，请保留当前配置后重试。',{exact:true})).toBeVisible();await expect(page.getByTestId('purchase-server-login').getByLabel('主机密码',{exact:true})).toHaveValue('');await expect(page.getByTestId('purchase-server-login')).toContainText('主机名可修改，密码留空保留已设置的密码。');await ui.screenshot(templateID+'-cart-retry-keeps-credential-token');
  await ui.addToCart();await expect(page.getByTestId('website-cart-item-retry-item')).toBeVisible();expect(prepared).toBe(1);expect(tokens).toEqual(['retry-same-token','retry-same-token']);
 });

 test('TC-6h: 多台自动生成项目补全时首次自填提示分别配置，不写凭据或改台数',async({page})=>{
  let loginWrites=0,configurationWrites=0,cartWrites=0;
  const item={cartItemNo:'multi-item',configurationReference:'no-local-draft',publicSlug:'11',productNameZhCN:'配置购买测试服务器',quantity:3,purchaseCycles:1,version:1,hasServerLogin:false};
  const quote={quoteNo:'multi-quote',cartVersion:2,currencyCode:'CNY',subtotalMinor:'300',discountMinor:'0',taxAmountMinor:'0',totalAmountMinor:'300',expiresAt:Date.now()+60000,items:[{...item,quoteItemNo:'multi-quote-item',configurationRequired:true,configurationFrozen:false,totalAmountMinor:'300'}]};
  const cart={cartNo:'multi-cart',version:2,currencyCode:'CNY',items:[item],latestQuote:quote};
  await page.route('**/account/commerce/guest-cart-merge',r=>r.fulfill({json:{code:0,data:{merged:true}}}));await page.route('**/account/commerce/cart',r=>r.fulfill({json:{code:0,data:{cart}}}));await page.route('**/account/commerce/quotes',r=>r.fulfill({json:{code:0,data:{quote}}}));
  const view={configurationRevision:'multi-policy',fields:[],values:[],ready:true,serverLoginPolicy:{hostnameEnabled:true,passwordEnabled:true}};
  await page.route('**/products/11/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
  await page.route('**/purchase-configuration/server-login',r=>{loginWrites++;return r.fulfill({status:500});});
  await page.route('**/quote-items/multi-quote-item/purchase-configuration',r=>{configurationWrites++;return r.fulfill({status:500});});
  await page.route('**/account/commerce/cart-items**',r=>{cartWrites++;return r.fulfill({status:500});});
  const ui=new ProductPurchasePage(page,origin);await ui.openCheckout();await ui.completeConfiguration();await ui.fillServerLogin('one-host','PrivateAbc123');await ui.saveConfiguration();
  await expect(page.getByTestId('purchase-server-login-issue')).toHaveText('此项包含多台服务器，请分别配置后加入购物车');await expect(page.getByTestId('website-cart-item-multi-item')).toContainText('数量：3');
  await expect(page.getByTestId('purchase-server-login').getByRole('textbox',{name:'主机名',exact:true})).toHaveValue('one-host');expect(loginWrites).toBe(0);expect(configurationWrites).toBe(0);expect(cartWrites).toBe(0);await ui.screenshot(templateID+'-multiple-servers-require-separate-configuration');
 });

 test('TC-6i: CentOS版本边界分类不误判TencentOS，系统选择保留原始令牌',async({page})=>{
  const view={configurationRevision:'os-name-boundaries',fields,values:initial,ready:true};let evaluated:any[]=[],quoted:any[]=[];
  await page.route('**/products/11/purchase-configuration',r=>r.fulfill({json:{code:0,data:view}}));
  await page.route('**/purchase-configuration/evaluate',r=>{evaluated=r.request().postDataJSON().values;return r.fulfill({json:{code:0,data:{...view,values:evaluated,valid:true,issues:[]}}});});
  await page.route('**/purchase-configuration/quote',r=>{quoted=r.request().postDataJSON().values;return r.fulfill({json:{code:0,data:{configurationRevision:view.configurationRevision,currencyCode:'CNY',terms:[{billingTermToken:'monthly',labelZhCN:'每月',labelEnUS:'Monthly',minimumPurchaseCycles:1,status:'final',subtotalAmount:'100',setupAmount:'0',taxAmount:'0',totalAmount:'100'}]}}});});
  const ui=new ProductPurchasePage(page,origin);await ui.open();await ui.chooseFamily('centos');
  await expect.poll(()=>evaluated).toContainEqual({fieldToken:'opaque.os',type:'single_select',optionTokens:['token.os.centos-dash']});await ui.chooseVersion('CentOS7');
  await expect.poll(()=>evaluated).toContainEqual({fieldToken:'opaque.os',type:'single_select',optionTokens:['token.os.centos-digit']});
  await ui.chooseFamily('other');await ui.chooseVersion('TencentOS Server 3.2');
  await expect(page.getByTestId('purchase-os-family-other')).toHaveAttribute('aria-pressed','true');await expect(page.getByTestId('purchase-os-family-centos')).toHaveAttribute('aria-pressed','false');await expect.poll(()=>evaluated).toContainEqual({fieldToken:'opaque.os',type:'single_select',optionTokens:['token.os.tencent']});
  await ui.checkPrice();await expect(page.getByTestId('public-add-to-cart')).toBeVisible();expect(quoted).toContainEqual({fieldToken:'opaque.os',type:'single_select',optionTokens:['token.os.tencent']});await ui.screenshot(templateID+'-tencent-os-remains-other-system');
 });

});
