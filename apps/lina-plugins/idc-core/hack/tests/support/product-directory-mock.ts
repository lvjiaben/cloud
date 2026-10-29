// UI transport fixtures only; backend PostgreSQL regressions prove customer isolation and atomic writes.
import {readFile} from 'node:fs/promises';
import type {Page} from '@host-tests/support/playwright';
export async function installDirectoryMock(page:Page,customer=false){
 const state={failNext:false,directoryFailure:false,paginated:false,hierarchy:false,writes:[] as Record<string,unknown>[],services:[] as string[]};
 const directory={navigationType:'product',targetUrl:'',icon:'lucide:server',pagePluginIds:[] as string[],directoryNo:'dir_ui_cloud',nameZhCn:'云服务器',nameEnUs:'Cloud servers',sortOrder:100,statusCode:'enabled',version:1,productNos:['prd_ui_cloud'],productCount:1,products:[{productNo:'prd_ui_cloud',nameZhCn:'回归云服务器',nameEnUs:'Test cloud server'}]};
 const created:Record<string,unknown>[]=[];
 if(customer){
  await page.route('**/api/v1/auth/login',route=>route.fulfill({json:{code:0,data:{accessToken:'ui-contract-only',refreshToken:'ui-contract-refresh',tenants:[]}}}));
  await page.route('**/api/v1/user/info',route=>route.fulfill({json:{code:0,data:{id:701,username:'directory-customer',realName:'目录测试客户',roles:['user'],permissions:[],menus:[],homePath:'/account/product-directories'}}}));
  await page.route('**/api/v1/plugins/dynamic',route=>route.fulfill({json:{code:0,data:{list:[]}}}));
  await page.route('**/customer/external-menus',route=>route.fulfill({json:{code:0,data:{items:[]}}}));
  await page.route('**/account/initial-password/status',route=>route.fulfill({json:{code:0,data:{required:false}}}));
 }
 await page.addInitScript(()=>{for(const key of Object.keys(localStorage)){if(key.startsWith('linapro:i18n:runtime:'))localStorage.removeItem(key);}});
 await page.route('https://api.iconify.design/collection?prefix=lucide',route=>route.fulfill({json:{prefix:'lucide',total:45,uncategorized:['server','cloud','database-zap','network','shield-check','wallet','user','folder',...Array.from({length:37},(_,i)=>'test-icon-'+i)]}}));
 const translations=JSON.parse(await readFile(new URL('../../../manifest/i18n/zh-CN/product-directory.json',import.meta.url),'utf8'));
 await page.route('**/api/v1/i18n/runtime/messages*',async route=>{
  const headers={...route.request().headers()};delete headers['if-none-match'];delete headers['if-modified-since'];
  const response=await route.fetch({headers});const payload=await response.json();
  const messages=payload.data?.messages??payload.messages??{};
  messages.plugin??={};messages.plugin['idc-core']??={};
  const directoryMessages:Record<string,any>={};
  for (const [key,value] of Object.entries(translations)) {
   const parts=key.replace('plugin.idc-core.productDirectory.','').split('.');
   let target=directoryMessages;
   for (const part of parts.slice(0,-1)) { target[part]??={}; target=target[part]; }
   target[parts.at(-1)!]=value;
  }
  messages.plugin['idc-core'].productDirectory=directoryMessages;
  await route.fulfill({json:{code:0,data:{messages}}});
 });
 await page.route(/\/x\/idc-core\/api\/v1\/admin\/catalog\/products(?:\?|$)/u,route=>route.fulfill({json:{code:0,data:{products:new URL(route.request().url()).searchParams.get('namePrefix')?[]:directory.products,total:1,nextCursor:''}}}));
 await page.route(/\/x\/idc-core\/api\/v1\/(admin|account)\/product-directories(?:\/[^?]+)?(?:\/[^?]+)?(?:\?|$)/u,async route=>{
  const request=route.request(),url=new URL(request.url());
  if(request.method()==='GET'){
   if(url.pathname.endsWith('/services')){
    const dir=url.pathname.split('/').at(-2)!;state.services.push(dir);
    await route.fulfill({json:{code:0,data:{items:[{entitlementNo:'ent_ui_cloud',productNo:'prd_ui_cloud',productNameZhCn:dir==='dir_ui_ssl'?'已购SSL证书':'已购云服务器',productNameEnUs:dir==='dir_ui_ssl'?'Purchased SSL certificate':'Purchased server',pluginId:'idc-zjmfcloud',statusCode:'active',startedAt:1788470400000,expiresAt:null,orderNo:'order_ui_cloud'}],total:1,page:1,pageSize:20}}});
   }else if(state.directoryFailure){await route.fulfill({status:503,json:{code:50,message:'目录加载失败'}});}else {
    const ssl={...directory,directoryNo:'dir_ui_ssl',nameZhCn:'SSL证书',nameEnUs:'SSL certificates',serviceCount:1};
    const current=Number(url.searchParams.get('page')||1);
    const first=[{...directory,serviceCount:1},...Array.from({length:99},(_,i)=>({...directory,directoryNo:`dir_extra_${i}`,nameZhCn:`其他服务${i}`,serviceCount:0}))];
    if(state.hierarchy && !("parentDirectoryNo" in directory))Object.assign(directory,{parentDirectoryNo:'dir_root',navigationType:'product',icon:'lucide:server'});
    const hierarchy=[{directoryNo:'dir_root',nameZhCn:'产品与服务',nameEnUs:'Products and services',parentDirectoryNo:'',navigationType:'system',targetUrl:'/account/servers',icon:'lucide:cloud',sortOrder:10,statusCode:'enabled',version:1,productCount:0},{...directory,serviceCount:1},{...ssl,parentDirectoryNo:'dir_root',icon:'lucide:shield-check'},{directoryNo:'dir_external',nameZhCn:'帮助文档',nameEnUs:'Documentation',navigationType:'custom',targetUrl:'https://docs.example.com',icon:'lucide:book-open',sortOrder:200,statusCode:'enabled',version:1,productCount:0}];
    const items=state.hierarchy?hierarchy:state.paginated?(current===1?first:[ssl]):[{...directory,serviceCount:1},ssl,...created];
    await route.fulfill({json:{code:0,data:{items,total:state.paginated?101:items.length,page:current,pageSize:100}}});
   }
   return;
  }
  const input=request.postDataJSON();state.writes.push(input);
  if(state.failNext){state.failNext=false;await route.fulfill({status:409,json:{code:50,message:'菜单信息冲突，请重新确认'}});return;}
  if(request.method()==='POST'){const added={...input,directoryNo:'dir_ui_new',statusCode:'enabled',version:1,productNos:[],productCount:0,products:[]};created.push(added);await route.fulfill({json:{code:0,data:{directory:added}}});return;}
  Object.assign(directory,input);directory.version++;await route.fulfill({json:{code:0,data:{directory}}});
 });
 return Object.assign(state,{directory});
}
