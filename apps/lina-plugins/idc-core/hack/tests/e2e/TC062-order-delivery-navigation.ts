// Staff move from an abnormal order to its bounded fulfillment read scope.
// These fixtures never submit provisioning, verification or refund actions.
import {expect, test} from '../support/catalog-auth';
import {workspacePath} from '@host-tests/fixtures/config';
import {waitForRouteReady} from '@host-tests/support/ui';
import {CommerceOrdersPage} from '../pages/CommerceOrdersPage';
import {ProductFulfillmentPage} from '../pages/ProductFulfillmentPage';
import {installCatalogAutomationMock} from '../support/catalog-automation-mock';
import {commerceAdminOrderNo, installCommerceOrderMock} from '../support/commerce-order-mock';
import {installProductFulfillmentMock} from '../support/product-fulfillment-mock';

const permissions=[
 'idc:admin:commerce:order:list','idc:admin:commerce:order:query',
 'idc:admin:product-fulfillment:task:list','idc:admin:product-fulfillment:task:query',
];

test.describe('TC-62 后台订单开通交付记录入口',()=>{
 test('TC-62a: 本订单筛选、刷新、切换失败清旧记录及清除上下文，不执行恢复动作',async({adminPage})=>{
  await installCommerceOrderMock(adminPage,{deliveryStatus:'manual_processing'});
  const state=await installProductFulfillmentMock(adminPage,{orderNo:commerceAdminOrderNo,strictOrderFilter:true,recovery:true,responseExcerpt:JSON.stringify({httpStatus:200,response:{status:400,msg:'镜像在当前节点未下载'}})});
  await installCatalogAutomationMock(adminPage,{permissions});
  await adminPage.reload();await waitForRouteReady(adminPage);
  const orders=new CommerceOrdersPage(adminPage);await orders.goto();await orders.openDetail(commerceAdminOrderNo);
  const attention=adminPage.getByTestId('order-delivery-attention');
  await expect(attention).toContainText('此订单有交付需要人工处理');
  await expect(attention.getByTestId('order-open-delivery-tasks')).toBeInViewport();
  await orders.screenshot('order-delivery-direct-entry');
  await attention.getByTestId('order-open-delivery-tasks').click();
  const fulfillment=new ProductFulfillmentPage(adminPage);
  await expect(fulfillment.pageRoot).toBeVisible();
  await expect(fulfillment.pageRoot.getByRole('textbox',{name:'订单编号',exact:true})).toHaveValue(commerceAdminOrderNo);
  await expect.poll(()=>state.listRequests.at(-1)?.searchParams.get('orderNo')).toBe(commerceAdminOrderNo);
  await expect(fulfillment.pageRoot).toContainText(commerceAdminOrderNo);
  await adminPage.reload();
  await expect(fulfillment.pageRoot.getByRole('textbox',{name:'订单编号',exact:true})).toHaveValue(commerceAdminOrderNo);
  await expect(fulfillment.pageRoot).toContainText(commerceAdminOrderNo);
  await fulfillment.pageRoot.getByRole('button',{name:/详\s*情/u}).first().click();
  const drawer=adminPage.getByRole('dialog').filter({hasText:commerceAdminOrderNo});
  await expect(drawer.getByTestId('fulfillment-http-response')).toContainText('镜像在当前节点未下载');
  await drawer.getByTestId('fulfillment-http-response').scrollIntoViewIfNeeded();
  await fulfillment.screenshot('order-delivery-matching-response');
  await adminPage.keyboard.press('Escape');
  state.failNextList=true;
  await adminPage.goto(workspacePath('/idc/product-fulfillment/tasks?orderNo=ORD_other_fixture'));
  await expect(fulfillment.pageRoot).toContainText('履约任务暂时加载失败，请重试。');
  await expect(fulfillment.pageRoot.locator('.vxe-table--main-wrapper .vxe-body--row')).toHaveCount(0);
  await expect(fulfillment.pageRoot.getByRole('textbox',{name:'订单编号',exact:true})).toHaveValue('ORD_other_fixture');
  await fulfillment.screenshot('order-delivery-new-scope-failed');
  await adminPage.goBack();
  await expect(fulfillment.pageRoot).toContainText(commerceAdminOrderNo);
  await fulfillment.pageRoot.getByRole('button',{name:/^重\s*置$/u}).click();
  await expect(adminPage).not.toHaveURL(/orderNo=/);
  await expect(fulfillment.pageRoot.getByRole('textbox',{name:'订单编号',exact:true})).toHaveValue('');
  expect(state.actionBodies).toHaveLength(0);
 });

 test('TC-62b: 英文异常订单及目标订单筛选为真实译文',async({adminPage,mainLayout})=>{
  await installCommerceOrderMock(adminPage,{deliveryStatus:'manual_processing'});
  const state=await installProductFulfillmentMock(adminPage,{orderNo:commerceAdminOrderNo,strictOrderFilter:true,recoveryUnavailable:true});
  await installCatalogAutomationMock(adminPage,{permissions});
  await adminPage.reload();await waitForRouteReady(adminPage);await mainLayout.switchLanguage('English');
  const orders=new CommerceOrdersPage(adminPage);await orders.goto();await orders.openDetail(commerceAdminOrderNo);
  const attention=adminPage.getByTestId('order-delivery-attention');
  await expect(attention).toContainText('Delivery on this order needs staff attention.');
  await expect(attention.getByTestId('order-open-delivery-tasks')).toHaveText('Provisioning and delivery records');
  await orders.screenshot('order-delivery-entry-en');
  await attention.getByTestId('order-open-delivery-tasks').click();
  const fulfillment=new ProductFulfillmentPage(adminPage);
  await expect(fulfillment.pageRoot).toContainText(commerceAdminOrderNo);
  await expect(fulfillment.pageRoot.getByRole('textbox',{name:'Order number',exact:true})).toHaveValue(commerceAdminOrderNo);
  await expect(fulfillment.pageRoot).not.toContainText('plugin.idc-core.');
  await fulfillment.screenshot('order-delivery-filter-en');
  expect(state.actionBodies).toHaveLength(0);
 });

 for(const [suffix,missing] of [['c','idc:admin:product-fulfillment:task:list'],['d','idc:admin:product-fulfillment:task:query']] as const){
  test(`TC-62${suffix}: 缺少${missing.endsWith('list')?'列表':'详情'}权限隐藏交付记录入口`,async({adminPage})=>{
   await installCommerceOrderMock(adminPage,{deliveryStatus:'manual_processing'});
   const state=await installProductFulfillmentMock(adminPage);
   await installCatalogAutomationMock(adminPage,{permissions:permissions.filter(value=>value!==missing)});
   await adminPage.reload();await waitForRouteReady(adminPage);
   const orders=new CommerceOrdersPage(adminPage);await orders.goto();await orders.openDetail(commerceAdminOrderNo);
   await expect(adminPage.getByTestId('order-delivery-attention')).toContainText('此订单有交付需要人工处理');
   await expect(adminPage.getByTestId('order-open-delivery-tasks')).toHaveCount(0);
   expect(state.listRequests).toHaveLength(0);expect(state.actionBodies).toHaveLength(0);
   await orders.screenshot('order-delivery-entry-denied-'+suffix);
  });
 }
});
