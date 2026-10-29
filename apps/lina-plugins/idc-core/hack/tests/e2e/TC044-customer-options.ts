import { test, expect } from '@host-tests/fixtures/auth';
import type { Page } from '@host-tests/support/playwright';
import { installCatalogManagementMock, mockProductDetail } from '../support/catalog-mock';
import { CustomerOptionsPage } from '../pages/CustomerOptionsPage';

async function inlineOptionsFixture(page:Page, changeStoredConfiguration=false) {
 await installCatalogManagementMock(page);
 const field={key:'cpu',type:'integer',label:{zhCN:'处理器核心数',enUS:'CPU cores'},minimum:'1',maximum:'1024'};
 const selection={fieldKey:'cpu',minimum:'2',maximum:'16',displayNameZhCN:'已保存的规格名',displayNameEnUS:'Saved specification',displayControl:'input',quantityStep:'1'};
 const configuration:any={version:1,merchantValues:[],customerConfigurableFields:[field],customerFieldSelections:[selection],customerDeclaration:{version:'v1',fields:[{...field,minimum:'2',maximum:'16'}]},productResolvedCandidates:{},customerResolvedCandidates:{}};
 const detail:any=mockProductDetail('product-manual-service');detail.product.descriptionVersion=1;
 detail.fulfillmentBinding={evidenceVerified:true,interfaceNewBusinessState:'accepting',interfaceDisplayName:'魔方云',interfaceInstanceNo:'interface-test'};
 const writes:Array<{path:string;body:any}>=[];
 await page.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
 await page.route('**/admin/catalog/products/product-manual-service/description',async r=>{
  Object.assign(detail.product,r.request().postDataJSON(),{descriptionVersion:2});detail.version.versionNo='version-after-basic-save';configuration.version=2;
  if(changeStoredConfiguration) configuration.customerFieldSelections[0].maximum='32';
  return r.fulfill({json:{code:0,data:{version:2}}});
 });
 await page.route('**/admin/catalog/product-versions/*/configuration',r=>{
  if(r.request().method()==='PUT') {const body=r.request().postDataJSON();writes.push({path:new URL(r.request().url()).pathname,body});configuration.customerFieldSelections=body.customerFieldSelections;configuration.version++;}
  return r.fulfill({json:{code:0,data:{configuration}}});
 });
 return {writes};
}

