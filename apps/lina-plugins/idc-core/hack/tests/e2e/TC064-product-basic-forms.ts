import type { Page } from '@host-tests/support/playwright';
import { test, expect } from '@host-tests/fixtures/auth';
import { installCatalogManagementMock, mockProductDetail } from '../support/catalog-mock';
import { installCatalogAutomationMock } from '../support/catalog-automation-mock';
import { CatalogWorkspacePage } from '../pages/CatalogWorkspacePage';

async function fixture(page:Page) {
 const state=await installCatalogManagementMock(page);
 const detail:any=mockProductDetail('product-manual-service');
 detail.product.descriptionVersion=1;
 const descriptions:any[]=[];const stocks:Array<{body:any;url:string}>=[];
 let failDescription=false;let failInventory=false;
 await page.route('**/admin/catalog/products/product-manual-service',route=>route.fulfill({json:{code:0,data:detail}}));
 await page.route('**/admin/catalog/products/product-manual-service/description',route=>{
  const body=route.request().postDataJSON();descriptions.push(body);
  if(failDescription){failDescription=false;return route.fulfill({status:409,json:{code:409,message:'商品资料已变更，请重新检查'}});}
  Object.assign(detail.product,body,{descriptionVersion:body.version+1});
  return route.fulfill({json:{code:0,data:{version:body.version+1}}});
 });
 await page.route('**/admin/catalog/product-versions/*/inventory',route=>{
  const body=route.request().postDataJSON();stocks.push({body,url:route.request().url()});
  if(failInventory){failInventory=false;return route.fulfill({status:503,json:{code:503,message:'库存暂时无法保存'}});}
  detail.inventory={trackingEnabled:body.trackingEnabled,sellableQuantity:body.sellableQuantity,version:body.expectedVersion+1};
  return route.fulfill({json:{code:0,data:{inventory:detail.inventory}}});
 });
 let policyWrites=0;
 await page.route('**/admin/catalog/product-versions/*/fulfillment-policy',route=>{
  policyWrites++;detail.version.versionNo=`working-copy-${policyWrites+1}`;
  detail.inventory={...detail.inventory,version:detail.inventory.version+1};
  return route.fulfill({json:{code:0,data:{policy:{strategyCode:'manual'}}}});
 });
 return {state,detail,descriptions,stocks,view:new CatalogWorkspacePage(page),failDescription(){failDescription=true;},failInventory(){failInventory=true;},policyWrites:()=>policyWrites};
}
async function open(page:Page,view:CatalogWorkspacePage) {
 await view.goto();await view.openProductDetail('product-manual-service');
 await page.getByTestId('catalog-editor-tab-basic').click();
 await expect(page.getByTestId('catalog-description-name-zh')).toBeVisible();
}
async function copy(page:Page) {
 await page.getByTestId('catalog-description-summary-zh').fill('专业迁移服务');
 await page.getByTestId('catalog-description-summary-en').fill('Professional migration service');
}

