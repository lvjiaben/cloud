import {test,expect} from '../support/live-commerce';
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {ServerSpecificationsPage} from '../pages/ServerSpecificationsPage';

const simulationTitles=['区域','操作系统','CPU','内存','系统盘','网络类型','带宽','流入带宽','IP数量','数据盘','远程端口','主机名和密码'];
const fixedTitles=['操作系统','远程端口','主机名和密码'];
test.use({baseURL:'http://127.0.0.1:6680'});
test.describe('TC-99 全站统一服务器规格与真实报价',()=>{
 test.describe.configure({timeout:60000});
 test.beforeEach(async({page})=>{
  if(process.env.WEBSITE_SPECIFICATIONS_CANDIDATE){
   for(const name of ['site.js','server-specifications.js','server-country.js','product-sidebar.js','catalog-page.js','commerce-heading.js','catalog-page.css']){
    const body=await readFile(resolve(import.meta.dirname,'../../../frontend/public/lina',name),'utf8');
    await page.route('**/lina/'+name,route=>route.fulfill({contentType:name.endsWith('.css')?'text/css':'application/javascript',body}));
   }
   await page.route('**/lina/flags/*.svg',async route=>route.fulfill({contentType:'image/svg+xml',body:await readFile(resolve(import.meta.dirname,'../../../frontend/public/lina/flags',new URL(route.request().url()).pathname.split('/').pop()!))}));
  }
 });
 for(const [index,slug]of ['ecs-zhenjiang-balanced-draft-20261020','ecs-zhenjiang-configurable-simulation','ecs-shanghai-configurable-simulation','ecs-guangzhou-configurable-simulation','ecs-ningbo-configurable-simulation'].entries()){
  test(`TC-99${['a','b','e','f','g'][index]}: ${slug} 共用规格顺序，后台选项和周期报价保持有效`,async({page})=>{
   const site=new ServerSpecificationsPage(page);let quoted:any;
   page.on('response',async response=>{if(/\/purchase-configuration\/(quote|evaluate)$/.test(response.url())){const body=await response.json();if(!response.ok()||body.code)console.log('TC099 pricing response: '+JSON.stringify({status:response.status(),code:body.code,message:body.message}));if(response.url().endsWith('/quote')&&response.ok())quoted=body.data;}});
   await site.open('/products.html?product='+slug+'&action=configureproduct');
   await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
   const titles=index===0?fixedTitles:simulationTitles;
   expect(await site.titles().allTextContents()).toEqual(titles);
   await expect(page.locator('[data-models] .lina-server-flag')).toHaveAttribute('alt','中国国旗');
   await expect.poll(()=>page.locator('[data-models] .lina-server-flag').evaluate((img:HTMLImageElement)=>img.complete&&img.naturalWidth>0)).toBe(true);
   await expect(page.locator('[data-models] [data-catalog-usage-notice]')).toBeVisible();
   await expect(page.locator('[data-catalog-usage-notice] p').first()).toHaveText('温馨提示：服务器禁止涉及色情、赌博、政治、诈骗等违法违规业务，请遵守所在地以及中国大陆法律法规，合法使用云产品资源。');
   await expect(site.specification('hostname')).toHaveCount(0);
   await expect(site.specification('password')).toHaveCount(0);
   await expect(page.locator('[data-fields]')).toContainText('主机名和密码');
   await expect(page.locator('[data-fields]')).toContainText('人工交付时安全生成，不在订单保存明文');
   if(index===0){await expect(site.specification('cpu')).toHaveCount(0);await expect(site.specification('systemDisk')).toHaveCount(0);}
   else{
    const config=await page.request.get('/x/idc-core/api/v1/public/catalog/products/'+slug+'/purchase-configuration');
    const fields=(await config.json()).data.fields,cpu=fields.find((field:any)=>field.labelZhCN==='处理器核心数'),network=fields.find((field:any)=>field.labelZhCN==='网络类型'),systemDisk=fields.find((field:any)=>field.labelZhCN==='系统盘（GiB）'),dataDisk=fields.find((field:any)=>field.labelZhCN==='数据盘（GiB，0表示不要）');
    await expect(site.specification('cpu').locator('select')).toHaveAttribute('id','field-'+cpu.fieldToken);
    await expect(site.specification('cpu').locator('option')).toHaveText(cpu.options.map((option:any)=>option.labelZhCN));
    const diskButtons=site.specification('systemDisk').locator('.lina-option-buttons button');
    await expect(site.specification('region').locator('.lina-server-flag')).toHaveAttribute('alt','中国国旗');
    await expect(diskButtons).toHaveText(['30 GiB','50 GiB','100 GiB']);
    // Presentation order must not change the backend's current/default selection.
    const initialValues=(await config.json()).data.values;
    const diskToken=initialValues.find((value:any)=>value.fieldToken===systemDisk.fieldToken)?.optionTokens?.[0]||systemDisk.options.find((option:any)=>!option.disabled).optionToken;
    await expect(site.specification('systemDisk').locator('button[aria-pressed="true"]')).toHaveAttribute('data-option',diskToken);
    await expect(site.specification('disk').locator('input[type="range"]')).toHaveAttribute('min',String(dataDisk.minimumInteger));
    await expect(site.specification('disk').locator('input[type="range"]')).toHaveAttribute('max',String(dataDisk.maximumInteger));
    const networkSelect=site.specification('network').locator('select');await expect(networkSelect).toHaveCount(1);await expect(site.specification('network').locator('.lina-option-buttons')).toHaveCount(0);
    const networkBefore=quoted;await networkSelect.selectOption({index:1});await expect.poll(()=>quoted).not.toBe(networkBefore);
    const before=quoted;await site.chooseSpecification('cpu',1);await expect.poll(()=>quoted).not.toBe(before);
    await expect(diskButtons).toHaveText(['30 GiB','50 GiB','100 GiB']);
    const diskQuote=quoted;await site.chooseSpecification('systemDisk',0);await expect.poll(()=>quoted).not.toBe(diskQuote);
    await expect(site.specification('systemDisk').locator('button[aria-pressed="true"]')).toHaveText('30 GiB');
    await expect(page.locator('[data-quote] button')).toHaveCount(4);
   }
   await expect(page.locator('[data-quote] button')).toHaveText(['月','季','半年','年']);
   await expect(page.locator('[data-purchase-controls] .sky-config-row')).toHaveCount(0);
   await expect(page.locator('[data-purchase-controls] [aria-label="币种"]')).toHaveCount(0);
   await expect(page.locator('[data-purchase-controls] [name="quantity"]')).toHaveAttribute('type','hidden');
   await expect(page.locator('[data-purchase-controls] [name="cycles"]')).toHaveAttribute('type','hidden');
   await site.selectTerm(0);await expect(page.locator('[data-add]')).toBeEnabled();
   const periodButton=page.locator('[data-quote] button').first(),termToken=await periodButton.getAttribute('data-billing-term-token'),term=quoted.terms.find((item:any)=>item.billingTermToken===termToken);
   expect(term).toBeDefined();
   await expect(page.locator('[data-total-amount]')).toHaveText('¥'+(Number(term.totalAmount)/100).toFixed(2));
   await expect(page.locator('[data-quote]').locator('..').locator('..').locator('.label-title')).toHaveText('周期');
   console.log('TC099 fields: '+await site.captureSpecifications('tc099-'+index+'-specifications',index===0?'os':'region'));
   await site.reload();await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
   expect(await site.titles().allTextContents()).toEqual(titles);
   if(index!==0)await expect(site.specification('systemDisk').locator('.lina-option-buttons button')).toHaveText(['30 GiB','50 GiB','100 GiB']);
  });
 }
 test('TC-99c: 报价失败禁购，重读恢复后缺失规格不变成虚假选择',async({page})=>{
  const site=new ServerSpecificationsPage(page);let fail=true;
  await page.route('**/purchase-configuration/quote',route=>fail?route.fulfill({status:503,json:{code:1}}):route.fallback());
  await site.open('/configure.html?product=ecs-zhenjiang-balanced-draft-20261020');
  await expect(page.locator('[data-status]')).toHaveText('配置或报价不可用，商品可能已变更。请重新读取配置。',{timeout:25000});await expect(page.locator('[data-add]')).toBeDisabled();
  await expect(site.specification('region')).toHaveCount(0);
  console.log('TC099 failure: '+await site.captureSpecifications('tc099-quote-failure','os'));
  fail=false;await site.refreshConfiguration();await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});await site.selectTerm(0);await expect(page.locator('[data-add]')).toBeEnabled();
 });
 test('TC-99d: 手机规格与费用栏无横向溢出，通用告示与商家提示并存',async({page})=>{
  const site=new ServerSpecificationsPage(page);await site.setViewport(390,900);
  await page.route('**/catalog/products?limit=1',async route=>{
   const response=await route.fetch(),body=await response.json();
   for(const category of body.data.categories)category.usageNotice='本方商品使用说明（受控测试）';
   await route.fulfill({response,json:body});
  });
  await site.open('/configure.html?product=ecs-zhenjiang-configurable-simulation');await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
  await expect(page.locator('[data-catalog-usage-notice]')).toContainText('温馨提示：服务器禁止涉及色情、赌博、政治、诈骗等违法违规业务，请遵守所在地以及中国大陆法律法规，合法使用云产品资源。');
  await expect(page.locator('[data-merchant-usage-notice]')).toHaveText('本方商品使用说明（受控测试）');
  expect(await site.titles().allTextContents()).toEqual(simulationTitles);
  expect((await site.layout()).scrollWidth).toBeLessThanOrEqual(390);
  console.log('TC099 mobile: '+await site.captureSpecifications('tc099-mobile','os'));
 });
 test('TC-99k: 字段和系统家族随后台响应变化，新增规格不丢失且隐藏字段不外露',async({page})=>{
  const site=new ServerSpecificationsPage(page);let restricted=false;
  const response=await page.request.get('/x/idc-core/api/v1/public/catalog/products/ecs-zhenjiang-balanced-draft-20261020/purchase-configuration');
  const fixture=await response.json();expect(fixture.code).toBe(0);
  const extra={fieldToken:'backend-custom-defense',labelZhCN:'单机防御',type:'single_select',visible:true,options:[{optionToken:'defense-20',labelZhCN:'20 Gbps'},{optionToken:'defense-50',labelZhCN:'50 Gbps'}]};
  await page.route('**/purchase-configuration',async route=>{
   const body=structuredClone(fixture);
   body.data.fields=body.data.fields.filter((field:any)=>field.labelZhCN==='操作系统');
   body.data.fields[0].options=body.data.fields[0].options.filter((option:any)=>/Ubuntu/i.test(option.labelZhCN));
   body.data.fields.push({...extra,visible:!restricted},{fieldToken:'backend-hidden',labelZhCN:'内部管理字段',type:'single_line',visible:false});
   await route.fulfill({json:body});
  });
  // Keep controlled field changes through evaluation; never send fictional tokens to the backend.
  await page.route('**/purchase-configuration/evaluate',async route=>{
   const body=structuredClone(fixture);
   body.data.fields=body.data.fields.filter((field:any)=>field.labelZhCN==='操作系统');
   body.data.fields[0].options=body.data.fields[0].options.filter((option:any)=>/Ubuntu/i.test(option.labelZhCN));
   body.data.fields.push({...extra,visible:!restricted});
   body.data.values=route.request().postDataJSON().values||[];
   body.data.valid=true;
   await route.fulfill({json:body});
  });
  await page.route('**/purchase-configuration/quote',route=>route.fulfill({status:503,json:{code:1}}));
  await site.open('/configure.html?product=ecs-zhenjiang-balanced-draft-20261020');
  await expect(site.titles()).toHaveText(['操作系统','单机防御']);
  await expect(page.locator('[data-fields] .os-card')).toHaveCount(1);
  await expect(page.locator('[data-fields] .os-name')).toHaveText('Ubuntu');
  await expect(site.specification(extra.fieldToken).locator('button')).toHaveText(['20 Gbps','50 Gbps']);
  await site.chooseSpecification(extra.fieldToken,1);
  await expect(site.specification(extra.fieldToken).locator('button[aria-pressed="true"]')).toHaveText('50 Gbps');
  await expect(page.locator('[data-fields]')).not.toContainText('内部管理字段');
  await expect(page.locator('[data-status]')).toHaveText('配置或报价不可用，商品可能已变更。请重新读取配置。');
  await expect(page.locator('[data-add]')).toBeDisabled();
  console.log('TC099 backend fields: '+await site.captureSpecifications('tc099-backend-fields','os'));
  restricted=true;await site.refreshConfiguration();
  await expect(site.titles()).toHaveText(['操作系统']);
  await expect(site.specification(extra.fieldToken)).toHaveCount(0);
  await expect(page.locator('[data-status]')).toHaveText('配置或报价不可用，商品可能已变更。请重新读取配置。');
 });
 for(const [suffix,name,country]of [['h','日本东京测试服务器','日本'],['i','美国洛杉矶测试服务器','美国'],['j','演示云服务器 标准型01','']]){
  test(`TC-99${suffix}: ${name} 在商品卡和型号中按明确地区配旗，未知留空且侧栏不显示`,async({page})=>{
   const slug='ecs-zhenjiang-balanced-draft-20261020',site=new ServerSpecificationsPage(page);let liveName=name;
   await page.route('**/catalog/products**',async route=>{
    const url=new URL(route.request().url());
    if(url.pathname.endsWith('/purchase-configuration')||url.pathname.includes('/purchase-configuration/'))return route.fallback();
    const response=await route.fetch(),body=await response.json();
    for(const product of [body.data?.product,...body.data?.products||[]])if(product?.publicSlug===slug){product.nameZhCN=liveName;product.nameEnUS='';product.showNewBadge=suffix==='h';}
    await route.fulfill({response,json:body});
   });
   const check=async(selector:string)=>{
    const title=page.locator(selector);await expect(title).toHaveText(name);
    if(country){await expect(title.locator('.lina-server-flag')).toHaveAttribute('alt',country+'国旗');await expect.poll(()=>title.locator('img').evaluate((img:HTMLImageElement)=>img.complete&&img.naturalWidth>0)).toBe(true);}
    else await expect(title.locator('.lina-server-flag')).toHaveCount(0);
   };
   await site.open('/products.html?product='+slug);
   await check('[data-products] h5');await expect(page.locator('[data-catalog-product="'+slug+'"] .lina-product-name')).toHaveText(name);await expect(page.locator('[data-catalog-product="'+slug+'"] .lina-server-flag')).toHaveCount(0);
   await site.buy();await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
   await check('[data-models] .lina-configure-model');await check('.allocation-header-title h1');
   await expect(page.locator('[data-models] .lina-usage-icon')).toHaveText('!');
   if(suffix==='h'){
    const amount=await page.locator('[data-total-amount]').textContent();
    liveName='美国洛杉矶实时更新服务器';await site.returnFocus();
    await expect(page.locator('[data-models] .lina-configure-model')).toHaveText(liveName,{timeout:10000});
    await expect(page.locator('[data-models] .lina-configure-model .lina-server-flag')).toHaveAttribute('alt','美国国旗');
    await expect(page.locator('.allocation-header-title h1')).toHaveText(liveName);
    await expect(page.locator('[data-catalog-product="'+slug+'"] .lina-server-flag')).toHaveCount(0);
    await expect(page.locator('[data-catalog-product="'+slug+'"] .lina-product-name')).toHaveText(liveName);
    await expect(page.locator('[data-catalog-product="'+slug+'"] .lina-product-new-badge')).toHaveText('NEW');
    await expect(page.locator('[data-total-amount]')).toHaveText(amount!);
    await expect(site.specification('os')).toBeVisible();
   }
   console.log('TC099 country: '+await site.captureSpecifications('tc099-country-'+suffix,'os'));
  });
 }
});
