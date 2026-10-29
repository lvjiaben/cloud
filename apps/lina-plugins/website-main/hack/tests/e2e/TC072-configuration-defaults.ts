import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {SettlementPage} from '../pages/SettlementPage';
test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-72 所有商品默认第一个规格',()=>{
 test('TC-72a: 自动选择、联动二次校验、保留用户改选与失败禁止加车',async({page},info)=>{
  await controlledBusiness(page);const root=prefix+'/catalog/products/new-default-server',revision='e'.repeat(43);
  const initial=[{fieldToken:'spec',type:'single_select',visible:true,required:true,labelZhCN:'规格',options:[{optionToken:'first',labelZhCN:'第一规格'},{optionToken:'second',labelZhCN:'第二规格'}]}];
  const linked={fieldToken:'linked',type:'single_select',visible:true,required:true,labelZhCN:'联动规格',options:[{optionToken:'linked-first',labelZhCN:'第一联动规格'}]};let latest:any[]=[],calls=0,failure=false;
  await page.route('**'+root,r=>r.fulfill({json:{code:0,data:{product:{publicSlug:'new-default-server',nameZhCN:'新服务器默认规格',availability:'available',currencies:['CNY']}}}}));
  await page.route('**'+root+'/purchase-configuration',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields:initial,values:[]}}}));
  await page.route('**'+root+'/purchase-configuration/evaluate',r=>{calls++;latest=r.request().postDataJSON().values;return r.fulfill({json:{code:0,data:{configurationRevision:revision,fields:[...initial,linked],values:latest,valid:latest.some(v=>v.fieldToken==='linked')}}});});
  await page.route('**'+root+'/purchase-configuration/quote',r=>failure?r.fulfill({status:503,json:{code:503}}):r.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[{billingTermToken:'month',labelZhCN:'月付',status:'final',minimumPurchaseCycles:1,totalAmount:latest.find(v=>v.fieldToken==='spec')?.optionTokens[0]==='second'?'20000':'10000'}]}}}));
  const site=new SettlementPage(page);await site.open('/configure.html?product=new-default-server');await expect(page.locator('[data-total-amount]')).toHaveText('¥100.00');expect(calls).toBe(2);await expect(page.locator('[data-fields] option[value=""]')).toHaveCount(0);await expect(page.locator('[data-fields] button').filter({hasText:'请选择'})).toHaveCount(0);await expect(page.locator('#field-linked')).toHaveValue('linked-first');
  await site.selectConfiguration('second');await expect(page.locator('[data-total-amount]')).toHaveText('¥200.00');await site.refreshConfiguration();await expect(page.locator('#field-spec')).toHaveValue('second');await expect(page.locator('[data-total-amount]')).toHaveText('¥200.00');await site.chooseTerm();await expect(page.locator('[data-add]')).toBeEnabled();await site.captureContent(info.outputPath(Date.now()+'-default-specs.png'),0);
  failure=true;await site.selectConfiguration('first');await expect(page.locator('[data-total-amount]')).toHaveText('待报价');await expect(page.locator('[data-add]')).toBeDisabled();await site.captureContent(info.outputPath(Date.now()+'-quote-failure.png'),0);
 });
});
