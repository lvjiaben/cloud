import { test, expect } from '@host-tests/fixtures/auth';
import { installCatalogManagementMock, mockProductDetail } from '../support/catalog-mock';
import { CatalogWorkspacePage } from '../pages/CatalogWorkspacePage';

test.describe('TC-57 在售商品修改、金额和上下架', () => {
 test('TC-57a: 重复打开、直接修改商品、价格编辑删除和销售状态', async ({adminPage}) => {
  await installCatalogManagementMock(adminPage);
  const detail:any=mockProductDetail('product-manual-service');
  detail.product.status='active';detail.product.descriptionVersion=3;detail.product.currentVersionNo=detail.version.versionNo;
  detail.version.status='published';
  detail.billingRules=[{billingRuleNo:'billing-month',chargeType:'recurring',periodUnit:'month',periodCount:1,pricePhase:'both',quantityMode:'fixed',status:'active',ruleCode:'month',minimumPurchaseCycles:1}];
  detail.prices=[{priceNo:'base',billingRuleNo:'billing-month',currencyCode:'CNY',configurationPriceDimensionNo:'',amountMinor:'100',setupFeeMinor:'1000',renewalAmountMinor:'100',minimumQuantity:0,maximumQuantity:null,status:'active'},
   {priceNo:'cpu',billingRuleNo:'billing-month',currencyCode:'CNY',configurationPriceDimensionNo:'cpu-dimension',configurationPricingMode:'per_unit',configurationFieldLabelZhCn:'处理器核心数',configurationFieldLabelEnUs:'CPU cores',configurationCandidateLabelZhCn:'处理器核心数',configurationCandidateLabelEnUs:'CPU cores',amountMinor:'500',setupFeeMinor:'0',renewalAmountMinor:'100',minimumQuantity:0,maximumQuantity:null,status:'active'}];
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  await adminPage.route('**/admin/catalog/products?*',r=>r.fulfill({json:{code:0,data:{products:[{...detail.product,versionNo:detail.version.versionNo,versionNumber:detail.version.versionNumber,versionStatus:detail.version.status,source:'manual',currencyCount:1,billingRuleCount:1}],total:1,nextCursor:''}}}));
  const revisions:any[]=[];await adminPage.route('**/admin/catalog/products/product-manual-service/draft-versions',async r=>{
   revisions.push(r.request().postDataJSON());detail.version.status='draft';detail.version.versionNo='version-revision-2';detail.version.versionNumber=2;
   await r.fulfill({json:{code:0,data:{productNo:detail.product.productNo,versionNo:detail.version.versionNo}}});
  });
  const edits:any[]=[];await adminPage.route('**/billing-rules/billing-month/price-schedules/cpu',async r=>{
   if(r.request().method()==='PUT'){const input=r.request().postDataJSON();edits.push(input);Object.assign(detail.prices[1],input.tiers[0]);}
   else detail.prices.splice(1,1);
   await r.fulfill({json:{code:0,data:{}}});
  });
  await adminPage.route('**/product-versions/version-revision-2/price-schedules/cpu',async r=>{detail.prices.splice(1,1);await r.fulfill({json:{code:0,data:{}}});});
  await adminPage.route('**/x/idc-core/api/v1/currencies',r=>r.fulfill({json:{code:0,data:{currencies:['CNY']}}}));
  await adminPage.route('**/configuration-price-candidates',r=>r.fulfill({json:{code:0,data:{candidates:[{candidateToken:'cpu-token',dimensionNo:'cpu-dimension',fieldLabelZhCN:'处理器核心数',fieldLabelEnUS:'CPU cores',candidateLabelZhCN:'处理器核心数',candidateLabelEnUS:'CPU cores',quantity:true,pricingMode:'per_unit'}]}}}));
  const saleWrites:any[]=[];
  await adminPage.route('**/admin/catalog/products/product-manual-service/sale-state',async r=>{const body=r.request().postDataJSON();saleWrites.push(body);detail.product.status=body.active?'active':'suspended';detail.product.descriptionVersion++;await r.fulfill({json:{code:0,data:{version:detail.product.descriptionVersion}}});});
  const view=new CatalogWorkspacePage(adminPage);await view.goto();
  for(let index=0;index<3;index++){
   await view.openProductDetail('product-manual-service');await expect(view.detailDrawer).toContainText('保存后立即生效');
   await view.selectEditSection('价格与税务');await expect(adminPage.getByTestId('catalog-base-price-amount-month:CNY')).toHaveValue('1.00');await expect(adminPage.getByTestId('catalog-base-price-setup-month:CNY')).toHaveValue('10.00');
   await view.closeProductDetail();await expect(view.detailDrawer).toBeHidden();
  }
  await view.openProductDetail('product-manual-service');await view.selectEditSection('购买配置');
  await view.openAdvancedConfigurationPrices();
  const prices=adminPage.getByTestId('catalog-configuration-prices');
  await expect(prices).toContainText('CNY 5.00');await expect(prices).toContainText('CNY 1.00');
  await expect(prices).not.toContainText('处理器核心数 · 处理器核心数');
  await expect(adminPage.getByTestId('catalog-configuration-price-edit-cpu')).toHaveText('编辑价格');
  await expect(adminPage.getByTestId('catalog-start-revision')).toHaveCount(0);
  await expect.poll(()=>revisions.length).toBe(1);
  expect(revisions[0]).toEqual({sourceVersionNo:detail.product.currentVersionNo});
  await expect(view.detailDrawer).toContainText('保存后立即生效');
  await view.screenshot('product-revision-prices');
  await expect(adminPage.getByTestId('catalog-configuration-price-edit-cpu')).toHaveText('编辑价格');
  await view.editConfigurationPrice('cpu');await view.setConfigurationPrice('3','1');await view.confirmConfigurationPrice();
  await expect.poll(()=>edits.length).toBe(1);expect(edits[0].tiers[0]).toMatchObject({amountMinor:'300',renewalAmountMinor:'100'});
  await expect(prices).toContainText('CNY 3.00');
  await view.deleteConfigurationPrice('cpu');await expect(prices).not.toContainText('处理器核心数');
  await view.screenshot('product-revision-price-deleted');
  await view.closeProductDetail();await expect(view.detailDrawer).toBeHidden();
  // The list's mocked source status is active. Verify the selected current
  // published pointer rather than accidentally sending the editing draft.
  await adminPage.getByTestId('catalog-product-sale-state-product-manual-service').click();
  await adminPage.locator('.ant-popover:visible').getByRole('button',{name:/确\s*定/}).click();
  await expect.poll(()=>saleWrites.length).toBe(1);expect(saleWrites[0]).toEqual({versionNo:detail.product.currentVersionNo,expectedVersion:3,active:false});
  await view.screenshot('product-off-sale-confirmed');
  const saleButton=adminPage.getByTestId('catalog-product-sale-state-product-manual-service');
  await expect(saleButton).toHaveText('重新上架');await saleButton.click();
  await adminPage.locator('.ant-popover:visible').getByRole('button',{name:/确\s*定/}).click();
  await expect.poll(()=>saleWrites.length).toBe(2);expect(saleWrites[1]).toEqual({versionNo:detail.product.currentVersionNo,expectedVersion:4,active:true});
  await expect(saleButton).toHaveText(/^下\s*架$/u);await view.screenshot('product-back-on-sale-confirmed');
 });
 test('TC-57b: 接口状态中文显示，旧插件记录不可用仍可直接编辑',async({adminPage})=>{
  await installCatalogManagementMock(adminPage);
  const detail:any=mockProductDetail('product-manual-service');detail.product.status='active';detail.product.currentVersionNo=detail.version.versionNo;detail.version.status='published';
  detail.billingRules=[{billingRuleNo:'billing-monthly',chargeType:'recurring',periodUnit:'month',periodCount:1,pricePhase:'both',quantityMode:'fixed',status:'active',ruleCode:'monthly',minimumPurchaseCycles:1}];
  detail.prices=[{priceNo:'base',billingRuleNo:'billing-monthly',currencyCode:'CNY',amountMinor:'500',setupFeeMinor:'0',renewalAmountMinor:'500',minimumQuantity:0,maximumQuantity:null,status:'active'}];
  detail.prices.push({priceNo:'cpu-local',billingRuleNo:'billing-monthly',currencyCode:'CNY',configurationPriceDimensionNo:'cpu-local-dimension',configurationPricingMode:'per_unit',configurationFieldLabelZhCn:'处理器核心数',configurationFieldLabelEnUs:'CPU cores',configurationCandidateLabelZhCn:'处理器核心数',configurationCandidateLabelEnUs:'CPU cores',amountMinor:'500',setupFeeMinor:'0',renewalAmountMinor:'100',minimumQuantity:0,maximumQuantity:null,status:'active'});
  const priceEdits:any[]=[];const priceCandidateReads:string[]=[];
  await adminPage.route('**/configuration-price-candidates',async r=>{priceCandidateReads.push(r.request().url());await r.fulfill({status:503,json:{code:50,message:'Provider unavailable'}});});
  await adminPage.route('**/billing-rules/billing-monthly/price-schedules/cpu-local',async r=>{
   const input=r.request().postDataJSON();priceEdits.push(input);Object.assign(detail.prices[1],input.tiers[0]);await r.fulfill({json:{code:0,data:{}}});
  });
  detail.fulfillmentBinding={interfaceInstanceNo:'frozen-interface',interfaceDisplayName:'历史自动化接口',evidenceVerified:false,interfaceNewBusinessState:'',boundAt:1786406400000};
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  const revisions:any[]=[];
  await adminPage.route('**/admin/catalog/products/product-manual-service/draft-versions',async r=>{
   revisions.push(r.request().postDataJSON());detail.version.status='draft';detail.version.versionNo='version-local-revision';detail.version.versionNumber=2;
   await r.fulfill({json:{code:0,data:{productNo:detail.product.productNo,versionNo:detail.version.versionNo}}});
  });
  const configurationReads:string[]=[];
  await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{configurationReads.push(r.request().url());await r.fulfill({json:{code:0,data:{configuration:null}}});});
  const view=new CatalogWorkspacePage(adminPage);await view.goto();
  for(const [state,label] of [['','未获取'],['stopped','已停止新销售'],['accepting','接收新销售']]){
   detail.fulfillmentBinding.interfaceNewBusinessState=state;
   await view.openProductDetail('product-manual-service');await view.selectEditSection('交付设置');
   await expect(view.detailDrawer).toContainText('历史自动化接口');await expect(view.detailDrawer).toContainText(label!);
   await expect(view.detailDrawer).not.toContainText('plugin.idc-core.catalogAutomation.state.');
   await expect(view.detailDrawer).not.toContainText('需要重新检查');
   await view.screenshot(`product-interface-state-${state||'unknown'}`);
   await view.closeProductDetail();await expect(view.detailDrawer).toBeHidden();
  }
  await view.openProductDetail('product-manual-service');await expect(adminPage.getByTestId('catalog-start-revision')).toHaveCount(0);
  await expect.poll(()=>revisions.length).toBe(1);expect(revisions[0]).toEqual({sourceVersionNo:detail.product.currentVersionNo});
  await expect(view.detailDrawer).toContainText('保存后立即生效');
  await view.selectEditSection('价格与税务');await expect(adminPage.getByTestId('catalog-base-price-amount-monthly:CNY')).toHaveValue('5.00');
  await expect(adminPage.locator('.ant-notification-notice-error')).toHaveCount(0);
  expect(configurationReads).toHaveLength(0);

  const bindingSaves:any[]=[];
  await adminPage.route('**/admin/catalog/automation/interfaces?*',r=>r.fulfill({json:{code:0,data:{items:[{interfaceInstanceNo:'frozen-interface',displayName:'历史自动化接口',newBusinessState:'accepting'}],total:1}}}));
  await adminPage.route('**/admin/catalog/product-versions/version-local-revision/fulfillment-policy',r=>{
   bindingSaves.push(r.request().postDataJSON());detail.fulfillmentBinding.interfaceNewBusinessState='accepting';
   return r.fulfill({json:{code:0,data:{policy:{strategyCode:'manual'}}}});
  });
  await view.openFulfillmentPolicy();await view.fulfillmentPolicyConfirm().click();
  await expect(adminPage.getByText('履约方式已保存',{exact:true})).toBeVisible();
  expect(bindingSaves).toEqual([{strategyCode:'manual'}]);
  await expect(view.fulfillmentPolicyModal).toBeVisible();
  await expect(view.detailDrawer).toContainText('历史自动化接口');
  await expect(view.detailDrawer).toContainText('接收新销售');
  expect(configurationReads).toHaveLength(0);
  await view.screenshot('product-unchanged-interface-saved');

  await view.selectEditSection('购买配置');await view.editConfigurationPrice('cpu-local');
  await view.setConfigurationPrice('3','1');await view.confirmConfigurationPrice();
  await expect.poll(()=>priceEdits.length).toBe(1);
  expect(priceEdits[0]).toMatchObject({configurationPriceDimensionNo:'cpu-local-dimension',currencyCode:'CNY',tiers:[{amountMinor:'300',renewalAmountMinor:'100'}]});
  expect(priceCandidateReads).toHaveLength(0);
  await expect(adminPage.getByTestId('catalog-configuration-prices')).toContainText('CNY 3.00');
  await expect(adminPage.locator('.ant-notification-notice-error')).toHaveCount(0);
  const localCandidateReads:string[]=[];const duplicateCreates:string[]=[];
  await adminPage.route('**/configuration-price-candidates',async r=>{localCandidateReads.push(r.request().url());await r.fulfill({json:{code:0,data:{candidates:[{candidateToken:'saved-cpu',dimensionNo:'cpu-local-dimension',pricingMode:'per_unit',quantity:true,fieldLabelZhCn:'处理器核心数',fieldLabelEnUs:'CPU cores',candidateLabelZhCn:'处理器核心数',candidateLabelEnUs:'CPU cores'}]}}});});
  await adminPage.route('**/billing-rules/billing-monthly/price-schedules',async r=>{duplicateCreates.push(r.request().url());await r.fulfill({status:409,json:{code:50,message:'不得重复创建配置价格'}});});
  await adminPage.getByTestId('catalog-configuration-price-add').click();
  await expect(adminPage.getByTestId('catalog-configuration-price-amount')).toHaveValue('3.00');
  await expect(adminPage.getByTestId('catalog-configuration-price-renewalAmount')).toHaveValue('1.00');
  await view.setConfigurationPrice('6','2');await view.confirmConfigurationPrice();
  await expect.poll(()=>priceEdits.length).toBe(2);
  expect(priceEdits[1]).toMatchObject({configurationPriceDimensionNo:'cpu-local-dimension',currencyCode:'CNY',tiers:[{amountMinor:'600',renewalAmountMinor:'200'}]});
  expect(duplicateCreates).toHaveLength(0);expect(localCandidateReads).toHaveLength(1);expect(configurationReads).toHaveLength(0);
  await expect(adminPage.getByTestId('catalog-configuration-prices')).toContainText('CNY 6.00');
  await view.screenshot('product-local-revision-provider-unavailable');
 });
 test('TC-57c: 接口暂不可用仍能打开本地价格，不请求发布检查',async({adminPage})=>{
  const state=await installCatalogManagementMock(adminPage);
  const detail:any=mockProductDetail('product-manual-service');
  detail.fulfillmentBinding={interfaceInstanceNo:'saved-interface',interfaceDisplayName:'已保存的接口',evidenceVerified:false,interfaceNewBusinessState:'accepting'};
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  let previewReads=0;
  await adminPage.route('**/publication-preview',async r=>{previewReads++;await r.fulfill({status:503,json:{code:50,message:'连接器暂不可用'}});});
  const view=new CatalogWorkspacePage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');
  await expect(view.detailDrawer).toContainText('保存后立即生效');
  await expect(adminPage.getByTestId('catalog-editor-step-publication')).toHaveCount(0);
  await expect(adminPage.getByTestId('catalog-start-revision')).toHaveCount(0);
  await view.selectEditSection('价格与税务');
  await expect(adminPage.locator('.ant-notification-notice-error')).toHaveCount(0);
  expect(previewReads).toBe(0);expect(state.publishRequests).toHaveLength(0);
  await view.screenshot('settings-no-publication-gate');
 });

 test('TC-57d: 当前接口的商品可直接进入配置，无需重建接口或发布',async({adminPage})=>{
  const state=await installCatalogManagementMock(adminPage);
  const detail:any=mockProductDetail('product-manual-service');
  detail.fulfillmentBinding={interfaceInstanceNo:'original-interface',interfaceDisplayName:'原自动化接口',evidenceVerified:true,interfaceNewBusinessState:'accepting'};
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
  let configurationReads=0;
  await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{
   expect(r.request().method()).toBe('GET');configurationReads++;
   await r.fulfill({json:{code:0,data:{configuration:null}}});
  });
  const view=new CatalogWorkspacePage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');await view.selectEditSection('交付设置');
  await expect.poll(()=>configurationReads).toBe(1);
  await expect(view.detailDrawer.locator('.ant-spin-spinning')).toHaveCount(0);
  await expect(view.detailDrawer).not.toContainText('商品暂未加载成功');
  await expect(view.detailDrawer).toContainText('原自动化接口');
  await expect(view.detailDrawer).toContainText('接收新销售');
  await expect(view.detailDrawer).not.toContainText('恢复旧包');
  await expect(adminPage.getByTestId('catalog-editor-step-publication')).toHaveCount(0);
  expect(state.previewRequests).toHaveLength(0);expect(state.publishRequests).toHaveLength(0);
  await view.screenshot('current-provider-direct-settings');
 });
});
