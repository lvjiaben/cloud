import { test, expect } from '@host-tests/fixtures/auth';
import { installCatalogManagementMock, mockProductDetail } from '../support/catalog-mock';
import { installCatalogAutomationMock } from '../support/catalog-automation-mock';
import { CatalogWorkspacePage } from '../pages/CatalogWorkspacePage';

test.describe('TC-54 商品抽屉页签表单', () => {
  test('TC-54c: 其他页签刷新资料时，未保存表单仍按原版本检查冲突', async ({adminPage}) => {
    await installCatalogManagementMock(adminPage);
    const original=mockProductDetail('product-manual-service');
    const detail={...original,product:{...original.product,descriptionVersion:1}};
    const writes: unknown[]=[];
    await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
    await adminPage.route('**/admin/catalog/product-versions/*/fulfillment-policy',async r=>{
      // Simulate another editor changing the product before the parent refresh.
      detail.product.descriptionVersion=2;
      detail.product.nameZhCn='其他员工已修改的商品';
      await r.fulfill({json:{code:0,data:{policy:{strategyCode:'manual'}}}});
    });
    await adminPage.route('**/admin/catalog/products/product-manual-service/description',async r=>{
      writes.push(r.request().postDataJSON());
      await r.fulfill({status:409,json:{code:50,message:'商品已被其他人修改，请重新打开核对'}});
    });
    const view=new CatalogWorkspacePage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');
    await adminPage.getByTestId('catalog-description-name-zh').fill('尚未保存的商品名');
    await adminPage.getByTestId('catalog-description-summary-zh').fill('尚未保存的说明');
    await adminPage.getByTestId('catalog-description-summary-en').fill('Unsaved description');
    await view.openFulfillmentPolicy();await view.fulfillmentPolicyConfirm().click();
    await expect(adminPage.getByText('履约方式已保存',{exact:true})).toBeVisible();
    await expect(view.detailDrawer.getByRole('heading',{name:'其他员工已修改的商品',exact:true})).toBeVisible();
    await view.selectEditSection('基本资料');
    await expect(adminPage.getByTestId('catalog-description-name-zh')).toHaveValue('尚未保存的商品名');
    await adminPage.getByTestId('catalog-description-save').click();
    await expect.poll(()=>writes.length).toBe(1);
    expect(writes[0]).toMatchObject({version:1,nameZhCn:'尚未保存的商品名'});
    await expect(adminPage.getByText('商品已被其他人修改，请重新打开核对',{exact:true})).toBeVisible();
    await expect(adminPage.getByTestId('catalog-description-name-zh')).toHaveValue('尚未保存的商品名');
    await view.screenshot('product-drawer-concurrent-update');
  });
  test('TC-54b: 页内修改履约方式，失败保留选择，成功仍在当前页签', async ({adminPage}) => {
    await installCatalogManagementMock(adminPage);
    const detail = {...mockProductDetail('product-manual-service'), fulfillmentBinding: {interfaceInstanceNo:'fixture-interface',interfaceDisplayName:'测试上游',evidenceVerified:false,interfaceNewBusinessState:'accepting'}, fulfillmentPolicy: {strategyCode:'manual',quantityHandling:'together',capabilityZhCn:'',capabilityEnUs:''}};
    const writes: unknown[] = [];
    await adminPage.route('**/admin/catalog/products/product-manual-service', r => r.fulfill({json:{code:0,data:detail}}));
    await adminPage.route('**/admin/catalog/product-versions/*/fulfillment-policy', async r => {
      const body=r.request().postDataJSON();writes.push(body);
      if(writes.length===1){await r.fulfill({status:503,json:{code:50,message:'开通方式暂时保存失败'}});return;}
      Object.assign(detail.fulfillmentPolicy,body);
      await r.fulfill({json:{code:0,data:{policy:detail.fulfillmentPolicy}}});
    });
    const view=new CatalogWorkspacePage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');await view.openFulfillmentPolicy();
    await expect(adminPage.getByTestId('catalog-fulfillment-policy-open')).toHaveCount(0);
    await view.selectFulfillmentPolicyStrategy('付款后自动开通');
    await view.fulfillmentPolicyConfirm().click();
    await expect(adminPage.getByText('开通方式暂时保存失败',{exact:true})).toBeVisible();
    await expect(view.fulfillmentPolicyStrategy).toContainText('付款后自动开通');
    await view.screenshot('product-drawer-policy-failed');
    await view.fulfillmentPolicyConfirm().click();
    await expect.poll(()=>writes.length).toBe(2);
    expect(writes).toEqual([{strategyCode:'payment_confirmed_plugin'},{strategyCode:'payment_confirmed_plugin'}]);
    await expect(view.detailDrawer.getByRole('tab',{name:'自动开通',exact:true})).toHaveAttribute('aria-selected','true');
    await expect(view.fulfillmentPolicyStrategy).toContainText('付款后自动开通');
    await view.screenshot('product-drawer-policy-saved');
  });
  test('TC-54a: 直接填写、切换保留、失败重试、刷新恢复与关闭回到原筛选', async ({adminPage}) => {
    const state = await installCatalogManagementMock(adminPage);
    const original = mockProductDetail('product-manual-service');
    const detail = {...original, product: {...original.product, descriptionVersion: 1}};
    const writes: Record<string, unknown>[] = [];
    await adminPage.route('**/admin/catalog/products/product-manual-service', route => route.fulfill({json: {code: 0, data: detail}}));
    await adminPage.route('**/admin/catalog/products/product-manual-service/description', async route => {
      const body = route.request().postDataJSON(); writes.push(body);
      if (writes.length === 1) {await route.fulfill({status: 503, json: {code: 50, message: '商品保存暂时失败'}}); return;}
      Object.assign(detail.product, body, {descriptionVersion: 2});
      await route.fulfill({json: {code: 0, data: {version: 2}}});
    });
    const view = new CatalogWorkspacePage(adminPage);
    await view.goto();
    await view.filterStatus('已下架');
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get('status')).toBe('suspended');
    await view.openProductDetail('product-manual-service');
    await expect(adminPage.getByRole('dialog').filter({has: view.detailDrawer})).toBeVisible();
    await expect(adminPage.getByRole('dialog').filter({has: view.detailDrawer}).getByRole('button',{name:/^确\s*认$/u})).toHaveCount(0);
    const tabs = view.detailDrawer.getByRole('tablist');
    for (const name of ['基本资料','自动开通','产品配置','计费与售价','附加商品']) await expect(tabs.getByRole('tab', {name, exact: true})).toBeVisible();
    await expect(adminPage.getByTestId('catalog-editor-next')).toHaveCount(0);
    await expect(adminPage.getByTestId('catalog-editor-previous')).toHaveCount(0);
    await expect(adminPage.getByTestId('catalog-description-edit')).toHaveCount(0);
    for (const [id,value] of [['name-zh','抽屉测试商品'],['name-en','Drawer product'],['summary-zh','保留未提交的说明'],['summary-en','Preserve the draft description']]) await adminPage.getByTestId(`catalog-description-${id}`).fill(value);
    await view.selectEditSection('购买配置');
    await expect(view.detailDrawer.getByText('暂无购买配置。人工交付商品可直接设置价格；自动交付商品先在交付设置中选择插件。',{exact:true})).toBeVisible();
    await view.selectEditSection('基本资料');
    await expect(adminPage.getByTestId('catalog-description-name-zh')).toHaveValue('抽屉测试商品');
    await expect(adminPage.getByTestId('catalog-description-summary-zh')).toHaveValue('保留未提交的说明');
    await view.screenshot('product-drawer-basic-form');
    await adminPage.getByTestId('catalog-description-save').click();
    await expect(adminPage.getByText('商品保存暂时失败',{exact:true})).toBeVisible();
    await expect(adminPage.getByTestId('catalog-description-name-zh')).toHaveValue('抽屉测试商品');
    await view.screenshot('product-drawer-save-failed');
    await adminPage.getByTestId('catalog-description-save').click();
    await expect.poll(() => writes.length).toBe(2);
    expect(writes[1]).toEqual(writes[0]);
    expect(writes[1]).toMatchObject({version:1,nameZhCn:'抽屉测试商品',summaryZhCn:'保留未提交的说明'});
    await expect(view.detailDrawer.getByRole('heading',{name:'抽屉测试商品',exact:true})).toBeVisible();
    await expect(view.detailDrawer.getByRole('tab',{name:'基本资料',exact:true})).toHaveAttribute('aria-selected','true');
    await view.screenshot('product-drawer-saved');
    await adminPage.setViewportSize({width:414,height:896});
    await expect.poll(() => view.detailDrawer.evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
    await view.screenshot('product-drawer-mobile');
    await adminPage.setViewportSize({width:1280,height:900});
    await view.closeProductDetail();
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get('status')).toBe('suspended');
    await expect(adminPage.getByTestId('catalog-product-create-open')).toBeVisible();
    await view.openProductDetail('product-manual-service');
    await adminPage.reload();
    await expect(view.detailDrawer.getByRole('heading',{name:'抽屉测试商品',exact:true})).toBeVisible();
    await view.closeProductDetail();
    expect(state.previewRequests).toHaveLength(0); expect(state.publishRequests).toHaveLength(0);
  });
  test('TC-54d: 直接回填接口、搜索翻页，一次保存失败保留输入且不调用旧绑定接口',async({adminPage})=>{
    await installCatalogManagementMock(adminPage);
    const current={interfaceInstanceNo:'pfi-current',interfaceDisplayName:'原服务器上游',evidenceVerified:false,interfaceNewBusinessState:'accepting',boundAt:1};
    const detail={...mockProductDetail('product-manual-service'),fulfillmentBinding:current,fulfillmentPolicy:{strategyCode:'manual',quantityHandling:'together',capabilityZhCn:'',capabilityEnUs:''}};
    await adminPage.route('**/admin/catalog/products/product-manual-service',r=>r.fulfill({json:{code:0,data:detail}}));
    const searches:URL[]=[];const writes:Record<string,unknown>[]=[];let legacyWrites=0;
    await adminPage.route('**/admin/catalog/automation/interfaces?*',r=>{
      const url=new URL(r.request().url());searches.push(url);
      const target=Number(url.searchParams.get('page')||1);
      return r.fulfill({json:{code:0,data:{items:[{interfaceInstanceNo:target===2?'pfi-new':'pfi-first',displayName:target===2?'南京魔方云':'上海魔方云',newBusinessState:'accepting',version:1}],total:21}}});
    });
    await adminPage.route('**/admin/catalog/product-versions/*/fulfillment-binding',r=>{legacyWrites++;return r.fulfill({status:500,json:{code:50,message:'禁止拆分保存'}});});
    await adminPage.route('**/admin/catalog/product-versions/*/fulfillment-policy',async r=>{
      const body=r.request().postDataJSON();writes.push(body);
      if(writes.length===1) return r.fulfill({status:503,json:{code:50,message:'接口与开通方式一起保存失败'}});
      Object.assign(detail.fulfillmentBinding,{interfaceInstanceNo:body.interfaceInstanceNo,interfaceDisplayName:'南京魔方云'});Object.assign(detail.fulfillmentPolicy,{strategyCode:body.strategyCode});
      return r.fulfill({json:{code:0,data:{policy:detail.fulfillmentPolicy}}});
    });
    const view=new CatalogWorkspacePage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');await view.openFulfillmentPolicy();
    const select=adminPage.getByTestId('catalog-fulfillment-interface');
    await expect(select).toContainText('原服务器上游');
    await expect(adminPage.getByTestId('catalog-fulfillment-provider')).toHaveCount(0);
    await expect.poll(()=>searches.length).toBe(1);expect(searches[0]!.searchParams.get('forProductBinding')).toBe('true');expect(searches[0]!.searchParams.has('providerSelectionToken')).toBe(false);
    await adminPage.getByTestId('catalog-interface-pagination').getByTitle('2',{exact:true}).click();
    await expect.poll(()=>searches.at(-1)!.searchParams.get('page')).toBe('2');
    await select.click();await adminPage.locator('.ant-select-dropdown:visible').getByText('南京魔方云',{exact:true}).click();
    await view.selectFulfillmentPolicyStrategy('付款后自动开通');await view.fulfillmentPolicyConfirm().click();
    await expect(adminPage.getByText('接口与开通方式一起保存失败',{exact:true})).toBeVisible();
    await expect(select).toContainText('南京魔方云');await expect(view.fulfillmentPolicyStrategy).toContainText('付款后自动开通');
    await view.screenshot('unified-provisioning-failure-retains-form');
    await view.selectEditSection('基本资料');await view.openFulfillmentPolicy();await expect(select).toContainText('南京魔方云');
    await select.click();await select.getByRole('combobox').fill('南京');
    await expect.poll(()=>searches.at(-1)!.searchParams.get('namePrefix')).toBe('南京');
    await adminPage.keyboard.press('Escape');await expect(select).toContainText('南京魔方云');await expect(view.detailDrawer).toBeVisible();
    await view.fulfillmentPolicyConfirm().click();await expect.poll(()=>writes.length).toBe(2);
    expect(writes).toEqual([{strategyCode:'payment_confirmed_plugin',interfaceInstanceNo:'pfi-new'},{strategyCode:'payment_confirmed_plugin',interfaceInstanceNo:'pfi-new'}]);expect(legacyWrites).toBe(0);
    await expect(view.detailDrawer.getByRole('tab',{name:'自动开通',exact:true})).toHaveAttribute('aria-selected','true');
    await expect(view.fulfillmentPolicyModal).not.toContainText('更换接口后');
    await expect(adminPage.getByText('接口与开通方式一起保存失败',{exact:true})).toBeHidden();
    await view.screenshot('unified-provisioning-saved');
  });
  test('TC-54e: 未选接口禁止自动开通提交，列表失败可重试，停用候选不可新选',async({adminPage})=>{
    await installCatalogManagementMock(adminPage);let attempts=0;let writes=0;
    await adminPage.route('**/admin/catalog/automation/interfaces?*',r=>{attempts++;return attempts===1?r.fulfill({status:503,json:{code:50,message:'接口列表暂不可用'}}):r.fulfill({json:{code:0,data:{items:[{interfaceInstanceNo:'pfi-stopped',displayName:'停用的上游',newBusinessState:'stopped',version:1}],total:1}}});});
    await adminPage.route('**/admin/catalog/product-versions/*/fulfillment-policy',r=>{writes++;return r.fulfill({json:{code:0,data:{policy:{strategyCode:'manual'}}}});});
    const view=new CatalogWorkspacePage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');await view.openFulfillmentPolicy();
    await expect(view.fulfillmentPolicyModal).toContainText('接口列表加载失败，请重试；已填写的内容仍保留。');
    await view.selectFulfillmentPolicyStrategy('付款后自动开通');await view.fulfillmentPolicyConfirm().click();
    await expect(view.fulfillmentPolicyModal).toContainText('请先选择用于自动开通的接口');expect(writes).toBe(0);
    await adminPage.getByTestId('catalog-fulfillment-interface-retry').click();
    await expect.poll(()=>attempts).toBe(2);await adminPage.getByTestId('catalog-fulfillment-interface').click();
    await expect(adminPage.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:'停用的上游'})).toHaveClass(/ant-select-item-option-disabled/);
    await adminPage.keyboard.press('Escape');await view.screenshot('unified-provisioning-validation-and-stopped-interface');
  });
  test('TC-54f: 只读商品不加载接口候选且不能保存自动开通',async({adminPage})=>{
    await installCatalogManagementMock(adminPage);await installCatalogAutomationMock(adminPage,{permissions:['idc:admin:catalog:product:list','idc:admin:catalog:product:query']});
    let reads=0;await adminPage.route('**/admin/catalog/automation/interfaces?*',r=>{reads++;return r.fulfill({status:403,json:{code:403,message:'无权读取接口'}});});
    // Hash navigation alone keeps the administrator's in-memory permission store.
    await adminPage.reload();
    const view=new CatalogWorkspacePage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');await view.openFulfillmentPolicy();
    await expect(adminPage.getByTestId('catalog-fulfillment-interface').getByRole('combobox')).toBeDisabled();await expect(view.fulfillmentPolicyConfirm()).toHaveCount(0);expect(reads).toBe(0);
    await view.screenshot('unified-provisioning-read-only');
  });
  test('TC-54g: 英文接口和开通方式显示真实译文',async({adminPage,mainLayout})=>{
    await installCatalogManagementMock(adminPage);await adminPage.route('**/admin/catalog/automation/interfaces?*',r=>r.fulfill({json:{code:0,data:{items:[{interfaceInstanceNo:'pfi-english',displayName:'Cloud upstream',newBusinessState:'accepting',version:1}],total:1}}}));
    await mainLayout.switchLanguage('English');const view=new CatalogWorkspacePage(adminPage);await view.goto();await view.openProductDetail('product-manual-service');
    await adminPage.getByTestId('catalog-editor-tab-fulfillment').click();
    await expect(view.fulfillmentPolicyModal).toContainText('Upstream / interface');await expect(view.fulfillmentPolicyModal).toContainText('No automation interface');
    await expect(view.fulfillmentPolicyConfirm()).toHaveText('Save provisioning settings');
    await adminPage.getByTestId('catalog-fulfillment-interface').click();await adminPage.locator('.ant-select-dropdown:visible').getByText('Cloud upstream',{exact:true}).click();
    await expect(adminPage.getByTestId('catalog-fulfillment-interface-state')).toContainText('Accepting new sales');
    await adminPage.getByTestId('catalog-fulfillment-policy-strategy').click();await expect(adminPage.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^Provision automatically after payment$/})).toBeVisible();
    await adminPage.keyboard.press('Escape');await view.screenshot('unified-provisioning-english');
  });

});
