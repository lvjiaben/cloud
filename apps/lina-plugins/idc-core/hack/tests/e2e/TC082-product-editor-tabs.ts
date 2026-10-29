// Product editing is verified through the real Vue/Vben page. All business
// writes in these interaction tests are scoped to this test's route fixtures.
import type { Page } from '@host-tests/support/playwright';
import { test,expect } from '../support/merchant-auth';
import { installCatalogManagementMock,mockProductDetail } from '../support/catalog-mock';
import { CatalogEditorTabsPage } from '../pages/CatalogEditorTabsPage';

test.describe('TC-82 商品页签、参数弹窗与上游选择',()=>{
  async function setup(page:Page, customize?: (detail:any,configuration:any)=>void) {
    await installCatalogManagementMock(page);
    const detail:any=mockProductDetail('product-manual-service');detail.version.status='draft';detail.product.directoryNo='';detail.product.descriptionVersion=1;
    const priced:any=mockProductDetail('product-ssl');detail.billingRules=priced.billingRules;detail.prices=priced.prices;detail.taxPolicies=priced.taxPolicies;
    const configuration:any={version:1,merchantValues:[],customerConfigurableFields:[],customerFieldSelections:[],productDeclaration:{version:'v1',fields:[]},customerDeclaration:{version:'v1',fields:[]},productResolvedCandidates:{},customerResolvedCandidates:{}};
    await page.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
    await page.route('**/admin/product-directories?*',r=>r.fulfill({json:{code:0,data:{items:[{directoryNo:'pdr_test',navigationType:'product',nameZhCn:'云产品',nameEnUs:'Cloud products',statusCode:'enabled'}],total:1}}}));
    await page.route('**/admin/catalog/products/*/addons',r=>r.fulfill({json:{code:0,data:{addons:[]}}}));
    await page.route('**/public/customer-center/configuration',r=>{expect(new URL(r.request().url()).pathname).toBe('/x/idc-portal/api/v1/public/customer-center/configuration');return r.fulfill({json:{code:0,data:{selectedPlugin:'customer-tdesign',config:{websiteurl:'https://shop.example',websiteconfigurepath:'/configure.html?product={product}',websitecatalogpath:'/products.html'}}}});});
    await page.route('**/admin/catalog/product-versions/*/configuration',r=>r.fulfill({json:{code:0,data:{configuration}}}));
    customize?.(detail,configuration);
    const ui=new CatalogEditorTabsPage(page);await ui.goto();await ui.openProductDetail('product-manual-service');
    return {ui,detail,configuration};
  }
  test('TC-82a: 五页签、直显库存配套商品、会员导航必选及真实购买链接',async({adminPage})=>{
    const {ui,detail}=await setup(adminPage);
    const writes:any[]=[];let basicAttempts=0;let savedAddons:any[]=[];
    await adminPage.route('**/admin/catalog/products/*/addons',r=>r.fulfill({json:{code:0,data:{addons:savedAddons}}}));
    await adminPage.route('**/admin/catalog/products/*/description',async r=>{const input=r.request().postDataJSON();writes.push(input);if(++basicAttempts===1)return r.fulfill({json:{code:409,data:{},message:"库存已变化，请检查后重试"}});Object.assign(detail.product,input,{descriptionVersion:3});if(input.inventory)Object.assign(detail.inventory,input.inventory,{version:2});if(input.addonProductNos)savedAddons=input.addonProductNos.map((productNo:string)=>({productNo,nameZhCN:"通配符 SSL 证书",nameEnUS:"SSL"}));await r.fulfill({json:{code:0,data:{version:3}}});});
    await expect(adminPage.getByTestId('catalog-edit-sections').getByRole('tab')).toHaveText(['基本资料','套餐售价','商品配置','代理价格','自动开通']);
    await expect(adminPage.getByTestId('catalog-description-editor')).toContainText('会员导航菜单');
    await expect(adminPage.getByTestId('catalog-inventory-tracking')).toBeVisible();
    await expect(adminPage.getByTestId('catalog-addon-products')).toBeVisible();
    await expect(adminPage.getByTestId('catalog-addons')).toContainText('可选配套商品');
    await expect(ui.detailDrawer).not.toContainText('选择客户可以为本商品');
    await ui.saveBasic();expect(writes).toHaveLength(0);
    await ui.choose('catalog-description-member-menu','云产品');await adminPage.getByTestId('catalog-description-name-zh').fill('手工服务（编辑中）');
    await ui.choose('catalog-addon-products','通配符 SSL 证书');
    await adminPage.getByTestId('catalog-inventory-quantity').fill('31');
    await expect(adminPage.getByTestId('catalog-addons-save')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-inventory-save')).toHaveCount(0);
    await expect(adminPage.getByTestId('catalog-description-save')).toHaveText(/^保\s*存$/);
    const saveBox=await adminPage.getByTestId('catalog-description-save').boundingBox();expect(saveBox!.y).toBeGreaterThan(600);
    await ui.saveBasic();await expect.poll(()=>writes.length).toBe(1);await expect(adminPage.getByTestId('catalog-description-name-zh')).toHaveValue('手工服务（编辑中）');await expect(adminPage.getByTestId('catalog-inventory-quantity')).toHaveValue('31');
    await ui.screenshot('editor-basic-atomic-failed');
    await ui.saveBasic();await expect.poll(()=>writes.length).toBe(2);expect(writes[1]).toMatchObject({directoryNo:'pdr_test',version:1,nameZhCn:'手工服务（编辑中）',inventory:{trackingEnabled:true,sellableQuantity:31,expectedVersion:1},addonProductNos:['product-ssl']});await expect(adminPage.getByTestId('catalog-addon-products')).toContainText('通配符 SSL 证书');
    await expect(adminPage.getByTestId('catalog-description-name-zh')).toHaveValue('手工服务（编辑中）');
    await expect(adminPage.getByTestId('catalog-link-product')).toHaveValue('https://shop.example/configure.html?product='+detail.product.publicSlug);
    await expect(adminPage.getByTestId('catalog-link-group')).toHaveValue('https://shop.example/products.html?category='+detail.product.categoryNo);
    await adminPage.context().grantPermissions(['clipboard-read','clipboard-write']);await adminPage.getByTestId('catalog-link-copy-product').click();expect(await adminPage.evaluate(()=>navigator.clipboard.readText())).toContain('/configure.html?product=');
    await ui.screenshot('editor-basic-direct-fields');
    let added:any;
    await adminPage.route('**/priced-billing-rules',async r=>{added=r.request().postDataJSON();detail.billingRules.push({billingRuleNo:'new-month',ruleCode:added.ruleCode,periodUnit:added.periodUnit,periodCount:added.periodCount,chargeType:'recurring',quantityMode:'fixed',pricePhase:'both',status:'active'});detail.prices.push({priceNo:'new-price',billingRuleNo:'new-month',currencyCode:added.currencyCode,amountMinor:added.amountMinor,setupFeeMinor:added.setupFeeMinor,renewalAmountMinor:added.renewalAmountMinor,minimumQuantity:0,maximumQuantity:null,status:'active'});await r.fulfill({json:{code:0,data:{billingRuleNo:'new-month'}}});});
    await ui.tab('pricing');const addBox=await adminPage.getByTestId('catalog-priced-period-add').boundingBox();const tableBox=await adminPage.getByTestId('catalog-base-price-table').boundingBox();expect(addBox!.y).toBeLessThan(tableBox!.y);await expect(adminPage.getByTestId('catalog-product-costs')).toHaveCount(0);await ui.addPrice();await expect(adminPage.getByRole('dialog').filter({has:adminPage.getByTestId('catalog-priced-billing-form')})).toContainText('添加');await ui.screenshot('editor-add-price-modal');
    await adminPage.getByTestId('catalog-priced-amount').fill('88.00');await adminPage.getByTestId('catalog-priced-renewal').fill('99.00');await adminPage.getByTestId('catalog-priced-save').click();await expect(adminPage.getByTestId('catalog-priced-billing-form')).toBeHidden();expect(added.amountMinor).toBe('8800');expect(added.renewalAmountMinor).toBe('9900');await expect(adminPage.getByTestId(/^catalog-base-price-amount-/).last()).toHaveValue('88.00');
  });
  test('TC-82b: 参数弹窗取消不泄漏，参数与价格同存，失败保留输入后重试',async({adminPage})=>{
    const {ui,configuration}=await setup(adminPage);let attempts=0;const writes:any[]=[];
    await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{
      if(r.request().method()==='PUT'){
        const input=r.request().postDataJSON();writes.push(input);attempts++;
        if(attempts===1)return r.fulfill({json:{code:409,data:{},message:'请检查配置后重试'}});
        configuration.customerFieldSelections=input.customerFieldSelections;configuration.version++;configuration.customerDeclaration.fields=input.customerFieldSelections.map((s:any)=>s.localField);
      }
      return r.fulfill({json:{code:0,data:{configuration}}});
    });
    await ui.tab('configuration');await ui.customParameter();await ui.parameterInput('name_zhCN').fill('放弃的参数');await ui.cancelParameter();await expect(adminPage.getByTestId('catalog-options-overview')).not.toContainText('放弃的参数');expect(writes).toHaveLength(0);
    await ui.customParameter();await ui.parameterInput('name_zhCN').fill('备份保留时间');await ui.addChoice('7','七天');
    const prices=ui.parameterModal.getByTestId('catalog-offer-field-prices');await expect(prices).toBeVisible();await expect(ui.parameterModal).not.toContainText('参数标识');const definitionBox=await adminPage.getByTestId('catalog-parameter-definition').boundingBox();const priceBox=await adminPage.getByTestId('catalog-parameter-pricing').boundingBox();expect(priceBox!.x).toBeGreaterThan(definitionBox!.x+definitionBox!.width);expect(Math.abs(priceBox!.y-definitionBox!.y)).toBeLessThan(10);await prices.getByTestId('catalog-offer-price-amount').first().fill('5.00');await prices.getByTestId('catalog-offer-price-renewal').first().fill('6.00');await prices.getByTestId('catalog-offer-price-setup').first().fill('0.00');
    await ui.saveParameter();await expect.poll(()=>attempts).toBe(1);await expect(ui.parameterInput('name_zhCN')).toHaveValue('备份保留时间');await ui.screenshot('editor-parameter-save-failed');
    await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();expect(writes[1].customerFieldSelections[0].fieldKey).toMatch(/^local_/);expect(writes[1].priceRows[0].fieldKey).toBe(writes[1].customerFieldSelections[0].fieldKey);expect(writes[1].customerFieldSelections[0].localField.staticCandidates[0].value).toBe('7');expect(writes[1].priceRows[0].tiers[0].amountMinor).toBe('500');await expect(adminPage.getByTestId('catalog-options-overview')).toContainText('备份保留时间');await ui.screenshot('editor-parameter-saved');
    await ui.field(writes[1].customerFieldSelections[0].fieldKey).click();await ui.parameterInput('name_zhCN').fill('取消的名称');await ui.cancelParameter();await expect(adminPage.getByTestId('catalog-options-overview')).toContainText('备份保留时间');await expect(adminPage.getByTestId('catalog-options-overview')).not.toContainText('取消的名称');expect(attempts).toBe(2);
  });
  test('TC-82c: 魔方云无转售下拉，上游转售选择和加价重试，导入后配置只读',async({adminPage})=>{
    const {ui,detail,configuration}=await setup(adminPage);let attempts=0;const bodies:any[]=[];
    await adminPage.route('**/admin/catalog/automation/interfaces?*',r=>r.fulfill({json:{code:0,data:{items:[{interfaceInstanceNo:'cloud',displayName:'魔方云',newBusinessState:'accepting',catalogAvailable:false},{interfaceInstanceNo:'finance',displayName:'魔方财务',newBusinessState:'accepting',catalogAvailable:true}],total:2}}}));
    await adminPage.route('**/interfaces/finance/catalog-products?*',r=>r.fulfill({json:{code:0,data:{items:[{reference:'41',name:'上游云主机'}],total:1}}}));
    await adminPage.route('**/interfaces/finance/catalog-product?*',r=>r.fulfill({json:{code:0,data:{item:{product:{reference:'41',name:'上游云主机'},basePrices:[{currencyCode:'CNY',period:'month',periodCount:1,amount:'100.00',setupFee:'0.00'}],options:[],priceScheduleComplete:true}}}}));
    await adminPage.route('**/admin/catalog/product-versions/*/fulfillment-policy',async r=>{
      bodies.push(r.request().postDataJSON());attempts++;if(attempts===1)return r.fulfill({json:{code:409,data:{},message:'上游暂不可用，请重试'}});
      detail.fulfillmentBinding={interfaceInstanceNo:'finance',interfaceDisplayName:'魔方财务',catalogAvailable:true,interfaceNewBusinessState:'accepting'};configuration.importedConfiguration=true;configuration.importedProductReference='41';configuration.importedProductName='上游云主机';configuration.customerDeclaration.fields=[{key:'upstream-memory',type:'single_select',label:{zhCN:'上游内存',enUS:'Upstream memory'},staticCandidates:[{value:'4g',label:{zhCN:'4GB',enUS:'4GB'}}]}];
      return r.fulfill({json:{code:0,data:{}}});
    });
    await ui.tab('fulfillment');await ui.choose('catalog-fulfillment-interface','魔方云');await expect(adminPage.getByTestId('catalog-upstream-selection')).toHaveCount(0);
    await ui.choose('catalog-fulfillment-interface','魔方财务');await ui.choose('catalog-upstream-product','上游云主机');await expect(adminPage.getByTestId('catalog-upstream-pricing-mode')).toContainText('按上游价格加价');await expect(adminPage.getByTestId('catalog-upstream-markup')).toHaveValue('');await ui.saveAutomation();await expect(adminPage.getByTestId('catalog-upstream-selection')).toContainText('请填写加价百分比');expect(attempts).toBe(0);await adminPage.getByTestId('catalog-upstream-markup').fill('12.5');
    await ui.saveAutomation();await expect.poll(()=>attempts).toBe(1);await expect(adminPage.getByTestId('catalog-upstream-markup')).toHaveValue('12.5');await ui.screenshot('editor-upstream-save-failed');await ui.saveAutomation();await expect.poll(()=>attempts).toBe(2);expect(bodies[0].upstreamProduct.requestKey).toBe(bodies[1].upstreamProduct.requestKey);expect(bodies[1].upstreamProduct).toMatchObject({reference:'41',pricingMode:'markup',markupPercent:'12.5'});
    await ui.tab('configuration');await expect(adminPage.getByTestId('catalog-imported-configuration')).toContainText('上游内存');await expect(adminPage.getByTestId('catalog-local-option-add')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-offer-field-prices')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-customer-options-save')).toHaveCount(0);await ui.screenshot('editor-upstream-readonly');await ui.tab('pricing');await expect(adminPage.getByTestId('catalog-priced-period-add')).toHaveCount(0);await expect(adminPage.getByTestId(/^catalog-base-price-amount-/).first()).toBeEnabled();
  });
  test('TC-82d: 服务器登录规则仅在基本资料开关打开时显示，隐藏后保留输入',async({adminPage})=>{
    const {ui,configuration}=await setup(adminPage);
    configuration.productDeclaration.fields=[{key:'hostname_prefix',type:'single_line',label:{zhCN:'主机名前缀',enUS:'Hostname prefix'},maximumLength:64}];
    configuration.merchantValues=[{fieldKey:'hostname_prefix',textValue:'srv'}];
    await adminPage.route('**/configuration/evaluation',async r=>r.fulfill({json:{code:0,data:{evaluation:{valid:true,issues:[],candidates:{},fieldStates:{hostname_prefix:{visible:true,required:false,disabled:false}},values:r.request().postDataJSON().merchantValues}}}}));
    await ui.page.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();await ui.openProductDetail('product-manual-service');
    const toggle=adminPage.getByTestId('catalog-login-settings-toggle');await expect(toggle).toBeVisible();
    const hostname=adminPage.getByRole('textbox',{name:'主机名前缀',exact:true});await expect(hostname).toBeHidden();await toggle.click();await expect(hostname).toBeVisible();await expect(hostname).toHaveValue('srv');await hostname.fill('cloud');await toggle.click();await expect(hostname).toBeHidden();await toggle.click();await expect(hostname).toHaveValue('cloud');await hostname.scrollIntoViewIfNeeded();await ui.screenshot('editor-login-rule-toggle');
    let basicSaved:any;await adminPage.route('**/admin/catalog/products/*/description',async r=>{basicSaved=r.request().postDataJSON();Object.assign(configuration,{version:2,merchantValues:basicSaved.loginRules.values});await r.fulfill({json:{code:0,data:{version:2}}});});
    await ui.choose('catalog-description-member-menu','云产品');await ui.saveBasic();await expect.poll(()=>basicSaved).toBeTruthy();expect(basicSaved.loginRules).toEqual({expectedVersion:1,values:[{fieldKey:'hostname_prefix',textValue:'cloud'}]});
    await ui.tab('fulfillment');await expect(hostname).toBeHidden();await expect(ui.detailDrawer).not.toContainText('选择已经添加的对接上游，并设置何时为客户开通。');
  });

  test('TC-82e: 自定义参数名称可直接输入并保留真实接口绑定',async({adminPage})=>{
    const {ui,detail,configuration}=await setup(adminPage);let saved:any;
    detail.fulfillmentBinding={interfaceInstanceNo:'cloud',interfaceDisplayName:'魔方云',interfaceNewBusinessState:'accepting'};
    configuration.productDeclaration.fields=[{key:'server_port',type:'single_line',label:{zhCN:'服务端口',enUS:'Server port'},maximumLength:64}];
    await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{
      if(r.request().method()==='PUT'){
        saved=r.request().postDataJSON();configuration.version++;configuration.customerFieldSelections=saved.customerFieldSelections;configuration.customerDeclaration.fields=saved.customerFieldSelections.map((s:any)=>s.localField);
        for(const row of saved.priceRows||[]){
          const dimensionNo='dimension-native-'+row.fieldKey;
          configuration.merchantPriceTargets=[{fieldKey:row.fieldKey,candidateValue:row.candidateValue,quantity:row.quantity,pricingMode:row.pricingMode,dimensionNo}];
          detail.prices=detail.prices.filter((price:any)=>price.configurationPriceDimensionNo!==dimensionNo);
          detail.prices.push(...row.tiers.map((tier:any,index:number)=>({...tier,priceNo:'price-native-'+index,billingRuleNo:row.billingRuleNo,currencyCode:row.currencyCode,configurationPriceDimensionNo:dimensionNo,status:'active'})));
        }
      }
      await r.fulfill({json:{code:0,data:{configuration}}});
    });
    await ui.page.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();await ui.openProductDetail('product-manual-service');
    await ui.tab('configuration');await ui.customParameter();await ui.parameterInput('product_field').fill('vendor_profile');await ui.parameterInput('name_zhCN').fill('自定义上游档位');await ui.addChoice('fast','高性能');await ui.parameterModal.getByTestId('catalog-option-child-upstream').fill('500,500,2000,2000');
    await expect(ui.parameterModal.getByRole('switch',{name:'客户表单',exact:true})).toBeChecked();await expect(ui.parameterModal.getByRole('switch',{name:'客户表单',exact:true})).toBeEnabled();await expect(ui.parameterInput('product_field')).not.toHaveAttribute('role','combobox');await expect(ui.parameterModal).not.toContainText('参数标识');await ui.screenshot('editor-native-parameter-input');
    const prices=ui.parameterModal.getByTestId('catalog-offer-field-prices');await prices.getByTestId('catalog-offer-price-amount').first().fill('5.00');await prices.getByTestId('catalog-offer-price-renewal').first().fill('6.00');
    await ui.parameterInput('product_field').fill('vendor_profile_renamed');await ui.saveParameter();await expect.poll(()=>saved).toBeTruthy();expect(saved.customerFieldSelections[0]).toMatchObject({productFieldKey:'vendor_profile_renamed',productChoiceBindings:{fast:'500,500,2000,2000'},localField:{type:'single_select'}});expect(saved.customerFieldSelections[0].fieldKey).toMatch(/^local_/);
    expect(saved.priceRows[0].tiers[0]).toMatchObject({amountMinor:'500',renewalAmountMinor:'600'});const fieldKey=saved.customerFieldSelections[0].fieldKey;
    await expect(ui.parameterModal).toBeHidden();await expect(adminPage.getByTestId('catalog-options-overview')).toContainText('vendor_profile_renamed');await ui.field(fieldKey).click();await expect(ui.parameterInput('product_field')).toHaveValue('vendor_profile_renamed');await expect(ui.parameterModal.getByTestId('catalog-option-child-upstream')).toHaveValue('500,500,2000,2000');await expect(prices.getByTestId('catalog-offer-price-amount').first()).toHaveValue('5.00');await expect(prices.getByTestId('catalog-offer-price-renewal').first()).toHaveValue('6.00');
    await ui.parameterInput('product_field').fill('vendor_profile_final');await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();expect(saved.customerFieldSelections[0]).toMatchObject({fieldKey,productFieldKey:'vendor_profile_final',productChoiceBindings:{fast:'500,500,2000,2000'}});expect(saved.priceRows||[]).toHaveLength(0);
    await ui.field(fieldKey).click();await expect(ui.parameterInput('product_field')).toHaveValue('vendor_profile_final');await expect(ui.parameterModal.getByTestId('catalog-option-child-upstream')).toHaveValue('500,500,2000,2000');await expect(prices.getByTestId('catalog-offer-price-amount').first()).toHaveValue('5.00');await expect(prices.getByTestId('catalog-offer-price-renewal').first()).toHaveValue('6.00');await ui.cancelParameter();
  });
  test('TC-82f: 代理价格无搜索和说明，使用小弹窗保存，失败保留输入',async({adminPage})=>{
    const {ui,detail}=await setup(adminPage);let saved:any;let attempts=0;
    await adminPage.route('**/admin/sales-management/prices?*',r=>r.fulfill({json:{code:0,data:{items:saved?[{...saved,priceNo:'price-agent',version:1,levelName:'银牌代理',productName:'人工迁移服务'}]:[],total:saved?1:0}}}));
    await adminPage.route('**/admin/sales-management/price-level-candidates?*',r=>r.fulfill({json:{code:0,data:{items:[{levelNo:'level-silver',name:'银牌代理',status:'active'}],total:1}}}));
    await adminPage.route('**/admin/sales-management/price-product-candidates?*',r=>r.fulfill({json:{code:0,data:{items:[{productNo:detail.product.productNo,name:'人工迁移服务',englishName:'Manual service',terms:[{currencyCode:'CNY',chargeTypeCode:'recurring',periodUnitCode:'month',periodCount:1}]}],currencies:['CNY'],total:1}}}));
    await adminPage.route('**/admin/sales-management/prices',async r=>{const body=r.request().postDataJSON();if(++attempts===1)return r.fulfill({json:{code:409,data:{},message:'价格保存失败，请重试'}});saved=body;await r.fulfill({json:{code:0,data:{item:{...body,priceNo:'price-agent',version:1}}}});});
    await ui.tab('agents');await expect(adminPage.getByTestId('prices-workspace')).not.toContainText('设置这个商品对各代理等级');await expect(adminPage.getByTestId('prices-workspace').getByRole('textbox',{name:'搜索',exact:true})).toHaveCount(0);
    await ui.openAgentPrice();await expect(ui.agentPriceDialog).toContainText('添加代理价格');expect((await ui.agentPriceDialog.boundingBox())!.width).toBeLessThan(700);
    await ui.choose('sales-field-levelNo','银牌代理');await ui.agentPriceDialog.getByTestId('sales-field-amount').fill('12.34');await ui.saveAgentPrice();await expect.poll(()=>attempts).toBe(1);await expect(ui.agentPriceDialog.getByTestId('sales-field-amount')).toHaveValue('12.34');await ui.screenshot('editor-agent-price-failed');
    await ui.saveAgentPrice();await expect(ui.agentPriceDialog).toBeHidden();expect(saved).toMatchObject({productNo:detail.product.productNo,levelNo:'level-silver',fixedPriceMinor:'1234'});await expect(adminPage.getByTestId('prices-workspace')).toContainText('银牌代理');await ui.screenshot('editor-agent-price-saved');await ui.page.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();
  });

  test('TC-82g: 数量阶梯费用操作不截断，取消不写入',async({adminPage})=>{
    const {ui}=await setup(adminPage);let writes=0;
    await adminPage.route('**/admin/catalog/product-versions/*/configuration',r=>{if(r.request().method()==='PUT')writes++;return r.fallback();});
    await ui.tab('configuration');await ui.customParameter();await ui.choose('catalog-local-option-type','数量／规格／滑块');await ui.addPriceBand('2');
    await expect(ui.parameterModal.getByTestId('catalog-offer-price-upper')).toHaveCount(2);expect(await ui.clippedPriceActions()).toBe(0);await ui.screenshot('editor-price-bands-layout');
    await ui.cancelParameter();expect(writes).toBe(0);await expect(adminPage.getByTestId('catalog-options-overview')).not.toContainText('自定义配置');
  });

  test('TC-82h: 配置表格显式编辑、直接排序、升降级开关与范围保存并恢复',async({adminPage})=>{
    const {ui,configuration}=await setup(adminPage);let attempts=0;const writes:any[]=[];
    await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{
      if(r.request().method()==='PUT'){
        const input=r.request().postDataJSON();writes.push(input);attempts++;
        if(attempts===2)return r.fulfill({json:{code:409,data:{},message:'排序保存失败，请重试'}});
        configuration.customerFieldSelections=input.customerFieldSelections;configuration.customerDeclaration.fields=input.customerFieldSelections.map((s:any)=>s.localField);configuration.version++;
      }
      await r.fulfill({json:{code:0,data:{configuration}}});
    });
    await ui.tab('configuration');await ui.customParameter();await ui.choose('catalog-local-option-type','数量／规格／滑块');await ui.parameterInput('name_zhCN').fill('可升级容量');
    await ui.parameterInput('minimum').fill('0');await ui.parameterInput('maximum').fill('16');await ui.parameterInput('upgrade_minimum').fill('2');await ui.parameterInput('upgrade_maximum').fill('8');await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();
    const key=writes[0].customerFieldSelections[0].fieldKey;
    expect(writes[0].customerFieldSelections[0]).toMatchObject({upgradeEnabled:true,upgradeMinimum:'2',upgradeMaximum:'8'});
    await expect(ui.field(key)).toContainText('编辑');await expect(adminPage.getByTestId('catalog-options-overview').getByRole('button',{name:/^编\s*辑$/})).toBeVisible();
    await ui.sortParameter(key,'7');await ui.saveTable();await expect.poll(()=>attempts).toBe(2);await expect(ui.sortInput(key)).toHaveValue('7');await ui.screenshot('editor-sort-failed');
    await ui.sortParameter(key,'8');await ui.saveTable();await expect.poll(()=>attempts).toBe(3);expect(writes[2].customerFieldSelections[0].sortOrder).toBe(8);await expect(ui.sortInput(key)).toHaveValue('8');
    await ui.toggleUpgrade(key);await ui.saveTable();await expect.poll(()=>attempts).toBe(4);expect(writes[3].customerFieldSelections[0].upgradeEnabled).toBe(false);await expect(adminPage.getByTestId(`catalog-option-upgrade-${key}`)).toHaveAttribute('aria-checked','false');
    await ui.field(key).click();await expect(ui.parameterInput('upgrade_minimum')).toBeHidden();await ui.parameterModal.locator('[data-testid$="_upgrade_enabled"]').click();await expect(ui.parameterInput('upgrade_minimum')).toHaveValue('2');await expect(ui.parameterInput('upgrade_maximum')).toHaveValue('8');await ui.screenshot('editor-upgrade-controls');await ui.cancelParameter();
    await ui.customParameter();await ui.parameterInput('name_zhCN').fill('可升级档位');await ui.addChoice('basic','基础档');await ui.addChoice('fast','高性能档');
    await ui.chooseParameter('upgrade_choices','高性能档');await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();
    expect(writes[4].customerFieldSelections.find((item:any)=>item.displayNameZhCN==='可升级档位').upgradeCandidateValues).toEqual(['fast']);

  });

  test('TC-82i: 已有接口选项在配置编辑和费用区一致显示并保存',async({adminPage})=>{
    const {ui,detail,configuration}=await setup(adminPage);let saved:any;
    detail.fulfillmentBinding={interfaceInstanceNo:'cloud',interfaceDisplayName:'魔方云',interfaceNewBusinessState:'accepting'};
    const field={key:'operating_system',type:'single_select',required:true,label:{zhCN:'操作系统',enUS:'Operating system'},staticCandidates:[]};
    const choices=[{value:'linux',label:{zhCN:'Linux 系统',enUS:'Linux'}},{value:'windows',label:{zhCN:'Windows 系统',enUS:'Windows'}}];
    Object.assign(configuration,{customerDeclaration:{version:'v1',fields:[field]},customerConfigurableFields:[field],customerFieldSelections:[],customerResolvedCandidates:{operating_system:choices}});
    await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{
      if(r.request().method()==='PUT'){saved=r.request().postDataJSON();configuration.version++;configuration.customerFieldSelections=saved.customerFieldSelections;}
      await r.fulfill({json:{code:0,data:{configuration}}});
    });
    await ui.page.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();await ui.openProductDetail('product-manual-service');await ui.tab('configuration');await ui.field('operating_system').click();
    await expect(ui.parameterModal.getByTestId('catalog-option-child-0')).toContainText('Linux 系统');await expect(ui.parameterModal.getByTestId('catalog-option-child-1')).toContainText('Windows 系统');
    await expect(ui.parameterModal.getByTestId('catalog-offer-field-prices')).toContainText('Linux 系统');await expect(ui.parameterModal).not.toContainText('尚未添加子项');
    await ui.screenshot('editor-effective-options');await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();expect(saved.customerFieldSelections[0].candidateValues).toEqual(['linux','windows']);
  });

  test('TC-82j: 接口必填配置可隐藏并保存默认值，排序输入失焦后才移动',async({adminPage})=>{
    const choices=[{value:'linux',label:{zhCN:'Linux 系统',enUS:'Linux'}},{value:'windows',label:{zhCN:'Windows 系统',enUS:'Windows'}}];
    const field={key:'operating_system',type:'single_select',required:true,label:{zhCN:'操作系统',enUS:'Operating system'},staticCandidates:choices};
    const {ui,configuration}=await setup(adminPage,(detail,configuration)=>{
      detail.fulfillmentBinding={interfaceInstanceNo:'cloud',interfaceDisplayName:'魔方云',interfaceNewBusinessState:'accepting'};
      Object.assign(configuration,{customerDeclaration:{version:'v1',fields:[field]},customerConfigurableFields:[field],customerResolvedCandidates:{operating_system:choices}});
    });const writes:any[]=[];
    await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{
      if(r.request().method()==='PUT'){const input=r.request().postDataJSON();writes.push(input);configuration.version++;configuration.customerFieldSelections=input.customerFieldSelections;}
      await r.fulfill({json:{code:0,data:{configuration}}});
    });
    await ui.tab('configuration');await ui.field('operating_system').click();
    const visibility=ui.parameterModal.getByRole('switch',{name:'客户表单',exact:true});await expect(visibility).toBeEnabled();await expect(ui.parameterModal).not.toContainText('客户必须填写');await visibility.click();await expect(visibility).not.toBeChecked();await ui.chooseParameter('default','Windows 系统');await ui.screenshot('editor-hidden-default');
    await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();expect(writes[0].customerFieldSelections[0]).toMatchObject({fieldKey:'operating_system',customerVisible:false,defaultValue:'windows'});
    await expect(adminPage.getByTestId('catalog-options-overview')).toContainText('隐藏');await ui.field('operating_system').click();await expect(visibility).not.toBeChecked();await expect(ui.parameterInput('default')).toContainText('Windows 系统');await ui.cancelParameter();
    await ui.customParameter();await ui.parameterInput('name_zhCN').fill('附加档位');await ui.addChoice('standard','标准');await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();
    const key=writes[1].customerFieldSelections.find((row:any)=>row.localField).fieldKey;
    const names=adminPage.getByTestId('catalog-options-overview').getByTestId(/^catalog-option-tab-/);const before=await names.allTextContents();
    await ui.sortInput('operating_system').fill('9');await expect(names).toHaveText(before);expect(writes).toHaveLength(2);await ui.sortInput('operating_system').press('Tab');await expect(names.first()).toHaveAttribute('data-testid',`catalog-option-tab-${key}`);await ui.saveTable();await expect.poll(()=>writes.length).toBe(3);expect(writes[2].customerFieldSelections.find((row:any)=>row.fieldKey==='operating_system').sortOrder).toBe(9);
  });

  test('TC-82k: 购买入口读取失败只在表单提示并可重新读取',async({adminPage})=>{
    const {ui}=await setup(adminPage);
    await ui.page.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();
    let reads=0;
    await adminPage.route('**/public/customer-center/configuration',r=>{expect(new URL(r.request().url()).pathname).toBe('/x/idc-portal/api/v1/public/customer-center/configuration');reads++;return reads===1?r.fulfill({status:503,json:{code:503,message:'入口读取暂不可用'}}):r.fulfill({json:{code:0,data:{config:{websiteurl:'https://shop.example',websiteconfigurepath:'/configure.html?product={product}',websitecatalogpath:'/products.html'}}}});});
    await ui.openProductDetail('product-manual-service');const links=adminPage.getByTestId('catalog-purchase-links');
    await expect.poll(()=>reads).toBe(1);await expect(links).toContainText('购买入口读取失败，请重新读取后再复制链接。');await expect(adminPage.locator('.ant-notification-notice').filter({hasText:'入口读取暂不可用'})).toHaveCount(0);
    await links.getByRole('button',{name:'重新读取',exact:true}).click();await expect(adminPage.getByTestId('catalog-link-product')).toHaveValue(/shop\.example/);expect(reads).toBe(2);
  });

  test('TC-82l: 批量设置选项价格仅填草稿，复制配置定义不覆盖原项或重复绑定接口',async({adminPage})=>{
    const {ui,configuration}=await setup(adminPage);const writes:any[]=[];
    await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{
      if(r.request().method()==='PUT'){const input=r.request().postDataJSON();writes.push(input);configuration.version++;configuration.customerFieldSelections=input.customerFieldSelections;configuration.customerDeclaration.fields=input.customerFieldSelections.map((row:any)=>row.localField);}
      await r.fulfill({json:{code:0,data:{configuration}}});
    });
    await ui.tab('configuration');await ui.customParameter();await ui.parameterInput('name_zhCN').fill('备份等级');await ui.addChoice('basic','基础档');await ui.addChoice('fast','高性能');
    await ui.batchOptionPrices(['基础档 · 1 年','高性能 · 1 年'],'bad','6.00');await expect(ui.optionPriceBatch).toContainText('请选择至多 256 个子项');expect(writes).toHaveLength(0);
    await ui.optionPriceBatch.getByTestId('catalog-option-price-batch-amount').fill('5.00');await ui.optionPriceBatch.getByRole('button',{name:'填入草稿',exact:true}).click();await expect(ui.optionPriceBatch).toBeHidden();
    const prices=ui.parameterModal.getByTestId('catalog-offer-field-prices');await expect(prices.getByTestId('catalog-offer-price-amount')).toHaveCount(2);await expect(prices.getByTestId('catalog-offer-price-amount').first()).toHaveValue('5.00');await expect(prices.getByTestId('catalog-offer-price-amount').last()).toHaveValue('5.00');expect(writes).toHaveLength(0);await ui.screenshot('editor-option-price-batch-draft');
    await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();expect(writes[0].priceRows).toHaveLength(2);for(const price of writes[0].priceRows) expect(price.tiers[0]).toMatchObject({amountMinor:'500',renewalAmountMinor:'600'});
    const key=writes[0].customerFieldSelections[0].fieldKey;await ui.copyDefinition(key);await expect(ui.parameterInput('name_zhCN')).toHaveValue('备份等级（副本）');await expect(prices.getByTestId('catalog-offer-price-amount').first()).toHaveValue('');await ui.cancelParameter();await expect(adminPage.getByTestId('catalog-options-overview')).not.toContainText('备份等级（副本）');expect(writes).toHaveLength(1);
    await ui.copyDefinition(key);await ui.batchOptionPrices(['基础档 · 1 年','高性能 · 1 年'],'8.00','9.00');await expect(ui.optionPriceBatch).toBeHidden();await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();
    const copied=writes[1].customerFieldSelections.find((row:any)=>row.fieldKey!==key);expect(copied.localField.staticCandidates.map((row:any)=>row.value)).toEqual(['basic','fast']);expect(copied.productFieldKey).toBeUndefined();expect(writes[1].priceRows.every((price:any)=>price.fieldKey===copied.fieldKey&&price.tiers[0].amountMinor==='800')).toBe(true);await expect(adminPage.getByTestId('catalog-options-overview')).toContainText('备份等级（副本）');
  });

  test('TC-82m: 实时上游无固定基础价时仍可按周期自定义整件价，导入配置只读',async({adminPage})=>{
    const {ui,detail,configuration}=await setup(adminPage);const writes:any[]=[];let configurationWrites=0;
    adminPage.on('request',request=>{if(request.method()==='PUT'&&new URL(request.url()).pathname.endsWith('/configuration'))configurationWrites++;});
    await adminPage.route('**/admin/catalog/automation/interfaces?*',r=>r.fulfill({json:{code:0,data:{items:[{interfaceInstanceNo:'remote',displayName:'魔方业务 V10',newBusinessState:'accepting',catalogAvailable:true}],total:1}}}));
    await adminPage.route('**/interfaces/remote/catalog-products?*',r=>r.fulfill({json:{code:0,data:{items:[{reference:'91',name:'上游实时云主机'}],total:1}}}));
    await adminPage.route('**/interfaces/remote/catalog-product?*',r=>r.fulfill({json:{code:0,data:{item:{product:{reference:'91',name:'上游实时云主机'},basePrices:[],options:[{name:'上游配置'}],priceScheduleComplete:false,
      importSnapshot:{protocolVersion:'3',pricingMode:'remote_evaluation',prices:[{currencyCode:'CNY',chargeType:'recurring',periodUnit:'month',periodCount:3},{currencyCode:'USD',chargeType:'one_time',periodUnit:'once',periodCount:1}]},
      peerPurchase:{configurationMode:'remote_evaluation',billingTerms:[{currencyCode:'CNY',terms:[{chargeType:'recurring',periodUnit:'month',periodCount:3,subtotalAmount:'10001',setupAmount:'123'}]},{currencyCode:'USD',terms:[{chargeType:'one_time',periodUnit:'once',periodCount:1,subtotalAmount:'4500',setupAmount:'0'}]}]},
    }}}}));
    await adminPage.route('**/admin/catalog/product-versions/*/fulfillment-policy',r=>{
      writes.push(r.request().postDataJSON());
      detail.fulfillmentBinding={interfaceInstanceNo:'remote',interfaceDisplayName:'魔方业务 V10',catalogAvailable:true,interfaceNewBusinessState:'accepting'};
      configuration.importedConfiguration=true;configuration.importedProductReference='91';configuration.importedProductName='上游实时云主机';configuration.customerDeclaration.fields=[{key:'remote-memory',type:'single_select',label:{zhCN:'上游内存',enUS:'Supplier memory'},staticCandidates:[{value:'4g',label:{zhCN:'4GB',enUS:'4GB'}}]}];
      return r.fulfill({json:{code:0,data:{}}});
    });
    await ui.tab('fulfillment');await ui.choose('catalog-fulfillment-interface','魔方业务 V10');await ui.choose('catalog-upstream-product','上游实时云主机');
    await expect(adminPage.getByTestId('catalog-upstream-price-samples')).toContainText('上游当前默认配置示例价');await expect(adminPage.getByTestId('catalog-upstream-price-samples')).toContainText('CNY · 3 月');
    await expect(adminPage.getByTestId('catalog-upstream-selection')).not.toContainText('暂时无法保存此商品');
    await expect(adminPage.getByTestId('catalog-upstream-price-samples')).toContainText('100.01 CNY');
    await expect(adminPage.getByTestId('catalog-upstream-price-samples')).toContainText('45.00 USD');
    await ui.choose('catalog-upstream-pricing-mode','自定义售价');
    await expect(adminPage.getByTestId('catalog-upstream-amount-0')).toHaveValue('100.01');await expect(adminPage.getByTestId('catalog-upstream-setup-0')).toHaveValue('1.23');
    await expect(adminPage.getByTestId('catalog-upstream-amount-1')).toHaveValue('45.00');
    await adminPage.getByTestId('catalog-upstream-amount-0').fill('88.88');await adminPage.getByTestId('catalog-upstream-setup-0').fill('2.22');
    await adminPage.getByTestId('catalog-upstream-amount-1').fill('55.55');await adminPage.getByTestId('catalog-upstream-setup-1').fill('3.33');
    await ui.screenshot('editor-remote-whole-price');await ui.saveAutomation();await expect.poll(()=>writes.length).toBe(1);
    expect(writes[0].upstreamProduct).toMatchObject({reference:'91',pricingMode:'manual',markupPercent:'',prices:[{currencyCode:'CNY',period:'month',periodCount:3,amount:'88.88',setupFee:'2.22'},{currencyCode:'USD',period:'once',periodCount:1,amount:'55.55',setupFee:'3.33'}]});
    await ui.tab('configuration');await expect(adminPage.getByTestId('catalog-imported-configuration')).toContainText('上游内存');await expect(adminPage.getByTestId('catalog-local-option-add')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-offer-field-prices')).toHaveCount(0);expect(configurationWrites).toBe(0);
  });

  test('TC-82n: 已导入商品回显保存规则且不误dirty，可仅改百分比并保留手填或混合规则',async({adminPage})=>{
    const {ui,detail,configuration}=await setup(adminPage,(detail,configuration)=>{
      detail.fulfillmentBinding={interfaceInstanceNo:'remote',interfaceDisplayName:'魔方业务 V10',catalogAvailable:true,interfaceNewBusinessState:'accepting'};
      detail.fulfillmentPolicy={strategyCode:'payment_confirmed_plugin'};
      configuration.importedConfiguration=true;configuration.importedProductReference='91';configuration.importedProductName='已导入实时商品';configuration.importedPricingMode='markup';configuration.importedMarkupPercent='37.5';configuration.importedManualPrices=[];
      configuration.customerDeclaration.fields=[{key:'remote-memory',type:'single_select',label:{zhCN:'上游内存',enUS:'Supplier memory'},staticCandidates:[{value:'4g',label:{zhCN:'4GB',enUS:'4GB'}}]}];
    });
    const writes:any[]=[];let configurationWrites=0;let sourceReads=0;const original=JSON.stringify(configuration.customerDeclaration);
    adminPage.on('request',request=>{if(request.method()==='PUT'&&new URL(request.url()).pathname.endsWith('/configuration'))configurationWrites++;});
    await adminPage.route('**/admin/catalog/automation/interfaces?*',r=>r.fulfill({json:{code:0,data:{items:[{interfaceInstanceNo:'remote',displayName:'魔方业务 V10',newBusinessState:'accepting',catalogAvailable:true}],total:1}}}));
    await adminPage.route('**/interfaces/remote/catalog-products?*',r=>r.fulfill({json:{code:0,data:{items:[{reference:'91',name:'已导入实时商品'}],total:1}}}));
    await adminPage.route('**/interfaces/remote/catalog-product?*',r=>{if(++sourceReads===1)return r.fulfill({json:{code:409,data:{},message:'上游读取失败'}});return r.fulfill({json:{code:0,data:{item:{product:{reference:'91',name:'已导入实时商品'},basePrices:[],options:[],priceScheduleComplete:false,
      importSnapshot:{protocolVersion:'3',pricingMode:'remote_evaluation',prices:[{currencyCode:'CNY',chargeType:'recurring',periodUnit:'month',periodCount:1}]},
      peerPurchase:{configurationMode:'remote_evaluation',billingTerms:[{currencyCode:'CNY',terms:[{chargeType:'recurring',periodUnit:'month',periodCount:1,subtotalAmount:'10001',setupAmount:'123'}]}]},
    }}}});});
    await adminPage.route('**/admin/catalog/product-versions/*/fulfillment-policy',r=>{writes.push(r.request().postDataJSON());configuration.importedMarkupPercent='42.5';return r.fulfill({json:{code:0,data:{}}});});
    await ui.tab('fulfillment');await expect(adminPage.getByTestId('catalog-upstream-selection')).toContainText('上游商品读取失败');await adminPage.getByTestId('catalog-upstream-selection').getByRole('button',{name:'重新读取',exact:true}).click();await expect(adminPage.getByTestId('catalog-upstream-markup')).toHaveValue('37.5');await expect(adminPage.getByTestId('catalog-unsaved-changes')).toBeHidden();
    await adminPage.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();await expect(adminPage.getByRole('dialog').filter({hasText:'有修改尚未保存'})).toHaveCount(0);
    await ui.openProductDetail('product-manual-service');await ui.tab('fulfillment');await expect(adminPage.getByTestId('catalog-upstream-markup')).toHaveValue('37.5');
    await adminPage.getByTestId('catalog-upstream-markup').fill('42.50');await expect(adminPage.getByTestId('catalog-unsaved-changes')).toContainText('自动开通');
    await ui.saveAutomation();await expect.poll(()=>writes.length).toBe(1);expect(writes[0].upstreamProduct).toMatchObject({reference:'91',pricingMode:'markup',markupPercent:'42.50',prices:[]});
    await expect(adminPage.getByTestId('catalog-unsaved-changes')).toBeHidden();await ui.tab('configuration');await expect(adminPage.getByTestId('catalog-imported-configuration')).toContainText('上游内存');await expect(adminPage.getByTestId('catalog-local-option-add')).toHaveCount(0);expect(configurationWrites).toBe(0);expect(JSON.stringify(configuration.customerDeclaration)).toBe(original);
    await adminPage.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();
    configuration.importedPricingMode='manual';configuration.importedMarkupPercent='';configuration.importedManualPrices=[{currencyCode:'CNY',period:'month',periodCount:1,amount:'125.50',setupFee:'1.50'}];
    await ui.openProductDetail('product-manual-service');await ui.tab('fulfillment');await expect(adminPage.getByTestId('catalog-upstream-amount-0')).toHaveValue('125.50');await expect(adminPage.getByTestId('catalog-upstream-setup-0')).toHaveValue('1.50');await expect(adminPage.getByTestId('catalog-unsaved-changes')).toBeHidden();await ui.screenshot('editor-imported-saved-policy');
    await adminPage.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();
    configuration.importedPricingMode='mixed';
    await ui.openProductDetail('product-manual-service');await ui.tab('fulfillment');await expect(adminPage.getByTestId('catalog-upstream-pricing-mode')).toContainText('按周期分别设置');await expect(adminPage.getByTestId('catalog-upstream-mixed-pricing')).toBeVisible();await expect(adminPage.getByTestId('catalog-upstream-markup')).toHaveCount(0);await expect(adminPage.getByTestId('catalog-unsaved-changes')).toBeHidden();
    await adminPage.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();expect(writes).toHaveLength(1);
  });

  test('TC-82o: 插件说明更新后重读既有配置，保留规格、商家说明和价格且不自动保存',async({adminPage})=>{
    const {ui,configuration}=await setup(adminPage,(_detail,configuration)=>{
      const field={key:'cpu',type:'integer',label:{zhCN:'处理器核心数',enUS:'CPU cores'},minimum:'1',maximum:'4',description:{zhCN:'留空使用套餐默认配置',enUS:'Leave blank for package size'}};
      Object.assign(configuration,{customerDeclaration:{version:'v1',fields:[field]},customerConfigurableFields:[field],customerFieldSelections:[{fieldKey:'cpu',minimum:'1',maximum:'4',displayNameZhCN:'处理器核心数',displayNameEnUS:'CPU cores',descriptionZhCN:'选择需要的核心数',descriptionEnUS:'Choose the required cores',numericChoices:['1','2','4'],displayControl:'radio'}]});
    });
    await adminPage.getByTestId('catalog-editor-back').click();await expect(ui.detailDrawer).toBeHidden();
    let reads=0,writes=0;
    const saved=JSON.stringify(configuration);
    await adminPage.route('**/admin/catalog/product-versions/*/configuration',r=>{
      if(r.request().method()!=='GET') {writes++;return r.fulfill({status:500,json:{code:500,message:'本测试禁止配置写入'}});}
      if(++reads===1)return r.fulfill({json:{code:409,message:'当前无法验证自动化插件的发布版本、履约扩展点、契约或可用方法，请检查插件状态后重试'}});
      return r.fulfill({json:{code:0,data:{configuration}}});
    });
    await ui.openProductDetail('product-manual-service');await ui.tab('configuration');
    await expect(adminPage.getByTestId('catalog-settings-failed')).toContainText('本地配置读取失败，商品资料和售价仍可维护，请重新读取。');
    await ui.screenshot('editor-description-update-before-retry');
    await ui.retryConfiguration();await expect(adminPage.getByTestId('catalog-settings-failed')).toBeHidden();
    await expect(adminPage.getByText('当前无法验证自动化插件的发布版本、履约扩展点、契约或可用方法，请检查插件状态后重试',{exact:true})).toBeHidden();
    await expect(adminPage.getByTestId('catalog-options-overview')).toContainText('处理器核心数');await ui.screenshot('editor-description-update-configuration');await ui.field('cpu').click();
    await expect(ui.parameterInput('minimum')).toHaveValue('1');await expect(ui.parameterInput('maximum')).toHaveValue('4');
    await ui.cancelParameter();
    await ui.tab('pricing');await expect(adminPage.getByTestId(/^catalog-base-price-amount-/).first()).toHaveValue('199.00');
    expect(reads).toBe(2);expect(writes).toBe(0);expect(JSON.stringify(configuration)).toBe(saved);await ui.screenshot('editor-description-update-restored');
  });
});
