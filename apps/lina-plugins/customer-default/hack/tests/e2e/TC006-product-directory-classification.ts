// Customer directory UI regressions use isolated HTTP fixtures. PostgreSQL tests
// in idc-core prove the same type filter, counts and authenticated ownership.
import type { Page } from '@host-tests/support/playwright';
import { test, expect } from '@host-tests/fixtures/auth';
import { LoginPage } from '@host-tests/pages/LoginPage';
import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
const origin=process.env.CUSTOMER_E2E_BASE_URL || 'http://127.0.0.1:6032';
async function fixture(page:Page,productCount=2,serviceProgress=false) {
 const state={failProduct:false,manualProcessing:true,queries:[] as URL[],services:[] as string[]};
 const system={directoryNo:'dir_system_orders',navigationType:'system',targetUrl:'/account/orders',icon:'lucide:list',nameZhCn:'订单入口',nameEnUs:'Order entry',sortOrder:1,serviceCount:0};
 const products=Array.from({length:productCount},(_,index)=>({directoryNo:`dir_product_${index+1}`,navigationType:'product',targetUrl:'',icon:'lucide:server',nameZhCn:index===0?'云服务器':index===1?'证书服务':`产品目录${index+1}`,nameEnUs:index===0?'Cloud servers':index===1?'Certificates':`Product directory ${index+1}`,sortOrder:index+2,serviceCount:serviceProgress?(index===0?2:3):1}));
 const all=[system,...products,{...system,directoryNo:'dir_help',navigationType:'custom',targetUrl:'https://example.com/help',nameZhCn:'帮助外链',nameEnUs:'External help',sortOrder:1000}];
 await page.route('**/api/v1/auth/login',r=>r.fulfill({json:{code:0,data:{accessToken:'fixture-directory-session',refreshToken:'fixture-directory-refresh',tenants:[]}}}));
 await page.route('**/api/v1/user/info',r=>r.fulfill({json:{code:0,data:{id:706,username:'directory-fixture',realName:'目录测试客户',roles:['user'],permissions:[],menus:[],homePath:'/account/product-directories'}}}));
 await page.route('**/api/v1/plugins/dynamic',r=>r.fulfill({json:{code:0,data:{list:[]}}}));
 await page.route('**/customer/external-menus',r=>r.fulfill({json:{code:0,data:{items:[]}}}));
 await page.route('**/account/initial-password/status',r=>r.fulfill({json:{code:0,data:{required:false}}}));
 await page.route(/\/x\/idc-core\/api\/v1\/account\/product-directories(?:\?|$)/,r=>{
  const url=new URL(r.request().url());state.queries.push(url);
  const productOnly=url.searchParams.get('navigationType')==='product';
  if(productOnly && state.failProduct)return r.fulfill({status:503,json:{code:50,message:'Product classification temporarily unavailable'}});
  const selected=productOnly?products:all;const current=Number(url.searchParams.get('page')||1),size=Number(url.searchParams.get('pageSize')||100);
  return r.fulfill({json:{code:0,data:{items:selected.slice((current-1)*size,current*size),total:selected.length,page:current,pageSize:size}}});
 });
 await page.route('**/account/product-directories/*/services?*',r=>{
  const directory=new URL(r.request().url()).pathname.split('/').at(-2)!;state.services.push(directory);
  if(directory!=='all' && !products.some(item=>item.directoryNo===directory))return r.fulfill({status:403,json:{code:403,message:'Non-product menus cannot filter services'}});
  let rows=(directory==='all'?products.slice(0,2):products.filter(item=>item.directoryNo===directory)).map((item,index)=>({entitlementNo:`ent_${item.directoryNo}`,productNo:`product_${item.directoryNo}`,productNameZhCn:`已购${item.nameZhCn}`,productNameEnUs:`Purchased ${item.nameEnUs}`,pluginId:'fixture-service',statusCode:'active',requiresManualProcessing:false,startedAt:1792494724000,expiresAt:null,orderNo:`order_${index}`}));
  if(serviceProgress) rows=[
   {...rows[0]!,entitlementNo:'ent_manual',productNameZhCn:'人工处理服务器',productNameEnUs:'Server requiring attention',statusCode:'pending_activation',requiresManualProcessing:state.manualProcessing,startedAt:0,orderNo:'order_manual'},
   {...rows[0]!,entitlementNo:'ent_queued',productNameZhCn:'等待开通服务器',productNameEnUs:'Server awaiting activation',statusCode:'pending_activation',startedAt:0,orderNo:'order_queued'},
   {...rows[0]!,entitlementNo:'ent_active',productNo:'product_dir_product_2',productNameZhCn:'现有运行服务器',productNameEnUs:'Existing active server',requiresManualProcessing:true,orderNo:'order_active'},
   {...rows[0]!,entitlementNo:'ent_incomplete',productNo:'product_dir_product_2',productNameZhCn:'处理未完成服务',productNameEnUs:'Uncompleted service',statusCode:'pending_activation',requiresManualProcessing:state.manualProcessing,startedAt:0,orderNo:'order_incomplete'},
   {...rows[0]!,entitlementNo:'ent_unknown',productNo:'product_dir_product_2',productNameZhCn:'历史服务',productNameEnUs:'Historical service',statusCode:'private_unrecognized_status',orderNo:'order_unknown'},
  ];
  if(serviceProgress && directory!=='all') rows=rows.filter(item=>item.productNo===`product_${directory}`);
  return r.fulfill({json:{code:0,data:{items:rows,total:rows.length,page:1,pageSize:20}}});
 });
 await page.route('**/account/commerce/orders?*',r=>r.fulfill({json:{code:0,data:{orders:[],nextCursor:''}}}));
 await page.route('**/account/commerce/orders/order_manual',r=>r.fulfill({json:{code:0,data:{order:{summary:{orderNo:'order_manual',currencyCode:'CNY',statusCode:'paid',totalAmountMinor:'1100',createdAt:1792494724000,canCancel:false,canPreparePayment:false},items:[],deliveries:[{productNameZhCN:'人工处理服务器',productNameEnUS:'Server requiring attention',quantity:1,statusCode:'manual_processing',ongoingService:true,updatedAt:1792494724000}],serviceEntitlements:[],reviews:[]}}}}));
 return state;
}
async function login(page:Page) {await page.goto(origin+'/auth/login');await new LoginPage(page).login('directory-fixture','fixture-only-password');await page.getByTestId('customer-product-directories').waitFor();}
async function screenshot(page:Page,name:string) {
 const now=new Date().toISOString();const dir=fileURLToPath(new URL(`../../../../../../temp/${now.slice(0,10).replaceAll('-','')}/`,import.meta.url));await mkdir(dir,{recursive:true});
 await page.screenshot({path:resolve(dir,`${now.slice(11,23).replace(/[:.]/g,'')}-customer-directories-${name}.png`),animations:'disabled'});
}

