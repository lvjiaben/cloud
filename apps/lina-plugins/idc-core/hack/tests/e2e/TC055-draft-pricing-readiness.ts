import { test, expect } from '@host-tests/fixtures/auth';
import { installCatalogManagementMock, mockProductDetail } from '../support/catalog-mock';
import { CatalogWorkspacePage } from '../pages/CatalogWorkspacePage';
import { SalesManagementPage } from '../pages/SalesManagementPage';

test.describe('TC-55 草稿配置价格及恢复校验',()=>{
 test('TC-55a: 客户范围、快捷定价、草稿月付代理价及暂时失败恢复',async({adminPage})=>{
  await installCatalogManagementMock(adminPage);
  const detail:any=mockProductDetail('product-manual-service');
  detail.fulfillmentBinding={interfaceDisplayName:'魔方云',interfaceInstanceNo:'reference',evidenceVerified:true,interfaceNewBusinessState:'accepting'};
  detail.billingRules=[{billingRuleNo:'billing-month',chargeType:'recurring',periodUnit:'month',periodCount:1,pricePhase:'both',quantityMode:'fixed',status:'active',ruleCode:'month',minimumPurchaseCycles:1}];
  detail.prices=[{priceNo:'base',billingRuleNo:'billing-month',currencyCode:'CNY',configurationPriceDimensionNo:'',amountMinor:'10000',setupFeeMinor:'100000',renewalAmountMinor:'10000',status:'active'}];
  const cpu={key:'cpu',type:'integer',label:{zhCN:'处理器核心数',enUS:'CPU cores'},minimum:'1',maximum:'4',required:true};
  const configuration={version:2,merchantValues:[{fieldKey:'cpu',integerValue:'2'}],customerFieldSelections:[],customerConfigurableFields:[],productDeclaration:{version:'v1',fields:[cpu]},customerDeclaration:{version:'v1',fields:[{...cpu,required:false}]},customerResolvedCandidates:{},productResolvedCandidates:{}};
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  const writes:any[]=[];
  await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{if(r.request().method()==='PUT'){writes.push(r.request().postDataJSON());configuration.merchantValues=writes.at(-1).merchantValues;configuration.version++;}await r.fulfill({json:{code:0,data:{configuration}}});});
  let checks=0;let failNext=1;
  await adminPage.route('**/admin/catalog/product-versions/*/configuration/evaluation',async r=>{checks++;if(failNext>0){failNext--;await r.fulfill({json:{code:50,message:'商品目录管理服务暂时不可用'}});return;}const input=r.request().postDataJSON();await r.fulfill({json:{code:0,data:{version:configuration.version,evaluation:{valid:true,values:input.merchantValues,issues:[],fieldStates:{cpu:{visible:true,required:true,disabled:false}},candidates:{}}}}});});
  await adminPage.route('**/x/idc-core/api/v1/currencies',r=>r.fulfill({json:{code:0,data:{currencies:['CNY']}}}));
  await adminPage.route('**/configuration-price-candidates',r=>r.fulfill({json:{code:0,data:{candidates:[{candidateToken:'cpu-token',dimensionNo:'',fieldLabelZhCN:'处理器核心数',fieldLabelEnUS:'CPU cores',candidateLabelZhCN:'处理器核心数',candidateLabelEnUS:'CPU cores',quantity:true}]}}}));
  let dimensions=0;await adminPage.route('**/configuration-price-dimensions',r=>{dimensions++;return r.fulfill({json:{code:0,data:{dimensionNo:'cpu-dimension'}}});});
  const prices:any[]=[];await adminPage.route('**/billing-rules/*/price-schedules',async r=>{prices.push(r.request().postDataJSON());await r.fulfill({json:{code:0,data:{}}});});
  await adminPage.route('**/sales-management/prices?*',r=>r.fulfill({json:{code:0,data:{items:[],total:0}}}));
  await adminPage.route('**/sales-management/price-level-candidates?*',r=>r.fulfill({json:{code:0,data:{items:[{levelNo:'agent-standard',name:'标准代理',status:'active'}],total:1}}}));
  await adminPage.route('**/sales-management/price-product-candidates?*',r=>r.fulfill({json:{code:0,data:{items:[{productNo:'product-manual-service',name:'人工迁移服务',englishName:'Migration',terms:[{currencyCode:'CNY',chargeTypeCode:'recurring',periodUnitCode:'month',periodCount:1}]}],currencies:['CNY'],total:1}}}));
  const agentPrices:any[]=[];await adminPage.route('**/sales-management/prices',async r=>{agentPrices.push(r.request().postDataJSON());await r.fulfill({json:{code:0,data:{item:{...agentPrices.at(-1),priceNo:'agent-price',version:1}}}});});
  const workspace=new CatalogWorkspacePage(adminPage);await workspace.goto();await workspace.openProductDetail('product-manual-service');await workspace.selectEditSection('购买配置');
  await expect(workspace.detailDrawer).toContainText('购买时选择 · 1—4');
  await workspace.openMerchantConfiguration(false);await expect.poll(()=>checks).toBeGreaterThanOrEqual(2);await expect(workspace.merchantConfigurationForm()).not.toContainText('暂时无法核对最新配置');
  failNext=2;await workspace.fillMerchantField('处理器核心数','3');await expect(adminPage.getByTestId('catalog-configuration-retry')).toBeVisible();await expect(workspace.merchantConfigurationForm().getByLabel('处理器核心数',{exact:true})).toHaveValue('3');await workspace.screenshot('configuration-retry-retains-input');
  await workspace.retryConfiguration();await expect(adminPage.getByTestId('catalog-configuration-retry')).toBeHidden();await workspace.saveMerchantConfiguration();await expect(adminPage.getByText('配置已保存',{exact:true})).toBeVisible();await expect(workspace.merchantConfigurationForm()).toBeVisible();expect(writes[0].merchantValues).toContainEqual({fieldKey:'cpu',integerValue:'3'});
  await workspace.openFieldPrice('cpu');const priceModal=adminPage.getByTestId('catalog-configuration-price-modal');await expect(priceModal).toContainText('CNY');await expect(priceModal.getByTestId('catalog-configuration-price-billing-rule')).toHaveCount(0);await expect(priceModal.getByTestId('catalog-configuration-price-currency')).toHaveCount(0);await expect(priceModal.getByTestId('catalog-configuration-price-setupFee')).toHaveCount(0);await workspace.setConfigurationPrice('5','6');await workspace.confirmConfigurationPrice();await expect(priceModal).toBeHidden();expect(dimensions).toBe(1);expect(prices[0]).toMatchObject({currencyCode:'CNY',configurationPriceDimensionNo:'cpu-dimension',tiers:[{amountMinor:'500',renewalAmountMinor:'600',setupFeeMinor:'0'}]});
  await workspace.openProductAgentPrice();const sales=new SalesManagementPage(adminPage);await expect(sales.drawer).toContainText('CNY · 1 月');await sales.select('levelNo','标准代理');await sales.fill('amount','80');await workspace.confirmAgentPrice();await expect(sales.drawer).toBeHidden();expect(agentPrices[0]).toMatchObject({levelNo:'agent-standard',productNo:'product-manual-service',currencyCode:'CNY',periodUnitCode:'month',periodCount:1,fixedPriceMinor:'8000'});await workspace.screenshot('draft-agent-price-saved');
 });
});