test.describe('TC-44 商品客户可选配置', () => {
  test('TC-44a: 默认关闭、明确范围、保存失败保留与重试', async ({adminPage}) => {
    test.setTimeout(45000);

    await installCatalogManagementMock(adminPage);
    const field = {key:'cpu',type:'integer',label:{enUS:'CPU cores',zhCN:'处理器核心数'},minimum:'1',maximum:'1024'};
    const configuration = {version:1,merchantValues:[],customerConfigurableFields:[field],customerFieldSelections:[],customerDeclaration:{version:'v1',fields:[]},productResolvedCandidates:{},customerResolvedCandidates:{}};
    await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({json:{code:0,message:'success',data:{product:{productNo:'product-manual-service',nameZhCn:'云服务器',nameEnUs:'Server',status:'draft'},version:{versionNo:'version-manual-service-1',status:'draft',source:'manual',versionNumber:1},billingRules:[],prices:[],taxPolicies:[],inventory:{trackingEnabled:false},fulfillmentBinding:{evidenceVerified:true,interfaceNewBusinessState:'accepting',interfaceDisplayName:'魔方云',interfaceInstanceNo:'interface-test',pluginId:'idc-zjmfcloud',pluginReleaseVersion:'v0.2.7'},fulfillmentPolicy:null}}}));
    const writes: any[] = [];
    await adminPage.route('**/admin/catalog/product-versions/*/configuration', async route => {
      if(route.request().method()==='GET') { await route.fulfill({json:{code:0,message:'success',data:{configuration}}}); return; }

      if(route.request().method()==='PUT') {
        writes.push(route.request().postDataJSON());
        if(writes.length===1) {await route.fulfill({json:{code:500,message:'保存失败，请重试',data:null}});return;}
        Object.assign(configuration,{customerFieldSelections:writes.at(-1).customerFieldSelections,customerDeclaration:{version:'v1',fields:[{...field,minimum:'2',maximum:'16'}]},version:2});
      }
      await route.fulfill({json:{code:0,message:'success',data:{configuration}}});
    });
    const workspace=new CustomerOptionsPage(adminPage);
    await workspace.goto();await workspace.openProductDetail('product-manual-service');await workspace.openOptions();
    await expect(adminPage.getByTestId('option_0_enabled')).toHaveCount(0);
    await workspace.addParameter('处理器核心数');
    await expect(adminPage.getByTestId('option_0_enabled')).toHaveAttribute('aria-checked','true');
    await expect(adminPage.getByTestId('option_0_minimum')).toBeVisible();
    await expect(workspace.optionsForm).toContainText('免费也要填0');
    await workspace.confirmOptions();
    expect(writes).toHaveLength(0);
    await workspace.setCPUBounds('2','16');
    await expect(workspace.optionsForm).toContainText('前台展示方式');
    for(const label of ['数字输入框','滑块','下拉选择','单选按钮']) {await workspace.selectCPUControl(label);await expect(adminPage.getByTestId('option_0_control')).toContainText(label);}
    await workspace.confirmOptions();expect(writes).toHaveLength(0);
    await workspace.setCPUChoices('2','2, 4, 8, 16');
    await workspace.screenshot('customer-options-ranges');await workspace.confirmOptions();
    await expect(workspace.optionsForm).toBeVisible();
    await expect(adminPage.getByTestId('option_0_maximum')).toHaveValue('16');
    await workspace.confirmOptions();await expect(adminPage.getByText('客户可选配置已保存',{exact:true})).toBeVisible();await expect(workspace.optionsForm).toBeVisible();
    expect(writes).toHaveLength(2);
    expect(writes.at(-1)).toMatchObject({expectedVersion:1,merchantValues:[],customerFieldSelections:[{fieldKey:'cpu',minimum:'2',maximum:'16',displayControl:'radio',quantityStep:'2',numericChoices:['2','4','8','16']}]});
    await workspace.reopenOptions();
    await expect(adminPage.getByTestId('option_0_enabled')).toHaveAttribute('aria-checked','true');
    await expect(adminPage.getByTestId('option_0_maximum')).toHaveValue('16');
    await expect(adminPage.getByTestId('option_0_control')).toContainText('单选按钮');
    await expect(adminPage.getByTestId('option_0_step')).toHaveValue('2');
    await expect(adminPage.getByTestId('option_0_numbers')).toHaveValue('2|2|2, 4|4|4, 8|8|8, 16|16|16');
    await workspace.screenshot('customer-options-saved');
  });
 test('TC-44b: 下拉子项替代旧固定范围，错误值明确提示并保留输入',async({adminPage})=>{
  test.setTimeout(45000);
  await installCatalogManagementMock(adminPage);
  const field={key:'memory',type:'integer',minimum:'1',maximum:'1024',label:{zhCN:'内存（GiB）',enUS:'Memory (GiB)'}};
  const core={key:'os',type:'single_select',required:true,label:{zhCN:'操作系统',enUS:'Operating system'},staticCandidates:[{value:'linux',label:{zhCN:'Linux',enUS:'Linux'}}]};
  const configuration:any={version:4,merchantValues:[],customerConfigurableFields:[core,field],customerFieldSelections:[{fieldKey:'memory',minimum:'1024',maximum:'1024',displayControl:'select',quantityStep:'1',numericChoices:['1024']}],customerDeclaration:{version:'v1',fields:[core,{...field,minimum:'1024',maximum:'1024'}]},productResolvedCandidates:{},customerResolvedCandidates:{}};
  const writes:any[]=[];let delayedRefresh=false;
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:{product:{productNo:'product-manual-service',nameZhCn:'云服务器',nameEnUs:'Server',status:'draft'},version:{versionNo:'version-manual-service-1',status:'draft',source:'manual',versionNumber:1},billingRules:[],prices:[],taxPolicies:[],inventory:{trackingEnabled:false},fulfillmentBinding:{evidenceVerified:true,interfaceNewBusinessState:'accepting',interfaceInstanceNo:'interface-test',interfaceDisplayName:'魔方云'},fulfillmentPolicy:null}}}));
  await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{if(r.request().method()==='GET'&&writes.length&&!delayedRefresh){delayedRefresh=true;await new Promise(resolve=>setTimeout(resolve,750));}if(r.request().method()==='PUT'){const body=r.request().postDataJSON();writes.push(body);expect(body.customerFieldSelections.find((item:any)=>item.fieldKey==='memory')).toMatchObject({minimum:'2',maximum:'2',numericChoices:['2']});configuration.customerFieldSelections=body.customerFieldSelections;configuration.version++;}await r.fulfill({json:{code:0,data:{configuration}}});});
  const workspace=new CustomerOptionsPage(adminPage);await workspace.goto();await workspace.openProductDetail('product-manual-service');await workspace.openOptions();
  await expect(workspace.optionsForm).toContainText('内存（GiB）');await adminPage.getByTestId('catalog-option-tab-memory').click();
  await adminPage.getByTestId('option_1_numbers').fill('2048');await workspace.confirmOptions();
  await expect(adminPage.getByTestId('catalog-customer-options-validation')).toContainText('1—1024');expect(writes).toHaveLength(0);await expect(adminPage.getByTestId('option_1_numbers')).toHaveValue('2048');await workspace.screenshot('customer-options-invalid-choice');
  await adminPage.getByTestId('option_1_numbers').fill('2');await workspace.confirmOptions();await expect(adminPage.getByText('客户可选配置已保存',{exact:true})).toBeVisible();await expect(workspace.optionsForm).toBeVisible();expect(writes).toHaveLength(1);
  await workspace.reopenOptions();await adminPage.getByTestId('catalog-option-tab-memory').click();await expect(adminPage.getByTestId('option_1_numbers')).toHaveValue('2|2|2');await workspace.screenshot('customer-options-discrete-saved');
 });

 test('TC-44c: 核心项保留，本地自定义分组子项保存回填并可移除',async({adminPage})=>{
  test.setTimeout(45000);
  await installCatalogManagementMock(adminPage);
  const core={key:'os',type:'single_select',required:true,label:{zhCN:'操作系统',enUS:'Operating system'},staticCandidates:[{value:'linux',label:{zhCN:'Linux',enUS:'Linux'}}]};
  const requiredCPU={key:'required_cpu',type:'integer',required:true,label:{zhCN:'核心处理器',enUS:'Required CPU'},minimum:'1',maximum:'16'};
  const configuration:any={version:1,merchantValues:[],customerConfigurableFields:[core,requiredCPU],customerFieldSelections:[],customerDeclaration:{version:'v1',fields:[core,requiredCPU]},productResolvedCandidates:{},customerResolvedCandidates:{}};
  const writes:any[]=[];
  await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:{product:{productNo:'product-manual-service',nameZhCn:'云服务器',nameEnUs:'Server',status:'draft'},version:{versionNo:'version-manual-service-1',status:'draft',source:'manual',versionNumber:1},billingRules:[],prices:[],taxPolicies:[],inventory:{trackingEnabled:false},fulfillmentBinding:{evidenceVerified:true,interfaceNewBusinessState:'accepting',interfaceInstanceNo:'interface-test',interfaceDisplayName:'魔方云'},fulfillmentPolicy:null}}}));
  await adminPage.route('**/admin/catalog/product-versions/*/configuration',async r=>{if(r.request().method()==='PUT'){const body=r.request().postDataJSON();writes.push(body);configuration.customerFieldSelections=body.customerFieldSelections;configuration.version++;}await r.fulfill({json:{code:0,data:{configuration}}});});
  const workspace=new CustomerOptionsPage(adminPage);await workspace.goto();await workspace.openProductDetail('product-manual-service');await workspace.openOptions();
  await expect(adminPage.getByTestId('option_0_enabled')).toBeDisabled();await adminPage.getByTestId('catalog-local-option-type').click();
  for(const label of ['下拉／单选','多选','是／否开关','数量／规格／滑块','小数输入','单行文本','多行文本','日期','日期时间','层级联动／树选择','地区联动选择'])await expect(adminPage.locator('.ant-select-dropdown:visible').getByText(label,{exact:true})).toBeVisible();
  await adminPage.locator('.ant-select-dropdown:visible').getByText('下拉／单选',{exact:true}).click();await adminPage.getByTestId('catalog-local-option-add').click();
  await adminPage.getByTestId('option_2_name_zhCN').fill('套餐等级');await adminPage.getByTestId('option_2_name_enUS').fill('Plan');
  await workspace.confirmOptions();expect(writes).toHaveLength(0);
  await adminPage.getByTestId('option_2_choices').fill('basic|基础版|Basic\npro|专业版|Pro');await workspace.optionsForm.getByLabel('分组名称（中文）',{exact:true}).fill('服务设置');await workspace.optionsForm.getByLabel('分组名称（英文）',{exact:true}).fill('Service settings');
  await workspace.screenshot('customer-custom-option-editor');await workspace.confirmOptions();await expect(adminPage.getByText('客户可选配置已保存',{exact:true})).toBeVisible();await expect(workspace.optionsForm).toBeVisible();
  expect(writes[0].customerFieldSelections.find((item:any)=>item.fieldKey==='required_cpu')).not.toHaveProperty('minimum');
  const local=writes[0].customerFieldSelections.find((item:any)=>item.localField);expect(local).toMatchObject({groupZhCN:'服务设置',groupEnUS:'Service settings',localField:{type:'single_select',label:{zhCN:'套餐等级',enUS:'Plan'},staticCandidates:[{value:'basic',label:{zhCN:'基础版',enUS:'Basic'}},{value:'pro',label:{zhCN:'专业版',enUS:'Pro'}}]}});
  await workspace.reopenOptions();await workspace.optionsForm.getByRole('button',{name:'套餐等级',exact:true}).click();await expect(adminPage.getByTestId('option_2_choices')).toHaveValue('basic|基础版|Basic\npro|专业版|Pro');await expect(workspace.optionsForm.getByLabel('分组名称（中文）',{exact:true})).toHaveValue('服务设置');
  await adminPage.getByTestId('catalog-option-remove').click();await workspace.confirmOptions();await expect(adminPage.getByText('客户可选配置已保存',{exact:true})).toBeVisible();await expect(workspace.optionsForm).toBeVisible();expect(writes[1].customerFieldSelections.every((item:any)=>!item.localField)).toBe(true);expect(writes[1].customerFieldSelections).toContainEqual(expect.objectContaining({fieldKey:'os'}));
 });

 for(const changed of [false,true]) test(`TC-44${changed?'e':'d'}: ${changed?'后台配置真正变化时保留输入并阻止覆盖':'其他页签保存相同配置快照时延续未保存输入'}`,async({adminPage})=>{
  const {writes}=await inlineOptionsFixture(adminPage,changed);
  const view=new CustomerOptionsPage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');await view.openOptions();
  await expect(adminPage.getByTestId('catalog-customer-options-edit')).toHaveCount(0);
  await adminPage.getByTestId('option_0_name_zhCN').fill('尚未保存的规格名称');
  await view.selectEditSection('基本资料');await adminPage.getByTestId('catalog-description-name-zh').fill('更新后的商品资料');await adminPage.getByTestId('catalog-description-save').click();
  await expect(view.detailDrawer.getByRole('heading',{name:'更新后的商品资料',exact:true})).toBeVisible();await view.openOptions();
  await expect(adminPage.getByTestId('option_0_name_zhCN')).toHaveValue('尚未保存的规格名称');
  const save=adminPage.getByTestId('catalog-customer-options-save');
  if(changed) {
   await expect(view.optionsForm).toContainText('商品配置已被其他操作修改。已保留当前输入，请重新加载并核对后保存。');await expect(save).toBeDisabled();expect(writes).toHaveLength(0);
   await view.screenshot('inline-options-concurrent-change-retains-input');
   await adminPage.getByTestId('catalog-customer-options-reload').click();await adminPage.locator('.ant-popover:visible').getByRole('button',{name:/确\s*定/}).click();
   await expect(adminPage.getByTestId('option_0_name_zhCN')).toHaveValue('已保存的规格名');await expect(adminPage.getByTestId('option_0_maximum')).toHaveValue('32');await expect(save).toBeEnabled();
  } else {
   await expect(save).toBeEnabled();await view.confirmOptions();await expect.poll(()=>writes.length).toBe(1);
   expect(writes[0]!.path).toContain('/version-after-basic-save/configuration');expect(writes[0]!.body.expectedVersion).toBe(2);expect(writes[0]!.body.customerFieldSelections[0].displayNameZhCN).toBe('尚未保存的规格名称');
   await expect(adminPage.getByText('客户可选配置已保存',{exact:true})).toBeVisible();await view.reopenOptions();await expect(adminPage.getByTestId('option_0_name_zhCN')).toHaveValue('尚未保存的规格名称');
   await view.screenshot('inline-options-saved-without-nested-modal');
  }
 });
 test('TC-44f: 英文客户配置直接表单及保存按钮正确翻译',async({adminPage,mainLayout})=>{
  await inlineOptionsFixture(adminPage);await mainLayout.switchLanguage('English');const view=new CustomerOptionsPage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');await adminPage.getByTestId('catalog-editor-tab-configuration').click();
  await expect(adminPage.getByTestId('catalog-customer-options-save')).toHaveText('Save purchase options');await expect(view.optionsForm).not.toContainText('plugin.idc-core.');await expect(adminPage.getByRole('dialog')).toHaveCount(1);
  await view.screenshot('inline-options-english');
 });

});
