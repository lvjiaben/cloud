import {test,expect} from '../support/source-site';
import {controlledBusiness,prefix} from '../support/controlled-business';
import {SettlementPage} from '../pages/SettlementPage';

test.use({baseURL:'http://localhost:6680',viewport:{width:1440,height:900}});
test.describe('TC-78 服务器多付款周期',()=>{
 test('TC-78a: 四周期金额、规格改选重新报价与失败保护',async({page})=>{
  await controlledBusiness(page);
  const root=prefix+'/catalog/products/multiple-period-server',revision='p'.repeat(43);
  const fields=[{fieldToken:'spec',type:'single_select',visible:true,required:true,labelZhCN:'服务器规格',options:[{optionToken:'standard',labelZhCN:'标准配置'},{optionToken:'enhanced',labelZhCN:'增强配置'}]}];
  let enhanced=false,failure=false;
  await page.route('**'+root,r=>r.fulfill({json:{code:0,data:{product:{publicSlug:'multiple-period-server',nameZhCN:'多周期服务器测试',availability:'available',currencies:['CNY']}}}}));
  await page.route('**'+root+'/purchase-configuration',r=>r.fulfill({json:{code:0,data:{configurationRevision:revision,fields,values:[]}}}));
  await page.route('**'+root+'/purchase-configuration/evaluate',r=>{const values=r.request().postDataJSON().values;enhanced=values.find((v:any)=>v.fieldToken==='spec')?.optionTokens?.[0]==='enhanced';return r.fulfill({json:{code:0,data:{configurationRevision:revision,fields,values,valid:true}}});});
  await page.route('**'+root+'/purchase-configuration/quote',r=>failure?r.fulfill({status:503,json:{code:503}}):r.fulfill({json:{code:0,data:{configurationRevision:revision,currencyCode:'CNY',terms:[6,12,3,1].map(months=>({billingTermToken:'term-'+months,labelZhCN:months===1?'1月':months===3?'3月':months===6?'6月':'1年',minimumPurchaseCycles:1,status:'final',subtotalAmount:String((enhanced?13900:6900)*months),setupAmount:'0',taxAmount:'0',totalAmount:String((enhanced?13900:6900)*months)}))}}}));
  const site=new SettlementPage(page);await site.open('/configure.html?product=multiple-period-server');
  await expect(page.locator('[data-quote] button')).toHaveCount(4);await expect(page.locator('[data-quote] button')).toHaveText(['月：CNY 69.00','季：CNY 207.00','半年：CNY 414.00','年：CNY 828.00']);
  for(const [i,months]of [1,3,6,12].entries()){await site.selectTerm(i);await expect(page.locator('[data-total-amount]')).toHaveText('¥'+(69*months).toFixed(2));await expect(page.locator('[data-quote] button').nth(i)).toHaveAttribute('aria-pressed','true');await expect(page.locator('[data-add]')).toBeEnabled();}
  await site.selectConfiguration('enhanced');await expect(page.locator('[data-total-amount]')).toHaveText('¥139.00');await expect(page.locator('[data-add]')).toBeDisabled();
  for(const [i,months]of [1,3,6,12].entries()){await site.selectTerm(i);await expect(page.locator('[data-total-amount]')).toHaveText('¥'+(139*months).toFixed(2));}
  await site.toggleDetails();await expect(page.locator('[data-breakdown]')).toContainText('¥1668.00');await site.capture('tc078-yearly-configured',false);
  failure=true;await site.selectConfiguration('standard');await expect(page.locator('[data-total-amount]')).toHaveText('待报价');await expect(page.locator('[data-add]')).toBeDisabled();await expect(page.locator('[data-quote] button')).toHaveCount(0);
 });
});