test.describe('TC-6 服务分类与会员导航类型',()=>{
 test('TC-6d: 人工处理阶段保留生命周期与订单入口，刷新反映新事实且不展示私有状态',async({page})=>{
  const state=await fixture(page,2,true);
  await login(page);
  const view=page.getByTestId('customer-product-directories');
  const rows=view.locator('.vxe-table--main-wrapper .vxe-body--row');
  const manual=rows.filter({hasText:'人工处理服务器'});
  const active=rows.filter({hasText:'现有运行服务器'});
  await expect(manual).toContainText('待人工处理');
  await expect(manual).toContainText('未开始');
  await expect(rows.filter({hasText:'处理未完成服务'})).toContainText('待人工处理');
  await expect(active).toContainText('运行中');
  await expect(active).not.toContainText('待人工处理');
  await expect(rows.filter({hasText:'等待开通服务器'})).toContainText('待激活');
  await expect(rows.filter({hasText:'历史服务'})).toContainText('状态暂不可用');
  await expect(view).not.toContainText('private_unrecognized_status');
  await expect(view).not.toContainText('productDirectories.states.');
  await screenshot(page,'manual-service-progress');
  await manual.getByRole('button',{name:'查看订单',exact:true}).click();
  await expect(page).toHaveURL(/orders\/order_manual$/);
  await expect(page.getByTestId('commerce-deliveries')).toContainText('待人工处理');
  await page.goBack();
  await expect(view).toBeVisible();
  await new LoginPage(page).switchLanguage('English');
  // Reload verifies persisted language after the layout's asynchronous route refresh.
  await page.reload();
  await expect(view).toContainText('Server requiring attention');
  await expect(rows.filter({hasText:'Server requiring attention'})).toContainText('Needs staff attention');
  await expect(rows.filter({hasText:'Existing active server'})).toContainText('Active');
  await expect(rows.filter({hasText:'Uncompleted service'})).toContainText('Needs staff attention');
  await expect(rows.filter({hasText:'Historical service'})).toContainText('Status unavailable');
  await screenshot(page,'manual-service-progress-en');
  const readsBeforeReentry=state.services.length;
  state.manualProcessing=false;
  await rows.filter({hasText:'Server requiring attention'}).getByRole('button',{name:'View order',exact:true}).click();
  await expect(page.getByTestId('commerce-deliveries')).toBeVisible();
  await page.goBack();
  await expect.poll(()=>state.services.length).toBeGreaterThan(readsBeforeReentry);
  await expect(rows.filter({hasText:'Server requiring attention'})).toContainText('Pending activation');
  await expect(view).not.toContainText('Needs staff attention');
 });
 test('TC-6a: 只列产品分类，侧栏保留系统外链，点击与刷新保持正确服务',async({page})=>{
  const state=await fixture(page);await login(page);const view=page.getByTestId('customer-product-directories');
  await expect(view.getByRole('button',{name:'云服务器 (1)',exact:true})).toBeVisible();await expect(view.getByRole('button',{name:'证书服务 (1)',exact:true})).toBeVisible();
  await expect(view).not.toContainText('订单入口');await expect(view).not.toContainText('帮助外链');await expect(page.getByRole('menuitem',{name:'订单入口',exact:true})).toBeVisible();await expect(page.getByRole('menuitem',{name:'帮助外链',exact:true})).toBeVisible();
  await expect(view).toContainText('已购云服务器');await screenshot(page,'product-types-only');
  await view.getByRole('button',{name:'证书服务 (1)',exact:true}).click();await expect(page).toHaveURL(/product-directories\/dir_product_2$/);await expect(view).toContainText('已购证书服务');await expect(view).not.toContainText('已购云服务器');await page.reload();await expect(view).toContainText('已购证书服务');
  await page.getByTestId('directory-all').click();await expect(view).toContainText('已购云服务器');await expect(view).toContainText('已购证书服务');
  expect(state.queries.some(url=>!url.searchParams.has('navigationType'))).toBe(true);expect(state.queries.some(url=>url.searchParams.get('navigationType')==='product')).toBe(true);expect(state.services).not.toContain('dir_system_orders');expect(state.services).not.toContain('dir_help');await screenshot(page,'service-filter-saved-in-route');
 });
 test('TC-6b: 产品第101项使用筛选后的页码，深链接刷新与历史保持选择',async({page})=>{
  const state=await fixture(page,101);await login(page);await page.getByRole('menuitem',{name:'产品目录101',exact:true}).click();const view=page.getByTestId('customer-product-directories');
  await expect(view.getByRole('button',{name:'产品目录101 (1)',exact:true})).toBeVisible();await expect(view).toContainText('已购产品目录101');await expect(page).toHaveURL(/product-directories\/dir_product_101$/);
  expect(state.queries.some(url=>url.searchParams.get('navigationType')==='product' && url.searchParams.get('page')==='2')).toBe(true);
  await page.reload();await expect(view.getByRole('button',{name:'产品目录101 (1)',exact:true})).toHaveClass(/ant-btn-primary/);await expect(view).not.toContainText('订单入口');await screenshot(page,'filtered-page-101');
 });
 test('TC-6c: 产品分类失败保留全部服务，重试恢复，英文窄屏保持真实文案',async({page})=>{
  const state=await fixture(page);state.failProduct=true;await login(page);const view=page.getByTestId('customer-product-directories');
  await expect(view).toContainText('目录加载失败');await expect(view).toContainText('已购云服务器');await expect(page.getByRole('menuitem',{name:'订单入口',exact:true})).toBeVisible();await screenshot(page,'failure-keeps-owned-services');
  state.failProduct=false;await view.getByRole('button',{name:/^重\s*试$/}).click();await expect(view.getByRole('button',{name:'云服务器 (1)',exact:true})).toBeVisible();await expect(view).not.toContainText('目录加载失败');
  await new LoginPage(page).switchLanguage('English');await expect(view.getByRole('button',{name:'Cloud servers (1)',exact:true})).toBeVisible();await expect(view).toContainText('Purchased Cloud servers');await expect(view).toContainText('Status');await expect(view).not.toContainText('productDirectories.');await expect(page.getByRole('menuitem',{name:'Order entry',exact:true})).toBeVisible();
  await page.setViewportSize({width:414,height:896});await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await expect.poll(()=>view.locator('.vxe-pager').evaluate(el=>{const box=el.getBoundingClientRect();return [...el.querySelectorAll('button')].every(button=>{const r=button.getBoundingClientRect();return !r.width || r.left>=box.left-1 && r.right<=box.right+1 && r.top>=box.top-1 && r.bottom<=box.bottom+1;});})).toBe(true);await screenshot(page,'english-mobile');
 });
});
