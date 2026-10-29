import type { Page } from '@host-tests/support/playwright';
import { test,expect } from '../support/merchant-auth';
import { installCatalogManagementMock,mockProductDetail } from '../support/catalog-mock';
import { CatalogBulkSafetyPage } from '../pages/CatalogBulkSafetyPage';

test.describe('TC-84 商品子项批量维护及未保存保护',()=>{
  async function setup(page:Page) {
    await installCatalogManagementMock(page);
    const detail:any=mockProductDetail('product-manual-service');
    detail.version.status='draft';detail.product.directoryNo='pdr_test';detail.product.descriptionVersion=1;
    detail.fulfillmentBinding={interfaceInstanceNo:'cloud',interfaceDisplayName:'魔方云',interfaceNewBusinessState:'accepting'};
    const priced:any=mockProductDetail('product-ssl');detail.billingRules=priced.billingRules;detail.prices=priced.prices;detail.taxPolicies=priced.taxPolicies;
    const configuration:any={version:1,merchantValues:[],customerConfigurableFields:[],customerFieldSelections:[],productDeclaration:{version:'v1',fields:[]},customerDeclaration:{version:'v1',fields:[]},productResolvedCandidates:{},customerResolvedCandidates:{}};
    const writes:any[]=[];
    await page.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
    await page.route('**/admin/product-directories?*',r=>r.fulfill({json:{code:0,data:{items:[{directoryNo:'pdr_test',navigationType:'product',nameZhCn:'云产品',nameEnUs:'Cloud products',statusCode:'enabled'}],total:1}}}));
    await page.route('**/admin/catalog/products/*/addons',r=>r.fulfill({json:{code:0,data:{addons:[]}}}));
    await page.route('**/public/customer-center/configuration',r=>r.fulfill({json:{code:0,data:{selectedPlugin:'customer-tdesign',config:{websiteurl:'https://shop.example',websiteconfigurepath:'/configure.html?product={product}',websitecatalogpath:'/products.html'}}}}));
    await page.route('**/admin/catalog/product-versions/*/configuration',r=>{
      if(r.request().method()==='PUT'){
        const input=r.request().postDataJSON();writes.push(input);configuration.version++;
        configuration.customerFieldSelections=input.customerFieldSelections;
        configuration.customerDeclaration.fields=input.customerFieldSelections.map((row:any)=>({...row.localField,key:row.fieldKey}));
      }
      return r.fulfill({json:{code:0,data:{configuration}}});
    });
    const ui=new CatalogBulkSafetyPage(page);await ui.goto();await ui.openProductDetail('product-manual-service');
    return {ui,detail,configuration,writes};
  }
  test('TC-84a: 批量逐行校验，保留逗号内容，复制不覆盖旧子项或费用',async({adminPage})=>{
    const {ui,writes}=await setup(adminPage);
    await ui.tab('configuration');await ui.customParameter();
    await ui.parameterInput('product_field').fill('vendor_size');await ui.parameterInput('name_zhCN').fill('性能档位');
    await ui.openBatch();await expect(ui.batchDialog).toContainText('每行一个子项');
    await ui.pasteItems('fast\t高性能\t500,500,2000,2000\nfast\t重复档位\t1\n\t缺失值\t2');
    await expect(ui.batchDialog).toContainText('子项值重复，请使用不同的值');await expect(ui.batchDialog).toContainText('请填写子项值和显示名称');
    await expect(ui.batchDialog.getByRole('button',{name:'添加到子项',exact:true})).toBeDisabled();expect(writes).toHaveLength(0);
    await ui.screenshot('editor-batch-row-errors');
    await ui.pasteItems('fast\t高性能\t500,500,2000,2000\nbasic\t基础版\t200,200,500,500');await ui.applyBatch();
    await expect(ui.parameterModal.getByTestId('catalog-option-child-upstream')).toHaveValue('500,500,2000,2000');
    await ui.priceFirstItem();await ui.copyItem();await expect(ui.parameterModal.getByTestId('catalog-option-child-value')).toHaveValue('');
    await expect(ui.parameterModal.getByTestId('catalog-option-child-zhCN')).toHaveValue('高性能');
    await expect(ui.parameterModal.getByTestId('catalog-option-child-upstream')).toHaveValue('500,500,2000,2000');
    await ui.parameterModal.getByTestId('catalog-option-child-value').fill('fast-copy');
    await ui.openBatch();await ui.pasteItems('discard\t取消添加\t1');await ui.cancelBatch();
    await expect(ui.parameterModal.locator('[data-testid$="_choices"]').getByRole('list').getByRole('button')).toHaveCount(3);
    await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();expect(writes).toHaveLength(1);
    const selection=writes[0].customerFieldSelections[0];
    expect(selection.localField.staticCandidates.map((row:any)=>row.value)).toEqual(['fast','fast-copy','basic']);
    expect(selection.productChoiceBindings).toEqual({fast:'500,500,2000,2000','fast-copy':'500,500,2000,2000',basic:'200,200,500,500'});
    expect(writes[0].priceRows).toHaveLength(1);expect(writes[0].priceRows[0]).toMatchObject({candidateValue:'fast',tiers:[{amountMinor:'500',renewalAmountMinor:'600',setupFeeMinor:'0'}]});
    expect(writes[0].priceRows.some((row:any)=>row.candidateValue==='fast-copy')).toBe(false);
    await ui.screenshot('editor-batch-saved');
  });
  test('TC-84b: 无修改直接关闭，失败保存后跨页签保留，取消丢弃后继续保存',async({adminPage})=>{
    const {ui,detail}=await setup(adminPage);let attempts=0;
    await ui.closeEditor();await expect(ui.detailDrawer).toBeHidden();await expect(ui.leaveDialog).toHaveCount(0);
    await ui.openProductDetail('product-manual-service');
    await adminPage.route('**/admin/catalog/products/*/description',r=>{
      const input=r.request().postDataJSON();attempts++;
      if(attempts===1)return r.fulfill({json:{code:409,data:{},message:'保存失败，请重试'}});
      Object.assign(detail.product,input,{descriptionVersion:2});return r.fulfill({json:{code:0,data:{version:2}}});
    });
    await expect(ui.saveReminder).toBeHidden();
    await ui.changeName('未保存的商品');await expect(ui.saveReminder).toHaveText('还有内容未保存');await ui.tab('pricing');await expect(ui.saveReminder).toHaveText('还有内容未保存');await ui.tab('basic');
    await expect(ui.descriptionName).toHaveValue('未保存的商品');await expect(adminPage.getByTestId('catalog-unsaved-changes')).toContainText('基本资料');expect(await ui.beforeUnloadBlocked()).toBe(true);
    await ui.saveBasic();await expect.poll(()=>attempts).toBe(1);await expect(ui.saveReminder).toHaveText('还有内容未保存');await expect(ui.descriptionName).toHaveValue('未保存的商品');
    await ui.closeEditor();await expect(ui.leaveDialog).toContainText('关闭后将丢弃');await ui.screenshot('editor-unsaved-confirm');
    await ui.continueEditing();await expect(ui.descriptionName).toHaveValue('未保存的商品');await ui.saveBasic();await expect.poll(()=>attempts).toBe(2);
    await expect(adminPage.getByTestId('catalog-unsaved-changes')).toBeHidden();await expect(ui.saveReminder).toBeHidden();expect(await ui.beforeUnloadBlocked()).toBe(false);await ui.closeEditor();await expect(ui.detailDrawer).toBeHidden();
    await ui.openProductDetail('product-manual-service');await ui.changeName('主动放弃');await ui.closeEditor();await ui.discard();
    await ui.openProductDetail('product-manual-service');await expect(ui.descriptionName).toHaveValue('未保存的商品');expect(attempts).toBe(2);await ui.screenshot('editor-unsaved-saved');
  });
  test('TC-84c: 写入成功而读取失败时明确已保存，关闭不误报未保存',async({adminPage})=>{
    const {ui,detail}=await setup(adminPage);let failRead=false;let writes=0;
    await adminPage.route('**/admin/catalog/products/product-manual-service',r=>{
      if(failRead){failRead=false;return r.fulfill({json:{code:50,data:{},message:'读取最新商品失败'}});}
      return r.fulfill({json:{code:0,data:detail}});
    });
    await adminPage.route('**/admin/catalog/products/*/description',r=>{
      writes++;Object.assign(detail.product,r.request().postDataJSON(),{descriptionVersion:2});failRead=true;
      return r.fulfill({json:{code:0,data:{version:2}}});
    });
    await ui.changeName('已保存但读取失败');await ui.saveBasic();
    await expect(adminPage.getByTestId('catalog-detail-refresh-failed')).toBeVisible();
    await expect(adminPage.getByTestId('catalog-unsaved-changes')).toBeHidden();
    await expect(ui.saveReminder).toBeHidden();
    await ui.screenshot('editor-saved-read-recovery');
    await ui.closeEditor();await expect(ui.detailDrawer).toBeHidden();await expect(ui.leaveDialog).toHaveCount(0);
    await ui.openProductDetail('product-manual-service');await expect(ui.descriptionName).toHaveValue('已保存但读取失败');expect(writes).toBe(1);
  });

  test('TC-84d: 持续浮层提醒，取消刷新、返回及按Esc关闭均保留原输入',async({adminPage})=>{
    const {ui}=await setup(adminPage);await ui.prepareBackNavigation();
    await ui.changeName('取消退出保留的草稿');
    await expect(ui.saveReminder).toHaveText('还有内容未保存');
    await ui.screenshot('editor-save-floating-reminder');
    expect(await ui.cancelRefresh()).toBe('beforeunload');
    await expect(ui.descriptionName).toHaveValue('取消退出保留的草稿');
    await expect(ui.saveReminder).toBeVisible();
    expect(await ui.cancelBrowserClose()).toBe('beforeunload');
    await expect(ui.descriptionName).toHaveValue('取消退出保留的草稿');
    await ui.closeByEscape();await expect(ui.leaveDialog).toBeVisible();
    await ui.continueEditing();await expect(ui.descriptionName).toHaveValue('取消退出保留的草稿');
    await ui.leaveByBack();await expect(ui.leaveDialog).toBeVisible();
    await ui.continueEditing();await expect(ui.descriptionName).toHaveValue('取消退出保留的草稿');
    await expect(ui.saveReminder).toHaveText('还有内容未保存');
    await ui.closeEditor();await ui.discard();
    await expect(ui.saveReminder).toBeHidden();expect(await ui.beforeUnloadBlocked()).toBe(false);
    await ui.openProductDetail('product-manual-service');await expect(ui.descriptionName).not.toHaveValue('取消退出保留的草稿');
    await ui.changeName('确认离开丢弃的草稿');await ui.leaveByBack();await expect(ui.leaveDialog).toBeVisible();
    await ui.discard();await expect(adminPage).not.toHaveURL(/#\/idc\/catalog/);await expect(ui.leaveDialog).toHaveCount(0);
    expect(await ui.beforeUnloadBlocked()).toBe(false);
  });

  test('TC-84e: 表格排序失焦后提醒保存，显式保存才更新配置',async({adminPage})=>{
    const {ui,writes}=await setup(adminPage);
    await ui.tab('configuration');await ui.customParameter();
    await ui.parameterInput('product_field').fill('vendor_tier');await ui.parameterInput('name_zhCN').fill('规格档位');
    await ui.addChoice('basic','基础档');await ui.saveParameter();await expect(ui.parameterModal).toBeHidden();
    await expect.poll(()=>writes.length).toBe(1);
    const fieldKey=writes[0].customerFieldSelections[0].fieldKey;
    await expect(ui.saveReminder).toBeHidden();
    await ui.sortParameter(fieldKey,'7');
    await expect(ui.saveReminder).toHaveText('还有内容未保存');expect(writes).toHaveLength(1);
    await ui.screenshot('editor-sort-save-reminder');
    await ui.saveTable();await expect.poll(()=>writes.length).toBe(2);
    expect(writes[1].customerFieldSelections[0].sortOrder).toBe(7);
    await expect(ui.saveReminder).toBeHidden();expect(await ui.beforeUnloadBlocked()).toBe(false);
  });

});
