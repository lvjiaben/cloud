// Verify required defaults, supplier choices, custom option mappings and price maintenance.
import { test, expect } from '@host-tests/fixtures/auth';
import { installCatalogManagementMock, mockProductDetail } from '../support/catalog-mock';
import { CustomerOptionsPage } from '../pages/CustomerOptionsPage';
import { CatalogWorkspacePage } from '../pages/CatalogWorkspacePage';

const label=(zhCN:string,enUS:string)=>({zhCN,enUS});
const cpu={key:'cpu',type:'integer',label:label('处理器核心数','CPU cores'),minimum:'1',maximum:'1024'};

test.describe('TC-53 商品配置与价格维护',()=>{
 test('TC-53a: 默认规格、已选必填提示、自定义名称范围及真实系统候选投影',async({adminPage})=>{
  await installCatalogManagementMock(adminPage);
  const detail=mockProductDetail('product-manual-service');
  detail.fulfillmentBinding={interfaceDisplayName:'魔方云',interfaceInstanceNo:'server-reference',evidenceVerified:true,interfaceNewBusinessState:'accepting'} as any;
  const configuration:any={version:0,merchantValues:[],customerFieldSelections:[],customerConfigurableFields:[cpu],productDeclaration:{version:'v1',fields:[{key:'region',type:'single_select',label:label('地区','Region'),required:true,dynamicCandidates:{version:'v1',method:'configuration.candidates'}},{...cpu,required:true},{key:'hostname',type:'single_line',label:label('主机名前缀','Hostname prefix')}]},customerDeclaration:{version:'v1',fields:[{key:'operating_system',type:'single_select',label:label('操作系统','Operating system'),required:true,dynamicCandidates:{version:'v1',method:'configuration.candidates'}}]},productResolvedCandidates:{region:[{value:'west',label:label('陕西西安','Xi’an')}]},customerResolvedCandidates:{operating_system:[{value:'img-1',label:label('Ubuntu 24.04','Ubuntu 24.04')},{value:'img-2',label:label('Windows Server 2022','Windows Server 2022')}]}};
  const writes:any[]=[];const evaluations:any[]=[];
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  await adminPage.route('**/admin/catalog/product-versions/*/configuration',async route=>{
   if(route.request().method()==='PUT'){
    const input=route.request().postDataJSON();writes.push(input);configuration.merchantValues=input.merchantValues;configuration.version++;
    if(input.customerFieldSelections?.length){configuration.customerFieldSelections=input.customerFieldSelections;configuration.customerDeclaration.fields.push({...cpu,label:label(input.customerFieldSelections[0].displayNameZhCN,input.customerFieldSelections[0].displayNameEnUS),minimum:'2',maximum:'16'});}
   }
   await route.fulfill({json:{code:0,data:{configuration}}});
  });
  await adminPage.route('**/admin/catalog/product-versions/*/configuration/evaluation',async route=>{
   const input=route.request().postDataJSON();evaluations.push(input);
   const keys=new Set(input.merchantValues.map((x:any)=>x.fieldKey));const issues=['region','cpu'].filter(key=>!keys.has(key)).map(fieldKey=>({fieldKey,issueCode:'required'}));
   await route.fulfill({json:{code:0,data:{version:configuration.version,evaluation:{values:input.merchantValues,valid:!issues.length,issues,fieldStates:{region:{visible:true,required:true,disabled:false},cpu:{visible:true,required:true,disabled:false}},candidates:configuration.productResolvedCandidates}}}});
  });
  const workspace=new CustomerOptionsPage(adminPage);await workspace.goto();await workspace.openProductDetail('product-manual-service');await workspace.selectEditSection('购买配置');
  await expect(adminPage.getByTestId('catalog-customer-options-form')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-customer-options-prerequisite')).toContainText('请先保存后台商品配置');
  const systems=adminPage.getByTestId('catalog-customer-choices-operating_system');await expect(systems).toContainText('2 个可选项');await workspace.expandCustomerChoices('operating_system');await expect(systems).toContainText('Ubuntu 24.04');
  await workspace.openMerchantConfiguration(false);await expect(workspace.merchantConfigurationForm().getByLabel('主机名前缀',{exact:true})).toHaveCount(0);
  await expect.poll(()=>evaluations.length).toBeGreaterThan(0);
  await workspace.selectMerchantField('地区','陕西西安');await expect.poll(()=>evaluations.at(-1)?.merchantValues.some((x:any)=>x.fieldKey==='region')).toBe(true);
  await expect(workspace.merchantConfigurationForm().locator('.ant-form-item').filter({has:adminPage.getByLabel('地区',{exact:true})}).locator('.ant-form-item-explain-error')).toHaveCount(0);
  await workspace.fillMerchantField('处理器核心数','2');await workspace.toggleAdvancedConfiguration();await expect(workspace.merchantConfigurationForm().getByLabel('主机名前缀',{exact:true})).toBeVisible();
  await workspace.saveMerchantConfiguration();await expect(adminPage.getByText('配置已保存',{exact:true})).toBeVisible();await expect(workspace.merchantConfigurationForm()).toBeVisible();
  await workspace.openOptions();await workspace.addParameter('处理器核心数');await workspace.customizeCPU('性能核心','Performance cores','5');await workspace.setCPUBounds('2','16');await workspace.confirmOptions();await expect(adminPage.getByText('客户可选配置已保存',{exact:true})).toBeVisible();await expect(workspace.optionsForm).toBeVisible();
  expect(writes.at(-1)).toMatchObject({expectedVersion:1,customerFieldSelections:[{fieldKey:'cpu',displayNameZhCN:'性能核心',displayNameEnUS:'Performance cores',sortOrder:5,minimum:'2',maximum:'16'}]});
  expect(configuration.customerDeclaration.fields[0]).toMatchObject({key:'operating_system',required:true});
  await workspace.reopenOptions();await expect(adminPage.getByTestId('option_0_name_zhCN')).toHaveValue('性能核心');await expect(adminPage.getByTestId('option_0_maximum')).toHaveValue('16');await workspace.screenshot('custom-options-saved-core-preserved');
 });

 test('TC-53b: 配置页修改删除价格，税务使用名称，草稿代理价格解释前置条件',async({adminPage})=>{
  await installCatalogManagementMock(adminPage);
  const detail:any=mockProductDetail('product-manual-service');
  detail.billingRules=[{billingRuleNo:'billing-once',chargeType:'one_time',periodUnit:'once',periodCount:1,pricePhase:'new_purchase',quantityMode:'fixed',status:'active',ruleCode:'once',minimumPurchaseCycles:1}];
  detail.prices=[{priceNo:'price-old',billingRuleNo:'billing-once',currencyCode:'CNY',configurationPriceDimensionNo:'dimension-cpu',configurationFieldLabelZhCn:'性能核心',configurationFieldLabelEnUs:'Performance cores',configurationCandidateLabelZhCn:'按数量',configurationCandidateLabelEnUs:'Quantity',minimumQuantity:0,maximumQuantity:null,amountMinor:'500',setupFeeMinor:'0',renewalAmountMinor:null,status:'active'}];
  detail.prices.push({...detail.prices[0],priceNo:'base-price',configurationPriceDimensionNo:'',configurationFieldLabelZhCn:'',amountMinor:'1000'});
  detail.taxPolicies=[];
  const requests:any[]=[];
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  await adminPage.route('**/admin/catalog/product-versions/*/configuration',r=>r.fulfill({json:{code:0,data:{configuration:null}}}));
  await adminPage.route('**/x/idc-core/api/v1/currencies',r=>r.fulfill({json:{code:0,data:{currencies:['CNY']}}}));
  await adminPage.route('**/configuration-price-candidates',r=>r.fulfill({json:{code:0,data:{candidates:[{candidateToken:'cpu-token',dimensionNo:'dimension-cpu',fieldLabelZhCN:'性能核心',fieldLabelEnUS:'Performance cores',candidateLabelZhCN:'按数量',candidateLabelEnUS:'Quantity',quantity:true,pricingMode:'per_unit'}]}}}));
  await adminPage.route('**/price-schedules/price-*',async route=>{const method=route.request().method();requests.push({method,body:method==='PUT'?route.request().postDataJSON():undefined});if(method==='PUT')detail.prices=[{...detail.prices[0],priceNo:'price-new',amountMinor:'700'},...detail.prices.slice(1)];else detail.prices=detail.prices.filter((p:any)=>!p.configurationPriceDimensionNo);await route.fulfill({json:{code:0,data:{}}});});
  const taxes:any[]=[];
  await adminPage.route('**/tax-policies',async route=>{taxes.push(route.request().postDataJSON());detail.taxPolicies=[{...taxes[0],taxPolicyNo:'tax-cny'}];await route.fulfill({json:{code:0,data:{}}});});
  const workspace=new CatalogWorkspacePage(adminPage);await workspace.goto();await workspace.openProductDetail('product-manual-service');await workspace.selectEditSection('购买配置');
  const prices=adminPage.getByTestId('catalog-configuration-prices');await expect(prices).toContainText('在商品基础售价上增加对应费用');await workspace.editConfigurationPrice('price-old');
  await expect(adminPage.getByTestId('catalog-configuration-price-amount')).toHaveValue('5.00');await workspace.setConfigurationPrice('7','');await workspace.confirmConfigurationPrice();await expect.poll(()=>requests.filter(x=>x.method==='PUT').length).toBe(1);expect(requests[0].body.tiers[0].amountMinor).toBe('700');
  await workspace.deleteConfigurationPrice('price-new');await expect.poll(()=>requests.filter(x=>x.method==='DELETE').length).toBe(1);await expect(prices).not.toContainText('性能核心');
  await workspace.selectEditSection('价格与税务');await expect(adminPage.getByTestId('catalog-agent-price-prerequisite')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-edit-agent-prices')).toBeEnabled();await workspace.openTax('应税','cloud_service');await expect(adminPage.getByTestId('catalog-tax-modal')).toContainText('无需填写内部代码');await workspace.setTaxRate('6');await workspace.confirmTax();expect(taxes[0]).toMatchObject({taxCategoryCode:'cloud_service',taxRateBasisPoints:600});await workspace.screenshot('configuration-prices-managed-prerequisites');
 });
 test('TC-53c: 修改选择后忽略迟到的旧必填校验',async({adminPage})=>{
  await installCatalogManagementMock(adminPage);
  const detail:any=mockProductDetail('product-manual-service');detail.fulfillmentBinding={interfaceDisplayName:'魔方云',interfaceInstanceNo:'server-reference',evidenceVerified:true,interfaceNewBusinessState:'accepting'};
  const declaration={version:'v1',fields:[{key:'region',type:'single_select',label:label('地区','Region'),required:true,dynamicCandidates:{version:'v1',method:'configuration.candidates'}}]};
  const candidates={region:[{value:'west',label:label('陕西西安','Xi’an')}]};
  const configuration={version:1,merchantValues:[],customerFieldSelections:[],customerConfigurableFields:[],productDeclaration:declaration,customerDeclaration:{version:'v1',fields:[]},productResolvedCandidates:candidates};
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  const writes:any[]=[];
  await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{if(r.request().method()==='PUT')writes.push(r.request().postDataJSON());await r.fulfill({json:{code:0,data:{configuration}}});});
  let releaseOld:()=>void=()=>{};let oldArrived=false;let currentArrived=false;let oldDelivered=false;
  const oldFence=new Promise<void>(resolve=>{releaseOld=resolve;});
  await adminPage.route('**/admin/catalog/product-versions/*/configuration/evaluation',async r=>{
   const input=r.request().postDataJSON();const old=!input.merchantValues.length;
   if(old){oldArrived=true;await oldFence;}else currentArrived=true;
   await r.fulfill({json:{code:0,data:{version:1,evaluation:{valid:!old,values:input.merchantValues,issues:old?[{fieldKey:'region',issueCode:'required'}]:[],fieldStates:{region:{visible:true,required:true,disabled:false}},candidates}}}});
   if(old)oldDelivered=true;
  });
  const workspace=new CustomerOptionsPage(adminPage);await workspace.goto();await workspace.openProductDetail('product-manual-service');await workspace.openMerchantConfiguration(false);
  await expect.poll(()=>oldArrived).toBe(true);await workspace.selectMerchantField('地区','陕西西安');await expect.poll(()=>currentArrived).toBe(true);
  releaseOld();await expect.poll(()=>oldDelivered).toBe(true);
  await expect(workspace.merchantConfigurationForm().locator('.ant-form-item-explain-error')).toHaveCount(0);
  await workspace.saveMerchantConfiguration();await expect(adminPage.getByText('配置已保存',{exact:true})).toBeVisible();await expect(workspace.merchantConfigurationForm()).toBeVisible();expect(writes[0]).toMatchObject({merchantValues:[{fieldKey:'region',optionTokens:['west']}]});
 });

});
