import {test,expect} from '../support/live-commerce';
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';

test.use({baseURL:'http://127.0.0.1:6680',viewport:{width:1440,height:1000}});
test.describe('TC-101 所有服务器数值规格升序',()=>{
 test.describe.configure({timeout:60000});
 test.beforeEach(async({page})=>{
  if(process.env.WEBSITE_OPTION_ORDER_CANDIDATE||process.env.WEBSITE_COMMERCE_CANDIDATE){
   for(const file of ['site.js','configuration-defaults.js','configuration-controls.js','server-specifications.js']){
    const body=await readFile(resolve(import.meta.dirname,'../../../frontend/public/lina',file),'utf8');
    await page.route('**/lina/'+file,route=>route.fulfill({contentType:'application/javascript',body}));
   }
  }
 });
 for(const city of ['guangzhou','zhenjiang','shanghai','ningbo']){
  test('TC-101: '+city+'规格从小到大且选配正确报价',async({page},info)=>{
   const slug='ecs-'+city+'-configurable-simulation';
   const configurationResponsePromise=page.waitForResponse(response=>response.url().includes('/'+slug+'/purchase-configuration')&&!response.url().includes('/evaluate')&&!response.url().includes('/quote')&&response.request().method()==='GET');
   const evaluation=page.waitForRequest(request=>request.url().includes('/'+slug+'/purchase-configuration/evaluate'));
   const quote=page.waitForResponse(response=>response.url().includes('/'+slug+'/purchase-configuration/quote')&&response.ok());
   await page.goto('/products.html?product='+slug+'&action=configureproduct');
   const configurationResponse=await configurationResponsePromise;
   expect(configurationResponse.ok()).toBe(true);
   const configuration=(await configurationResponse.json()).data;
   await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
   const initialEvaluation=(await (await evaluation).postDataJSON()).values;
   const osField=configuration.fields.find((field:any)=>field.labelZhCN==='操作系统'||field.labelEnUS==='Operating system');
   expect(osField).toBeTruthy();
   const firstOSCard=page.locator('.os-container .os-card').first();
   const firstOSFamily=await firstOSCard.getAttribute('data-os');
   const familyPatterns:any={CentOS:/^centos(?:\b|[-_])/i,Ubuntu:/^ubuntu(?:\b|[-_])/i,Windows:/^(?:microsoft\s+)?windows(?:\b|[-_])/i,Debian:/^debian(?:\b|[-_])/i,Rocky:/^rocky(?:\b|[-_])/i,AlmaLinux:/^alma\s*linux(?:\b|[-_])/i};
   const firstOS=osField.options.find((option:any)=>familyPatterns[firstOSFamily!]?.test(option.labelZhCN||option.labelEnUS)&&!option.disabled);
   expect(firstOS).toBeTruthy();
   await expect(firstOSCard).toHaveClass(/active/);
   expect(initialEvaluation.find((value:any)=>value.fieldToken===osField.fieldToken)?.optionTokens).toEqual([firstOS.optionToken]);
   const initialQuote=(await (await quote).json()).data;
   const monthly=initialQuote.terms.find((term:any)=>/月|month/i.test(term.labelZhCN+' '+term.labelEnUS));
   expect(monthly).toBeTruthy();
   const directory=await (await page.request.get('/x/idc-core/api/v1/public/catalog/products/'+slug)).json();
   expect(Number(monthly.totalAmount)).toBeGreaterThanOrEqual(Number(directory.data.product.minimumConfigurationMonthlyPrice.amountMinor));
   await page.screenshot({path:info.outputPath('minimum-price-default.png'),animations:'disabled'});
   for(const key of ['cpu','memory','systemDisk','bandwidth','inbound']){
    const row=page.locator('[data-server-specification="'+key+'"]');
    const labels=await row.locator('.lina-option-buttons button').allTextContents();
    expect(labels.length).toBeGreaterThan(0);
    const numbers=labels.map(text=>Number(text.match(/\d+(?:\.\d+)?/)![0]));
    expect(numbers).toEqual([...numbers].sort((a,b)=>a-b));
    const select=row.locator('select'),token=(await select.getAttribute('id'))!.replace('field-','');
    const field=configuration.fields.find((f:any)=>f.fieldToken===token);
    expect(new Set(await row.locator('option').evaluateAll(options=>options.map((o:any)=>o.value)))).toEqual(new Set(field.options.map((o:any)=>o.optionToken)));
    const original=configuration.values.find((v:any)=>v.fieldToken===token)?.optionTokens?.[0]||field.options.find((o:any)=>!o.disabled).optionToken;
    await expect(select).toHaveValue(original);
   }
   const network=configuration.fields.find((f:any)=>f.labelZhCN==='网络类型');
   await expect(page.locator('[data-server-specification="network"] option')).toHaveText(network.options.map((o:any)=>o.labelZhCN));
   const memory=page.locator('[data-server-specification="memory"]'),buttons=memory.locator('.lina-option-buttons button');
   const target=await buttons.first().getAttribute('aria-pressed')==='true'?buttons.last():buttons.first();
   const token=await target.getAttribute('data-option');
   const quoteResponse=page.waitForResponse(r=>r.url().includes('/'+slug+'/purchase-configuration/quote')&&r.ok());
   await target.click();
   const quoted=(await (await quoteResponse).json()).data;
   await expect(memory.locator('select')).toHaveValue(token!);
   await expect(memory.locator('button[aria-pressed="true"]')).toHaveAttribute('data-option',token!);
   const periodButton=page.locator('[data-quote] button').first();
   const termToken=await periodButton.getAttribute('data-billing-term-token');
   const term=quoted.terms.find((item:any)=>item.billingTermToken===termToken);
   expect(term).toBeTruthy();
   await periodButton.click();
   await expect(page.locator('[data-total-amount]')).toHaveText('¥'+(Number(term.totalAmount)/100).toFixed(2));
   await memory.scrollIntoViewIfNeeded();
   await page.screenshot({path:info.outputPath('ascending-specifications.png'),animations:'disabled'});
   const refreshedConfigurationPromise=page.waitForResponse(response=>response.url().includes('/'+slug+'/purchase-configuration')&&!response.url().includes('/evaluate')&&!response.url().includes('/quote')&&response.request().method()==='GET');
   const reloadedEvaluationRequest=page.waitForRequest(request=>request.url().includes('/'+slug+'/purchase-configuration/evaluate'));
   await page.reload();
   await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
   const refreshedConfigurationResponse=await refreshedConfigurationPromise;
   expect(refreshedConfigurationResponse.ok()).toBe(true);
   const refreshedConfiguration=(await refreshedConfigurationResponse.json()).data;
   const refreshedOS=refreshedConfiguration.fields.find((field:any)=>field.labelZhCN==='操作系统'||field.labelEnUS==='Operating system');
   const refreshedFirstOSCard=page.locator('.os-container .os-card').first();
   const refreshedFirstOSFamily=await refreshedFirstOSCard.getAttribute('data-os');
   const refreshedFirstOS=refreshedOS.options.find((option:any)=>familyPatterns[refreshedFirstOSFamily!]?.test(option.labelZhCN||option.labelEnUS)&&!option.disabled);
   await expect(refreshedFirstOSCard).toHaveClass(/active/);
   await expect.poll(async()=>{
    const evaluation=await reloadedEvaluationRequest;
    return evaluation.postDataJSON().values.find((value:any)=>value.fieldToken===refreshedOS.fieldToken)?.optionTokens;
   }).toEqual([refreshedFirstOS.optionToken]);
   const refreshedMemory=page.locator('[data-server-specification="memory"]'),refreshedButtons=refreshedMemory.locator('.lina-option-buttons button');
   const labels=await refreshedButtons.allTextContents();
   expect(labels.map(x=>Number(x.match(/\d+/)![0]))).toEqual(labels.map(x=>Number(x.match(/\d+/)![0])).sort((a,b)=>a-b));
   // Earlier feedback: the same purchase view exposes server names below category groups.
   await expect(page.locator('[data-catalog-product="'+slug+'"]')).toBeVisible({timeout:15000});
  });
 }
 test('TC-101e: 镇江正式商品的评估回写后仍使用视觉首个操作系统',async({page},info)=>{
  const slug='ecs-zhenjiang-balanced-draft-20261020';
  const configPromise=page.waitForResponse(r=>r.url().includes('/'+slug+'/purchase-configuration')&&!r.url().includes('/evaluate')&&!r.url().includes('/quote')&&r.request().method()==='GET');
  await page.goto('/products.html?product='+slug+'&action=configureproduct');
  const configResponse=await configPromise;expect(configResponse.ok()).toBe(true);
  const config=(await configResponse.json()).data;
  const os=config.fields.find((field:any)=>field.labelZhCN==='操作系统'||field.labelEnUS==='Operating system');expect(os).toBeTruthy();
  const firstCard=page.locator('.os-container .os-card').first(),firstFamily=await firstCard.getAttribute('data-os');
  const patterns:any={CentOS:/^centos(?:\b|[-_])/i,Ubuntu:/^ubuntu(?:\b|[-_])/i,Windows:/^(?:microsoft\s+)?windows(?:\b|[-_])/i,Debian:/^debian(?:\b|[-_])/i,Rocky:/^rocky(?:\b|[-_])/i,AlmaLinux:/^alma\s*linux(?:\b|[-_])/i};
  const firstOption=os.options.find((option:any)=>patterns[firstFamily!]?.test(option.labelZhCN||option.labelEnUS)&&!option.disabled);expect(firstOption).toBeTruthy();
  await expect(page.locator('[data-quote] button')).toHaveCount(4,{timeout:25000});
  await expect(firstCard).toHaveClass(/active/);
  await expect(page.locator('.os-container .os-card.active')).toHaveCount(1);
  await expect(page.locator('.os-container .os-card.active')).toHaveAttribute('data-os',firstFamily!);
  await expect(page.locator('#field-'+os.fieldToken)).toHaveValue(firstOption.optionToken);
  await page.screenshot({path:info.outputPath('zhenjiang-first-os-after-evaluation.png'),animations:'disabled'});
 });
});
