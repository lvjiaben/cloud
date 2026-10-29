import {test,expect} from '../support/source-site';
import {controlledBusiness} from '../support/controlled-business';
import {WebsitePage} from '../pages/WebsitePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:1000}});
const fields=[
 {fieldToken:'os',type:'single_select',presentation:'operating_system',labelZhCN:'操作系统',labelEnUS:'Operating system',visible:true,required:true,options:[{optionToken:'ubuntu',labelZhCN:'Ubuntu 24.04',labelEnUS:'Ubuntu 24.04'},{optionToken:'centos',labelZhCN:'CentOS7',labelEnUS:'CentOS7'},{optionToken:'tencent',labelZhCN:'TencentOS 3',labelEnUS:'TencentOS 3'}]},
 {fieldToken:'region',type:'single_select',presentation:'region',labelZhCN:'地区',labelEnUS:'Region',visible:true,required:true,options:[{optionToken:'xa',labelZhCN:'陕西西安',labelEnUS:'Xi’an'},{optionToken:'cd',labelZhCN:'四川成都',labelEnUS:'Chengdu'}]},
 {fieldToken:'cpu',type:'integer',displayControl:'radio',labelZhCN:'CPU',labelEnUS:'CPU',visible:true,minimumInteger:'1',maximumInteger:'8',numericChoices:['2','1','4']},
 {fieldToken:'bandwidth',type:'integer',displayControl:'auto',labelZhCN:'带宽',labelEnUS:'Bandwidth',visible:true,minimumInteger:'20',maximumInteger:'205',quantityStep:'10',unit:'Mbps'},
];
const initial=[{fieldToken:'os',type:'single_select',optionTokens:['ubuntu']},{fieldToken:'region',type:'single_select',optionTokens:['xa']},{fieldToken:'cpu',type:'integer',integerValue:'2'},{fieldToken:'bandwidth',type:'integer',integerValue:'20'}];
async function fixture(page:any,{language='zh-CN',editing=false,quantity=1,savedEditing=true,annual=false}={}){
 await controlledBusiness(page);await page.addInitScript(({language})=>localStorage.setItem('lina.website.locale',language),{language});
 await page.route('**/__website/session',(route:any)=>route.fulfill({json:{checked:true,accessToken:'current-session'}}));
 const requests:any[]=[],state={rejectPrice:false,rejectLogin:false,rejectCart:true};
 const item={cartItemNo:'single',publicSlug:'server',version:1,quantity,configurationReference:'snapshot',purchaseCycles:annual?3:1,hasServerLogin:editing,serverLoginToken:editing?'previous-token':'',serverLoginHostname:editing?'original-host':'',serverLoginPasswordConfigured:editing};
 await page.route('**/x/idc-core/api/v1/**',async(route:any)=>{
  const r=route.request(),path=new URL(r.url()).pathname,body=r.postDataJSON();requests.push({path,body,method:r.method()});
  const ok=(data:any)=>route.fulfill({json:{code:0,data}});
  if(path.endsWith('/catalog/products/server')){expect(path).toContain('/public/');return ok({product:{publicSlug:'server',nameZhCN:'配置验收服务器',nameEnUS:'Configuration test server',availability:'available',currencies:['CNY']}});}
  if(path.endsWith('/server/purchase-configuration')){if(!path.includes('/public/'))return route.fulfill({status:404,json:{code:404,data:null}});return ok({ready:true,valid:true,configurationRevision:'revision',fields,values:initial,serverLoginPolicy:{hostnameEnabled:true,passwordEnabled:true,password:{length:16,characters:['lowercase','uppercase','digits']}}});}
  if(path.endsWith('/purchase-configuration/evaluate'))return ok({ready:true,valid:true,configurationRevision:'revision',fields,values:body.values,serverLoginPolicy:{hostnameEnabled:true,passwordEnabled:true}});
  if(path.endsWith('/purchase-configuration/quote')){if(state.rejectPrice)return route.fulfill({status:503,json:{code:50}});const bandwidth=body.values.find((v:any)=>v.fieldToken==='bandwidth')?.integerValue||'20';return ok({configurationRevision:'revision',currencyCode:body.currencyCode,terms:[...(annual?[{billingTermToken:'annual',status:'final',minimumPurchaseCycles:1,labelZhCN:'年付',labelEnUS:'Annually',totalAmount:'12000'}]:[]),{billingTermToken:'monthly',status:'final',minimumPurchaseCycles:1,labelZhCN:'月付',labelEnUS:'Monthly',totalAmount:String(Number(bandwidth)*100)}]});}
  if(path.endsWith('/purchase-configuration/server-login')){if(state.rejectLogin)return route.fulfill({status:503,json:{code:50}});return ok({serverLoginToken:'new-token',hostname:body.hostname,passwordConfigured:true});}
  if(path.endsWith('/commerce/cart')||path.endsWith('/commerce/carts'))return ok({cart:{version:2,currencyCode:annual?'USD':'CNY',items:[item]}});
  if(path.endsWith('/commerce/cart-items')||path.endsWith('/commerce/cart-items/single')){expect(body.expectedCartVersion).toBe(2);if(state.rejectCart)return route.fulfill({status:503,json:{code:50}});return ok({cart:{version:3,items:[{...item,configurationReference:'new-snapshot',serverLoginToken:body.serverLoginToken||body.purchase?.serverLoginToken}]}});}
  return route.fallback();
 });
 if(editing&&savedEditing)await page.addInitScript(({initial,annual})=>sessionStorage.setItem('website-main-purchase-drafts',JSON.stringify({snapshot:{configurationRevision:'revision',values:annual?initial.map(v=>v.fieldToken==='bandwidth'?{...v,integerValue:'40'}:v):initial,billingTermToken:annual?'annual':'monthly',termLabelZhCN:annual?'年付':'月付',termLabelEnUS:annual?'Annually':'Monthly'}})),{initial,annual});
 return {requests,state,item};
}
test.describe('TC-157 当前业务选配适配',()=>{
 for(const language of ['zh-CN','en-US'])test('TC-157'+(language==='zh-CN'?'a':'b')+': 分类、步长、逐台凭据和失败恢复 '+language,async({page},info)=>{
  const {requests,state}=await fixture(page,{language}),site=new WebsitePage(page),zh=language==='zh-CN';await site.open('/configure.html?product=server');
  await expect(page.locator('[data-total-amount]')).toHaveText('¥20.00');
  await expect(page.locator('.os-card.active')).toContainText('Ubuntu');
  await expect(page.locator('[data-catalog-login]')).toBeHidden();
  await expect(page.locator('[data-server-specification=cpu] .lina-option-buttons button')).toHaveText(['2','1','4']);
  await expect(page.locator('[data-server-specification=region] .lina-option-buttons button')).toHaveCount(2);
  await expect(page.locator('[data-server-specification=bandwidth] input[type=number]')).toHaveAttribute('step','10');
  await expect(page.locator('[data-server-specification=bandwidth] input[type=number]')).toHaveAttribute('max','200');
  await expect(page.locator('[data-server-login]')).toContainText(zh?'选填，留空则自动生成。':'Optional. Leave blank to generate automatically.');
  await site.capture(info.title+'-initial',false);
  await page.locator('[data-server-specification=bandwidth] .sky-config-increase').click();await expect(page.locator('[data-total-amount]')).toHaveText('¥30.00');
  state.rejectPrice=true;await page.locator('[data-server-specification=bandwidth] input[type=number]').fill('40');await expect(page.locator('[data-status]')).toContainText(zh?'配置或报价不可用':'The configuration or quote is unavailable.');await expect(page.locator('[data-add]')).toBeDisabled();
  await site.capture(info.title+'-price-failed',false);state.rejectPrice=false;await page.locator('[data-refresh]').click();await expect(page.locator('[data-total-amount]')).toHaveText('¥40.00');
  await page.locator('#server-hostname').fill('host-one');await page.locator('#server-password').fill('TestPassword7!');state.rejectLogin=true;await page.locator('[data-add]').click();await expect(page.locator('[data-add]')).toBeEnabled();await expect(page.locator('#server-password')).toHaveValue('TestPassword7!');expect(requests.filter(r=>r.path.endsWith('/commerce/cart-items'))).toHaveLength(0);
  state.rejectLogin=false;await page.locator('[data-add]').click();await expect.poll(()=>requests.filter(r=>r.path.endsWith('/commerce/cart-items')).length).toBe(1);await expect(page.locator('#server-password')).toHaveValue('');await expect(page.locator('[data-add]')).toBeEnabled();
  await page.locator('[data-add]').click();await expect.poll(()=>requests.filter(r=>r.path.endsWith('/commerce/cart-items')).length).toBe(2);
  expect(requests.filter(r=>r.path.endsWith('/server-login'))).toHaveLength(2); // rejected attempt + successful private save, then token reuse.
  expect(requests.filter(r=>r.path.endsWith('/commerce/cart-items')).map(r=>r.body)).toEqual([expect.objectContaining({quantity:1,serverLoginToken:'new-token',expectedCartVersion:2}),expect.objectContaining({quantity:1,serverLoginToken:'new-token',expectedCartVersion:2})]);
  expect(JSON.stringify(requests.filter(r=>!r.path.endsWith('/server-login')))).not.toContain('TestPassword7!');expect(await page.evaluate(()=>JSON.stringify({...sessionStorage,...localStorage}))).not.toContain('TestPassword7!');
  await site.open('/configure.html?product=server');await expect(page.locator('#server-password')).toHaveAttribute('placeholder',zh?'密码已设置，留空保留原密码':'Password is set. Leave blank to keep it.');
  await page.setViewportSize({width:390,height:844});await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await site.capture(info.title+'-mobile',true);
 });
 test('TC-157d: 以当前购物车版本完成加购并打开官网购物车',async({page})=>{
  const {requests,state}=await fixture(page);state.rejectCart=false;const site=new WebsitePage(page);await site.open('/configure.html?product=server');await expect(page.locator('[data-add]')).toBeEnabled();await page.locator('#server-hostname').fill('one-server');await page.locator('[data-add]').click();await page.waitForURL('**/cart.html?lang=zh-CN');expect(requests.find(r=>r.path.endsWith('/commerce/cart-items')).body).toMatchObject({expectedCartVersion:2,quantity:1,serverLoginToken:'new-token'});
 });
 test('TC-157c: 编辑主机名空密码保留原秘密引用，多台项目禁止复制输入',async({page},info)=>{
  const {requests,item}=await fixture(page,{editing:true});const site=new WebsitePage(page);await site.open('/configure.html?product=server&cartItemNo=single');await expect(page.locator('#server-hostname')).toHaveValue('original-host');await expect(page.locator('[data-add]')).toBeEnabled();await page.locator('#server-hostname').fill('edited-host');await page.locator('[data-add]').click();await expect.poll(()=>requests.filter(r=>r.path.endsWith('/commerce/cart-items/single')).length).toBe(1);
  expect(requests.find(r=>r.path.endsWith('/server-login')).body).toMatchObject({hostname:'edited-host',password:'',previousServerLoginToken:'previous-token'});expect(requests.find(r=>r.path.endsWith('/commerce/cart-items/single')).body).toMatchObject({quantity:1,purchase:{serverLoginToken:'new-token'}});
  item.quantity=2;await site.open('/configure.html?product=server&cartItemNo=single');await expect(page.locator('[data-add]')).toBeEnabled();await page.locator('#server-hostname').fill('not-shared');await page.locator('[data-add]').click();await expect(page.locator('[data-status]')).toContainText('其余服务器分别配置');expect(requests.filter(r=>r.path.endsWith('/server-login'))).toHaveLength(1);await site.capture('current-server-multiple-edit-rejected',false);
 });
 test('TC-157e: 编辑保留已选配置、币种、年度周期和购买周期数',async({page})=>{
  const {requests,state}=await fixture(page,{editing:true,annual:true});state.rejectCart=false;await page.goto('/configure.html?product=server&cartItemNo=single');await expect(page.locator('[data-add]')).toBeEnabled();await expect(page.locator('[data-billing-term-token=annual]')).toHaveAttribute('aria-pressed','true');await expect(page.locator('[data-server-specification=bandwidth] input[type=number]')).toHaveValue('40');await page.locator('#server-hostname').fill('annual-host');await page.locator('[data-add]').click();await page.waitForURL('**/cart.html?lang=zh-CN');expect(requests.find(r=>r.path.endsWith('/commerce/cart-items/single')).body.purchase).toMatchObject({currencyCode:'USD',billingTermToken:'annual',purchaseCycles:3,values:expect.arrayContaining([expect.objectContaining({fieldToken:'bandwidth',integerValue:'40'})])});
 });
 test('TC-157f: 跨会话无法恢复时明确提示并等待客户重新选择周期',async({page})=>{
  await fixture(page,{editing:true,savedEditing:false});await page.goto('/configure.html?product=server&cartItemNo=single');await expect(page.locator('[data-status]')).toContainText('原选项无法恢复');await expect(page.locator('[data-add]')).toBeDisabled();await page.locator('[data-billing-term-token=monthly]').click();await expect(page.locator('[data-add]')).toBeEnabled();
 });
 test('TC-157g: 自动生成项目已提交但丢响应后重试不新增第二台',async({page})=>{
  await fixture(page);let opens=0,writes=0,committed=0;const versions:number[]=[];
  await page.route('**/account/commerce/carts',route=>{opens++;return route.fulfill({json:{code:0,data:{cart:{version:committed?3:2,items:[]}}}});});
  await page.route('**/account/commerce/cart-items',route=>{writes++;const body=route.request().postDataJSON();versions.push(body.expectedCartVersion);expect(body.serverLoginToken).toBeUndefined();if(!committed){committed++;return route.abort('failed');}return route.fulfill({json:{code:51,data:null}});});
  await page.goto('/configure.html?product=server');await expect(page.locator('[data-add]')).toBeEnabled();await page.locator('[data-add]').click();await expect.poll(()=>writes).toBe(1);await expect(page.locator('[data-add]')).toBeEnabled();await page.locator('[data-add]').click();await expect.poll(()=>writes).toBe(2);expect(opens).toBe(1);expect(versions).toEqual([2,2]);expect(committed).toBe(1);
 });

 test('TC-157h: 已登录目录进入选配及往返重进只公开读取配置，账户核价',async({page},info)=>{
  const {requests}=await fixture(page);await page.route('**/public/catalog/products?**',route=>route.fulfill({json:{code:0,data:{products:[{publicSlug:'server',categoryNo:'cat-a',nameZhCN:'配置验收服务器',nameEnUS:'Configuration test server',availability:'available',currencies:['CNY']}],categories:[{categoryNo:'cat-a',nameZhCN:'测试分类甲'}],nextCursor:''}}}));const site=new WebsitePage(page);await site.open('/products.html');await expect(page.locator('[data-catalog-login]')).toBeHidden();
  for(let i=0;i<2;i++){await page.locator('[data-product="server"] .buy-now-button').click();await expect(page.locator('[data-add]')).toBeEnabled();await expect(page.locator('.allocation-header-title h1')).toContainText('配置验收服务器');await expect(page.locator('[data-total-amount]')).toHaveText('¥20.00');if(i===0)await page.locator('.nav-top-page .return').click();}
  const reads=requests.filter(r=>r.method==='GET'&&r.path.endsWith('/purchase-configuration'));expect(reads).toHaveLength(2);expect(reads.every(r=>r.path.includes('/public/'))).toBe(true);expect(requests.some(r=>r.method==='POST'&&r.path.includes('/account/')&&r.path.endsWith('/quote'))).toBe(true);await site.capture('signed-catalog-configuration-restored',false);
 });

});