test.describe('商品基本资料直接表单',()=>{
 test('TC-64a: 分类与资料同次提交，冲突保留输入并可重试',async({adminPage})=>{
  const f=await fixture(adminPage);await open(adminPage,f.view);await copy(adminPage);
  await adminPage.getByTestId('catalog-description-category').click();
  await adminPage.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({hasText:/^实物商品$/}).click();
  f.failDescription();await adminPage.getByTestId('catalog-description-save').click();
  await expect(adminPage.getByText('商品资料已变更，请重新检查',{exact:true})).toBeVisible();
  await expect(adminPage.getByTestId('catalog-description-summary-zh')).toHaveValue('专业迁移服务');
  await expect(adminPage.getByTestId('catalog-description-category')).toContainText('实物商品');
  await f.view.screenshot('product-basic-category-conflict-keeps-input');
  await adminPage.getByTestId('catalog-description-save').click();
  await expect.poll(()=>f.descriptions.length).toBe(2);
  expect(f.descriptions[1]).toEqual(f.descriptions[0]);
  expect(f.descriptions[1]).toMatchObject({categoryNo:'ctg_00000000000000000000000000000003',version:1,summaryEnUs:'Professional migration service'});
  await expect(adminPage.getByTestId('catalog-description-save')).not.toHaveClass(/loading/);
  await adminPage.getByTestId('catalog-description-save').click();
  await expect.poll(()=>f.descriptions.length).toBe(3);expect(f.descriptions[2].version).toBe(2);
  await f.view.screenshot('product-basic-category-direct-saved');
 });
 test('TC-64b: 没有分类查看权限仍可改资料，省略分类和会员导航，不显示库存写入口',async({adminPage})=>{
  const f=await fixture(adminPage);
  await installCatalogAutomationMock(adminPage,{permissions:['idc:admin:catalog:product:list','idc:admin:catalog:product:query','idc:admin:catalog:product:update']});
  await adminPage.reload();await open(adminPage,f.view);await copy(adminPage);
  await expect(adminPage.getByTestId('catalog-description-category')).toHaveCount(0);
  await expect(adminPage.getByTestId('catalog-description-member-menu')).toHaveCount(0);
  await expect(adminPage.getByTestId('catalog-inventory-save')).toHaveCount(0);
  await adminPage.getByTestId('catalog-description-save').click();await expect.poll(()=>f.descriptions.length).toBe(1);
  expect(f.descriptions[0].categoryNo).toBeUndefined();expect(f.descriptions[0].directoryNo).toBeUndefined();
  expect(f.state.categoryRequests).toHaveLength(0);expect(f.stocks).toHaveLength(0);
  await f.view.screenshot('product-basic-permission-preserves-assignments');
 });
 test('TC-64c: 库存限制与零库存直接保存，失败保留，不打开第二个弹窗',async({adminPage})=>{
  const f=await fixture(adminPage);await open(adminPage,f.view);
  await expect(adminPage.getByTestId('catalog-inventory-editor')).toBeVisible();
  await expect.poll(()=>adminPage.getByTestId('catalog-inventory-tracking').evaluate(el=>el.getBoundingClientRect().width)).toBeLessThanOrEqual(64);
  await adminPage.getByTestId('catalog-inventory-quantity').fill('0');f.failInventory();
  await adminPage.getByTestId('catalog-inventory-save').click();
  await expect(adminPage.getByText('库存暂时无法保存',{exact:true})).toBeVisible();
  await expect(adminPage.getByTestId('catalog-inventory-quantity')).toHaveValue('0');
  await adminPage.getByTestId('catalog-inventory-save').click();await expect.poll(()=>f.stocks.length).toBe(2);
  expect(f.stocks[1]!.body).toEqual({trackingEnabled:true,sellableQuantity:0,expectedVersion:1});
  await expect(adminPage.getByRole('dialog')).toHaveCount(1);
  await f.view.screenshot('product-inventory-inline-zero-stock');
  await adminPage.getByTestId('catalog-inventory-tracking').click();
  await expect(adminPage.getByTestId('catalog-inventory-quantity')).toBeHidden();
  await adminPage.getByTestId('catalog-inventory-save').click();await expect.poll(()=>f.stocks.length).toBe(3);
  expect(f.stocks[2]!.body).toMatchObject({trackingEnabled:false,sellableQuantity:0,expectedVersion:2});
 });
 test('TC-64d: 跨页签等价更新保留库存输入并推进锚点，真实冲突禁止覆盖',async({adminPage})=>{
  const f=await fixture(adminPage);await open(adminPage,f.view);
  await adminPage.getByTestId('catalog-inventory-quantity').fill('7');
  await f.view.openFulfillmentPolicy();await f.view.fulfillmentPolicyConfirm().click();await expect.poll(()=>f.policyWrites()).toBe(1);
  await adminPage.getByTestId('catalog-editor-tab-basic').click();
  await expect(adminPage.getByTestId('catalog-inventory-quantity')).toHaveValue('7');
  await adminPage.getByTestId('catalog-inventory-save').click();await expect.poll(()=>f.stocks.length).toBe(1);
  expect(f.stocks[0]!.url).toContain('/working-copy-2/');expect(f.stocks[0]!.body.expectedVersion).toBe(2);
  await expect.poll(()=>f.detail.inventory.version).toBe(3);
  await adminPage.getByTestId('catalog-inventory-quantity').fill('9');
  f.detail.inventory.sellableQuantity=12;
  await f.view.openFulfillmentPolicy();await f.view.fulfillmentPolicyConfirm().click();await expect.poll(()=>f.policyWrites()).toBe(2);
  await adminPage.getByTestId('catalog-editor-tab-basic').click();
  await expect(adminPage.getByTestId('catalog-inventory-conflict')).toBeVisible();
  await expect(adminPage.getByTestId('catalog-inventory-quantity')).toHaveValue('9');
  await expect(adminPage.getByTestId('catalog-inventory-save')).toBeDisabled();expect(f.stocks).toHaveLength(1);
  await f.view.screenshot('product-inventory-real-conflict-preserves-input');
  await adminPage.getByTestId('catalog-inventory-reload').click();
  await expect(adminPage.getByTestId('catalog-inventory-quantity')).toHaveValue('12');
  await expect(adminPage.getByTestId('catalog-inventory-conflict')).toHaveCount(0);
 });
 test('TC-64e: 英文和窄屏仍是同页表单，超安全整数库存不允许误写',async({adminPage,mainLayout})=>{
  const f=await fixture(adminPage);f.detail.inventory.sellableQuantity=9_007_199_254_740_992;
  await mainLayout.switchLanguage('English');await open(adminPage,f.view);
  await expect(adminPage.getByTestId('catalog-description-category')).toContainText('Professional services');
  await expect(adminPage.getByTestId('catalog-inventory-editor')).toContainText('too large to edit accurately');
  await expect(adminPage.getByTestId('catalog-inventory-save')).toBeDisabled();
  await adminPage.setViewportSize({width:414,height:896});
  await expect.poll(()=>f.view.detailDrawer.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
  expect(f.stocks).toHaveLength(0);await f.view.screenshot('product-basic-direct-forms-english-mobile');
 });
 test('TC-64f: 分类候选失败时不清空当前分类，其他资料照常保存',async({adminPage})=>{
  const f=await fixture(adminPage);
  await adminPage.route('**/admin/catalog/categories?*',route=>route.fulfill({status:503,json:{code:503,message:'分类暂时不可读取'}}));
  await open(adminPage,f.view);await copy(adminPage);
  await expect(adminPage.getByTestId('catalog-description-category').locator('input')).toBeDisabled();
  await adminPage.getByTestId('catalog-description-save').click();await expect.poll(()=>f.descriptions.length).toBe(1);
  expect(f.descriptions[0].categoryNo).toBeUndefined();
  await f.view.screenshot('product-category-read-failure-preserves-assignment');
 });
});
