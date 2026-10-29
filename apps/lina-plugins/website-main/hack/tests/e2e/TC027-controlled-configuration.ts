import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {WebsitePage} from '../pages/SourceWebsitePage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
const root=prefix+'/catalog/products/controlled-alpha',revision='c'.repeat(43);
const option=(optionToken:string,labelZhCN:string,children:any[]=[])=>({optionToken,labelZhCN,children});
const definitions:any[]=[
 {fieldToken:'text',type:'single_line',labelZhCN:'测试单行',required:true,minimumLength:2,maximumLength:12},
 {fieldToken:'memo',type:'multi_line',labelZhCN:'测试多行'},
 {fieldToken:'int',type:'integer',labelZhCN:'测试整数',minimumInteger:'1',maximumInteger:'8'},
 {fieldToken:'decimal',type:'decimal',labelZhCN:'测试小数',minimumDecimal:'0.1',maximumDecimal:'8.5'},
 {fieldToken:'bool',type:'boolean',labelZhCN:'测试布尔',required:true},
 {fieldToken:'date',type:'date',labelZhCN:'测试日期'},
 {fieldToken:'datetime',type:'datetime',labelZhCN:'测试时间'},
 {fieldToken:'single',type:'single_select',labelZhCN:'测试单选',options:[option('a','选项甲'),option('b','选项乙')]},
 {fieldToken:'multi',type:'multi_select',labelZhCN:'测试多选',options:[option('a','选项甲'),option('b','选项乙')]},
 {fieldToken:'tree',type:'tree_select',labelZhCN:'测试树选',options:[option('parent','父节点',[option('child','子节点')])]},
 {fieldToken:'address',type:'address',labelZhCN:'测试地址',options:[option('province','测试省',[option('city','测试市')])]},
 {fieldToken:'conditional',type:'single_line',labelZhCN:'条件必填'},
 {fieldToken:'disabled',type:'single_line',labelZhCN:'禁用字段',disabled:true}
];
test.describe('TC-27 多类型配置与报价分支（模拟UI）',()=>{
 async function configure(page:any,quoteStatus='final'){
  const fixture=await controlledBusiness(page);let latest:any[]=[];let evaluations=0;
  const fields=(values:any[])=>{const enabled=values.some(v=>v.fieldToken==='bool'&&v.booleanValue===true);return definitions.map(f=>({...f,visible:f.fieldToken!=='conditional'||enabled,disabled:f.disabled||f.fieldToken==='decimal'&&enabled,options:f.fieldToken==='single'&&enabled?[option('a','选项甲')]:f.options,required:f.fieldToken==='conditional'?enabled:!!f.required}));};
  await page.route('**'+root,r=>r.fulfill({json:{code:0,data:{product:{publicSlug:'controlled-alpha',nameZhCN:'复杂配置测试专用',availability:'available',currencies:['CNY','USD']}}}}));
  await page.route('**'+root+'/purchase-configuration',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields:fields([]),values:[{fieldToken:'text',type:'single_line',textValue:'初始值'},{fieldToken:'bool',type:'boolean',booleanValue:false}]}}}));
  await page.route('**'+root+'/purchase-configuration/evaluate',r=>{
   evaluations++;latest=r.request().postDataJSON().values;
   const active=fields(latest);latest=latest.filter(v=>active.find(f=>f.fieldToken===v.fieldToken)?.visible&&!(v.fieldToken==='single'&&v.optionTokens?.includes('b')&&active.find(f=>f.fieldToken==='single')?.options.length===1));
   const valid=latest.some(v=>v.fieldToken==='text'&&v.textValue?.length>=2)&&(!active.find(f=>f.fieldToken==='conditional')?.required||latest.some(v=>v.fieldToken==='conditional'&&v.textValue));
   return r.fulfill({json:{code:0,data:{configurationRevision:revision,fields:active,values:latest,valid}}});
  });
  await page.route('**'+root+'/purchase-configuration/quote',r=>{const body=r.request().postDataJSON();return r.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:body.currencyCode,terms:[{billingTermToken:'controlled-term-'+body.currencyCode,labelZhCN:'测试月付',minimumPurchaseCycles:2,status:quoteStatus,totalAmount:body.currencyCode==='USD'?'345':'1234'}]}}});});
  const site=new WebsitePage(page);await site.open('/configure.html?product=controlled-alpha');await expect(page.locator('[data-quote]')).not.toBeEmpty();
  return {...fixture,site,latest:()=>latest,evaluations:()=>evaluations};
 }
 test('TC-27a: 全部非文件字段的输入投影与条件显隐必填禁用',async({page})=>{
  const f=await configure(page);
  await expect(page.getByLabel('禁用字段')).toBeDisabled();await expect(page.getByLabel('条件必填')).toHaveCount(0);
  const edit=async(label:string,value:string)=>{const before=f.evaluations();await page.getByLabel(label,{exact:true}).fill(value);await expect.poll(f.evaluations).toBeGreaterThan(before);};
  await edit('测试单行','中文输入');await edit('测试多行','多行\n第二行');await edit('测试整数','5');await edit('测试小数','1.25');await edit('测试日期','2026-10-18');await edit('测试时间','2026-10-18T12:30');
  for(const [label,values] of [['测试单选','b'],['测试多选',['a','b']],['测试树选','child']] as const){const before=f.evaluations();await page.getByLabel(label,{exact:true}).selectOption(values as any);await expect.poll(f.evaluations).toBeGreaterThan(before);}
  let before=f.evaluations();await page.getByLabel('测试地址 第1级').selectOption('province');await expect.poll(f.evaluations).toBeGreaterThan(before);
  before=f.evaluations();await page.getByLabel('测试地址 第2级').selectOption('city');await expect.poll(f.evaluations).toBeGreaterThan(before);
  const values=f.latest();expect(values).toEqual(expect.arrayContaining([
   {fieldToken:'int',type:'integer',integerValue:'5'},{fieldToken:'decimal',type:'decimal',decimalValue:'1.25'},
   {fieldToken:'tree',type:'tree_select',treeOptionTokens:['child']},{fieldToken:'multi',type:'multi_select',optionTokens:['a','b']},
   {fieldToken:'address',type:'address',addressOptionTokens:['province','city']}
  ]));
  expect(values.find(v=>v.fieldToken==='datetime').dateTimeValue).toMatch(/^2026-10-18T/);
  before=f.evaluations();await page.getByLabel('测试布尔',{exact:true}).check();await expect.poll(f.evaluations).toBeGreaterThan(before);
  await expect(page.getByLabel('测试小数',{exact:true})).toBeDisabled();await expect(page.getByLabel('测试单选',{exact:true}).locator('option')).toHaveCount(2);
  expect(f.latest().some(v=>v.fieldToken==='single')).toBe(false);
  await expect(page.getByLabel('条件必填')).toHaveAttribute('required','');await expect(page.locator('[data-add]')).toBeDisabled();
  await edit('条件必填','条件值');await expect(page.locator('[data-quote] button')).toBeVisible();
  before=f.evaluations();await page.getByLabel('测试布尔',{exact:true}).uncheck();await expect.poll(f.evaluations).toBeGreaterThan(before);
  expect(f.latest().some(v=>v.fieldToken==='conditional')).toBe(false);await expect(page.getByLabel('条件必填')).toHaveCount(0);
  await edit('测试单行','');await expect(page.locator('[data-quote]')).toBeEmpty();await expect(page.locator('[data-add]')).toBeDisabled();
  await edit('测试单行','恢复合法');await f.site.chooseTerm();await expect(page.locator('[data-add]')).toBeEnabled();
  await edit('测试整数','9');await f.site.chooseTerm();expect(await page.getByLabel('测试整数').evaluate((el:HTMLInputElement)=>el.validity.rangeOverflow)).toBe(true);
  let writes=0;await page.route('**/public/commerce/**',r=>{writes++;return r.abort();});await page.locator('#addCartForm').dispatchEvent('submit');expect(writes).toBe(0);
  await f.site.capture('controlled-complex-fields-invalid');
 });
 test('TC-27b: 多币种切换清旧报价，数量周期和权威周期标识精确加车',async({page})=>{
  const f=await configure(page);await expect(page.locator('[data-quote] button')).toHaveText('测试月付：CNY 12.34');
  await f.site.chooseTerm();await page.getByLabel('币种',{exact:true}).selectOption('USD');await expect(page.locator('[data-quote] button')).toHaveText('测试月付：USD 3.45');await expect(page.locator('[data-add]')).toBeDisabled();
  await f.site.setPurchaseCount('quantity','3');await f.site.setPurchaseCount('cycles','4');await f.site.chooseTerm();
  let body:any;
  await page.route('**/public/commerce/carts',r=>r.fulfill({json:{code:0,data:{cart:{reference:'controlled-cart'}}}}));
  await page.route('**/public/commerce/cart-items',r=>{body=r.request().postDataJSON();return r.fulfill({json:{code:0,data:{cart:{reference:'controlled-cart'}}}});});
  // Intercept the destination too: no real cart or customer action occurs.
  await page.route('**/account/cart',r=>r.fulfill({contentType:'text/html',body:'<h1>受控购物车交接</h1>'}));
  await f.site.submit();await expect(page).toHaveURL('http://localhost:6032/account/cart');
  expect(body).toEqual({publicSlug:'controlled-alpha',configurationRevision:revision,currencyCode:'USD',billingTermToken:'controlled-term-USD',quantity:3,purchaseCycles:4,values:f.latest()});
 });
 test('TC-27c: 客户上下文报价只打开本商品用户中心，不自动保存草稿',async({page,context})=>{
  const f=await configure(page,'requires_customer_context');let saves=0;
  await page.route('**/public/purchase-configuration-handoffs',r=>{saves++;return r.abort();});
  await context.route('**/account/products/controlled-alpha/configure?currency=CNY',r=>r.fulfill({contentType:'text/html',body:'<h1>受控用户中心配置入口</h1>'}));
  const link=page.locator('[data-quote] a');await expect(link).toContainText('需登录用户中心确认价格');
  const [popup]=await Promise.all([page.waitForEvent('popup'),link.click()]);await expect(popup).toHaveURL('http://localhost:6032/account/products/controlled-alpha/configure?currency=CNY');
  expect(await popup.evaluate(()=>window.opener)).toBeNull();await popup.close();
  await expect(page).toHaveURL(/configure.html\?product=controlled-alpha$/);await expect(page.locator('[data-add]')).toBeDisabled();expect(saves).toBe(0);
  await f.site.capture('controlled-customer-context-link');
 });
 test('TC-27d: 迟到评估不覆盖新字段，错版及错币种报价禁止加车',async({page})=>{
  const f=await configure(page);let release:()=>void=()=>{};const held=new Promise<void>(r=>{release=r;});let oldStarted=false;
  await page.route('**'+root+'/purchase-configuration/evaluate',async r=>{
   const values=r.request().postDataJSON().values;
   if(values.some((v:any)=>v.textValue==='迟到旧值')){oldStarted=true;await held;}
   return r.fulfill({json:{code:0,data:{configurationRevision:revision,values,fields:definitions.map(x=>({...x,visible:x.fieldToken!=='conditional'})),valid:true}}});
  });
  await page.getByLabel('测试单行',{exact:true}).fill('迟到旧值');await expect.poll(()=>oldStarted).toBe(true);
  await page.getByLabel('测试单行',{exact:true}).fill('当前新值');await expect(page.locator('[data-quote] button')).toBeVisible();
  const late=page.waitForResponse(r=>r.url().endsWith('/evaluate')&&r.request().postData()!.includes('迟到旧值'));release();await (await late).finished();
  await page.evaluate(()=>new Promise<void>(resolve=>requestAnimationFrame(()=>resolve())));
  // Wait for the old response itself, not an arbitrary timeout.
  await expect(page.getByLabel('测试单行',{exact:true})).toHaveValue('当前新值');
  for(const mismatch of ['revision','currency']){
   await page.route('**'+root+'/purchase-configuration/quote',r=>r.fulfill({json:{code:0,data:{configurationRevision:mismatch==='revision'?'old-revision':revision,currencyCode:mismatch==='currency'?'USD':'CNY',terms:[{billingTermToken:'bad',status:'final',totalAmount:'1'}]}}}));
   await f.site.refreshConfiguration();await expect(page.locator('[data-status]')).toHaveText('配置或报价不可用，商品可能已变更。请重新读取配置。');
   await expect(page.locator('[data-quote]')).toBeEmpty();await expect(page.locator('[data-add]')).toBeDisabled();
  }
  await f.site.capture('controlled-stale-quote-rejected');
 });
 test('TC-27e: 用量与空周期不提供匿名购买或登录成功承诺',async({page})=>{
  const f=await configure(page,'pending_usage');await expect(page.locator('[data-quote] button')).toContainText('按实际用量结算');
  await expect(page.locator('[data-quote] button')).toBeDisabled();await expect(page.locator('[data-quote] a')).toHaveCount(0);await expect(page.locator('[data-add]')).toBeDisabled();
  await page.route('**'+root+'/purchase-configuration/quote',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[]}}}));
  await f.site.refreshConfiguration();await expect(page.locator('[data-status]')).toHaveText('当前没有可用的结算周期。');await expect(page.locator('[data-add]')).toBeDisabled();
 });
});

