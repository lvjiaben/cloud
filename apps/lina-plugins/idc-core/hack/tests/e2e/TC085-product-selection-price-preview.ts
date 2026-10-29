import type { Page } from '@host-tests/support/playwright';
import { test,expect } from '../support/merchant-auth';
import { installCatalogManagementMock,mockProductDetail } from '../support/catalog-mock';
import { CatalogPricePreviewPage } from '../pages/CatalogPricePreviewPage';

test.describe('TC-85 商品选配费用预览',()=>{
 async function setup(page:Page,empty=false,partial=false){
  await installCatalogManagementMock(page);
  const detail:any=mockProductDetail('product-manual-service');detail.version.status='draft';detail.product.directoryNo='pdr_test';detail.product.descriptionVersion=1;
  const priced:any=mockProductDetail('product-ssl');detail.billingRules=priced.billingRules;detail.prices=[...priced.prices,...priced.prices.map((row:any)=>({...row,currencyCode:'USD'}))];detail.taxPolicies=priced.taxPolicies;
  const field:any={key:'local_plan',type:'single_select',label:{zhCN:'性能档位',enUS:'Performance plan'},required:false,staticCandidates:[{value:'basic',label:{zhCN:'基础版',enUS:'Basic'}},{value:'fast',label:{zhCN:'增强版',enUS:'Plus'}}]};
  const configuration:any={version:1,merchantValues:[],customerConfigurableFields:[],customerFieldSelections:empty?[]:[{fieldKey:'local_plan',localField:field,required:false,candidateValues:['basic','fast'],customerVisible:true}],productDeclaration:{version:'v1',fields:[]},customerDeclaration:{version:'v1',fields:empty?[]:[field]},productResolvedCandidates:{},customerResolvedCandidates:{}};
  await page.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  await page.route('**/admin/product-directories?*',r=>r.fulfill({json:{code:0,data:{items:[{directoryNo:'pdr_test',navigationType:'product',nameZhCn:'云产品',nameEnUs:'Cloud products',statusCode:'enabled'}],total:1}}}));
  await page.route('**/admin/catalog/products/*/addons',r=>r.fulfill({json:{code:0,data:{addons:[]}}}));
  await page.route('**/public/customer-center/configuration',r=>r.fulfill({json:{code:0,data:{selectedPlugin:'customer-tdesign',config:{websiteurl:'https://shop.example',websiteconfigurepath:'/configure.html?product={product}',websitecatalogpath:'/products.html'}}}}));
  await page.route('**/admin/catalog/product-versions/*/configuration',r=>r.fulfill({json:{code:0,data:{configuration}}}));
  const requests:any[]=[];const state={fail:false};
  await page.route('**/admin/catalog/products/*/price-preview?*',r=>{
   const url=new URL(r.request().url());const values=JSON.parse(url.searchParams.get('values')||'[]');const currencyCode=url.searchParams.get('currencyCode');requests.push({method:r.request().method(),values,currencyCode});
   if(state.fail){state.fail=false;return r.fulfill({json:{code:409,data:{},message:'配置价格暂不可用'}});}
   const fast=values.some((row:any)=>row.fieldKey==='local_plan' && row.optionTokens?.includes('fast'));
   const base=currencyCode==='USD'?2000:1000;const extra=fast?(currencyCode==='USD'?300:200):0;
   const terms=[{billingRuleNo:'monthly',status:'final',chargeType:'recurring',periodUnit:'month',periodCount:1,cycles:1,baseAmountMinor:String(base),configurationAmountMinor:String(extra),setupAmountMinor:'100',taxAmountMinor:'0',totalAmountMinor:String(base+extra+100)},{billingRuleNo:'yearly',status:'final',chargeType:'recurring',periodUnit:'year',periodCount:1,cycles:1,baseAmountMinor:String(base*10),configurationAmountMinor:String(extra*10),setupAmountMinor:'100',taxAmountMinor:'0',totalAmountMinor:String(base*10+extra*10+100)}];
   if(partial) terms.unshift({billingRuleNo:'daily',status:'unavailable',chargeType:'recurring',periodUnit:'day',periodCount:1,cycles:1,baseAmountMinor:null,configurationAmountMinor:null,setupAmountMinor:null,taxAmountMinor:null,totalAmountMinor:null,issues:[{kind:'option',reason:'missing',fieldKey:'cpu',nameZhCN:'CPU',nameEnUS:'CPU'},{kind:'option',reason:'missing',fieldKey:'memory',nameZhCN:'内存',nameEnUS:'Memory'},{kind:'option',reason:'missing',fieldKey:'bandwidth',nameZhCN:'带宽',nameEnUS:'Bandwidth'}]} as any);
   return r.fulfill({json:{code:0,data:{preview:{versionNo:detail.version.versionNo,currencyCode,priceIncludesTax:false,terms}}}});
  });
  const ui=new CatalogPricePreviewPage(page);await ui.goto();await ui.openProductDetail('product-manual-service');await ui.tab('configuration');await ui.setPreviewVisible(true);
  return {ui,requests,state};
 }
 test('TC-85a: 选配、周期和币种变化展示服务端费用分项，无写入动作',async({adminPage})=>{
  const {ui,requests}=await setup(adminPage);
  await expect(ui.priceSummary).toContainText('选配费用预览');await expect(ui.priceSummary).toContainText('基础价');await expect(ui.priceSummary).toContainText('配置价');await expect(ui.priceSummary).toContainText('安装费');
  await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveText('11.00 CNY');
  await ui.selectPreviewPlan('增强版');await expect(adminPage.getByTestId('catalog-preview-configuration-amount')).toHaveText('2.00 CNY');await expect(adminPage.getByTestId('catalog-preview-setup-amount')).toHaveText('1.00 CNY');await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveText('13.00 CNY');
  expect(requests.at(-1).values).toContainEqual({fieldKey:'local_plan',optionTokens:['fast']});
  await ui.selectPreviewPeriod('1 年');await expect(adminPage.getByTestId('catalog-preview-base-amount')).toHaveText('100.00 CNY');await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveText('121.00 CNY');
  await ui.selectPreviewCurrency('USD');await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveText('231.00 USD');expect(requests.every(row=>row.method==='GET')).toBe(true);await expect(adminPage.getByTestId('catalog-preview-header-total')).toHaveText('合计 231.00 USD');
  await expect(ui.priceSummary).toContainText('未计入客户专属价、代理价、活动或优惠券');await ui.screenshot('editor-selection-price-preview');
 });
 test('TC-85b: 计算失败保留客户选择，重试恢复；未保存配置不显示旧合计',async({adminPage})=>{
  const {ui,requests,state}=await setup(adminPage);await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveText('11.00 CNY');
  state.fail=true;await ui.selectPreviewPlan('增强版');await expect(adminPage.getByTestId('catalog-preview-price-failed')).toContainText('暂时无法计算费用');await expect(adminPage.getByTestId('catalog-option-preview-local_plan')).toContainText('增强版');await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-preview-header-total')).toHaveCount(0);await ui.screenshot('editor-selection-price-failed');
  await ui.retryPrice();await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveText('13.00 CNY');
  await ui.sortParameter('local_plan','4');await expect(adminPage.getByTestId('catalog-preview-price-dirty')).toContainText('请保存后查看准确费用');await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-preview-header-total')).toHaveCount(0);
  const requestCount=requests.length;await ui.selectPreviewPlan('基础版');await expect(adminPage.getByTestId('catalog-option-preview-local_plan')).toContainText('基础版');await ui.settlePriceDebounce();expect(requests.length).toBe(requestCount);await ui.screenshot('editor-selection-price-unsaved');
 });
 test('TC-85c: 未启用客户配置时空表单仍可按币种查看真实服务端费用',async({adminPage})=>{
  const {ui,requests}=await setup(adminPage,true);
  await expect(adminPage.getByTestId('catalog-offer-preview')).toContainText('尚未启用客户配置');
  await expect(adminPage.getByTestId('catalog-option-preview-local_plan')).toHaveCount(0);
  await expect(adminPage.getByTestId('catalog-preview-header-total')).toHaveText('合计 11.00 CNY');
  await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveText('11.00 CNY');
  expect(requests.at(-1).values).toEqual([]);await ui.selectPreviewCurrency('USD');
  await expect(adminPage.getByTestId('catalog-preview-header-total')).toHaveText('合计 21.00 USD');expect(requests.at(-1).values).toEqual([]);
  await ui.screenshot('editor-price-empty-configuration');
 });

 test('TC-85d: 缺价周期不阻断其他周期，明确缺项且不显示零金额',async({adminPage})=>{
  const {ui,requests}=await setup(adminPage,false,true);
  // Unavailable daily is returned first: a complete period must be selected.
  await expect(adminPage.getByTestId('catalog-preview-header-total')).toHaveText('合计 11.00 CNY');
  await ui.selectPreviewPlan('增强版');await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveText('13.00 CNY');
  await ui.selectPreviewPeriod('1 天 · 价格未就绪');
  const warning=adminPage.getByTestId('catalog-preview-price-unavailable');
  await expect(warning).toContainText('CPU：尚未设置价格');await expect(warning).toContainText('内存：尚未设置价格');await expect(warning).toContainText('带宽：尚未设置价格');await expect(warning).toContainText('补齐或修正此币种、周期的价格');
  await expect(adminPage.getByTestId('catalog-preview-header-total')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-preview-base-amount')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-preview-price-failed')).toHaveCount(0);
  await expect(adminPage.getByTestId('catalog-option-preview-local_plan')).toContainText('增强版');await ui.screenshot('editor-selection-price-missing-period');
  await ui.selectPreviewPeriod('1 月');await expect(adminPage.getByTestId('catalog-preview-total-amount')).toHaveText('13.00 CNY');expect(requests.every(row=>row.method==='GET')).toBe(true);
 });

});
