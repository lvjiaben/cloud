import { test, expect } from '../support/catalog-auth';
import { installCatalogManagementMock } from '../support/catalog-mock';
import { CustomerOptionsPage } from '../pages/CustomerOptionsPage';

test.describe('TC-44 商品客户可选配置', () => {
  test('TC-44a: 默认关闭、明确范围、保存失败保留与重试', async ({adminPage}) => {
    test.setTimeout(45000);
    await installCatalogManagementMock(adminPage);
    const field = {key:'cpu',type:'integer',label:{enUS:'CPU cores',zhCN:'处理器核心数'},minimum:'1',maximum:'1024'};
    const configuration = {version:1,merchantValues:[],customerConfigurableFields:[field],customerFieldSelections:[],productDeclaration:{version:'v1',fields:[]},customerDeclaration:{version:'v1',fields:[]},productResolvedCandidates:{},customerResolvedCandidates:{}};
    await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({json:{code:0,message:'success',data:{product:{productNo:'product-manual-service',nameZhCn:'云服务器',nameEnUs:'Server',status:'draft'},version:{versionNo:'version-manual-service-1',status:'draft',source:'manual',versionNumber:1},billingRules:[],prices:[],taxPolicies:[],inventory:{trackingEnabled:false},fulfillmentBinding:{interfaceDisplayName:'魔方云',interfaceInstanceNo:'interface-test',pluginId:'idc-zjmfcloud',pluginReleaseVersion:'v0.2.7'},fulfillmentPolicy:null}}}));
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
    await expect(adminPage.getByTestId('option_0_enabled')).toHaveAttribute('aria-checked','false');
    await expect(adminPage.getByTestId('option_0_minimum')).toBeHidden();
    await expect(workspace.optionsModal).toContainText('免费也要填0');
    await workspace.enableCPU(); await workspace.confirmOptions();
    expect(writes).toHaveLength(0);
    await workspace.setCPUBounds('2','16');
    await workspace.screenshot('customer-options-ranges');await workspace.confirmOptions();
    await expect(workspace.optionsModal).toBeVisible();
    await expect(adminPage.getByTestId('option_0_maximum')).toHaveValue('16');
    await workspace.confirmOptions();await expect(workspace.optionsModal).toBeHidden();
    expect(writes).toHaveLength(2);
    expect(writes.at(-1)).toMatchObject({expectedVersion:1,merchantValues:[],customerFieldSelections:[{fieldKey:'cpu',minimum:'2',maximum:'16'}]});
    await workspace.openOptions();
    await expect(adminPage.getByTestId('option_0_enabled')).toHaveAttribute('aria-checked','true');
    await expect(adminPage.getByTestId('option_0_maximum')).toHaveValue('16');
    await workspace.screenshot('customer-options-saved');
  });
});
