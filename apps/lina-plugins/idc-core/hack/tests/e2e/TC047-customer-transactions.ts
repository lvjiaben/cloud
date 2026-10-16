// Exact owner detail reads must remain reachable from the customer workspace.
import { expect, test } from '../support/catalog-auth';
import { config } from '@host-tests/fixtures/config';
import { CustomerTransactionPage } from '../pages/CustomerTransactionPage';
import { installCustomerWorkspaceMock, workspaceCustomerNo } from '../support/customer-workspace-mock';
import { commerceAdminOrderNo, installCommerceOrderMock } from '../support/commerce-order-mock';
import { financialInvoiceNo, installFinancialManagementMock } from '../support/financial-management-mock';

test.describe('TC-47 客户交易明细衔接', () => {
  for (const kind of ['order','invoice'] as const) {
    test(`TC-47${kind === 'order' ? 'a' : 'b'}: ${kind === 'order' ? '订单' : '账单'}直接打开准确明细并保留复制`, async ({adminPage}) => {
      await installCommerceOrderMock(adminPage);
      await installFinancialManagementMock(adminPage);
      await installCustomerWorkspaceMock(adminPage);
      const view = new CustomerTransactionPage(adminPage);
      const reference = kind === 'order' ? commerceAdminOrderNo : financialInvoiceNo;
      await view.goto(); await view.openOverview(workspaceCustomerNo); await view.showRecords(kind);
      await expect(view.overviewDrawer.getByRole('button',{name:reference,exact:true})).toBeVisible();
      await expect(view.detailButton(kind,reference)).toHaveText('详情');
      await view.openRecord(kind,reference);
      await expect(view.recordDialog).toContainText(reference);
      await expect(view.recordDialog).toContainText('CNY 9007199254740993.01');
      await view.screenshot(`customer-${kind}-linked-detail`);
    });
  }
  test('TC-47c: 只有列表权限不显示明细入口，拼接链接也不请求明细',async ({adminPage}) => {
    const orders = await installCommerceOrderMock(adminPage);
    await installFinancialManagementMock(adminPage);
    await installCustomerWorkspaceMock(adminPage);
    await adminPage.route('**/api/v1/user/info',async route => {
      const response = await route.fetch({url:`${config.apiBaseURL}user/info`}); const data = await response.json();
      data.data.permissions=['idc:admin:customer:list','idc:admin:customer:query','idc:admin:commerce:order:list','idc:admin:billing:invoice:list'];
      await route.fulfill({response,json:data});
    });
    const view = new CustomerTransactionPage(adminPage);
    await view.goto(); await view.openOverview(workspaceCustomerNo); await view.showRecords('order');
    await expect(view.overviewDrawer.getByRole('button',{name:commerceAdminOrderNo,exact:true})).toBeVisible();
    await expect(view.detailButton('order',commerceAdminOrderNo)).toHaveCount(0);
    await view.showRecords('invoice');
    await expect(view.overviewDrawer.getByRole('button',{name:financialInvoiceNo,exact:true})).toBeVisible();
    await expect(view.detailButton('invoice',financialInvoiceNo)).toHaveCount(0);
    let invoiceReads = 0;
    await adminPage.route(`**/x/idc-core/api/v1/admin/billing/invoices/${financialInvoiceNo}**`,async route=>{invoiceReads+=1;await route.fallback();});
    await view.gotoRecord('invoice',financialInvoiceNo);
    await expect(view.page.getByTestId('financial-management-page')).toBeVisible();
    await expect(view.recordDialog).toHaveCount(0);
    expect(invoiceReads).toBe(0);
    await view.gotoRecord('order',commerceAdminOrderNo);
    await expect.poll(()=>orders.listRequests.length).toBeGreaterThan(0);
    expect(orders.detailRequests).toHaveLength(0);
    await expect(view.recordDialog).toHaveCount(0);
    await view.screenshot('customer-transaction-read-permission');
  });
  for (const kind of ['order','invoice'] as const) {
    test(`TC-47${kind === 'order' ? 'd' : 'e'}: 明细读取失败显示中文错误并可退出`, async ({adminPage}) => {
      await installCommerceOrderMock(adminPage);
      await installFinancialManagementMock(adminPage);
      await installCustomerWorkspaceMock(adminPage);
      const reference = kind === 'order' ? commerceAdminOrderNo : financialInvoiceNo;
      const ownerPath = kind === 'order' ? 'commerce/orders' : 'billing/invoices';
      let failedOnce = false;
      await adminPage.route(`**/x/idc-core/api/v1/admin/${ownerPath}/${reference}**`,async route=>{
        if (failedOnce) {await route.fallback();return;}
        failedOnce=true;
        await route.fulfill({json:{code:500,message:'暂时无法读取，请重试',data:null}});
      });
      const view = new CustomerTransactionPage(adminPage);
      await view.goto(); await view.openOverview(workspaceCustomerNo); await view.showRecords(kind);
      await view.openRecord(kind,reference);
      await expect(view.recordDialog).toContainText(kind === 'order' ? '订单详情暂时加载失败' : '财务详情暂时无法加载');
      await expect(view.recordDialog).not.toContainText('9007199254740993.01');
      await view.screenshot(`customer-${kind}-detail-failed`);
      await view.retryRecord();
      await expect(view.recordDialog).toContainText(reference);
      await expect(view.recordDialog).toContainText('CNY 9007199254740993.01');
      await view.closeRecord();
    });
  }

  test('TC-47f: 关闭旧账单后迟到响应不覆盖新账单',async ({adminPage})=>{
    await installFinancialManagementMock(adminPage);
    let release: (()=>void) | undefined;
    const gate=new Promise<void>(resolve=>{release=resolve;});
    let started=false;
    let replied=false;
    await adminPage.route('**/x/idc-core/api/v1/admin/billing/invoices/*',async route=>{
      const number=new URL(route.request().url()).pathname.split('/').at(-1)!;
      if(number===financialInvoiceNo) {started=true;await gate;}
      await route.fulfill({json:{code:0,message:'success',data:{detail:{invoice:{invoiceNo:number,customerNo:workspaceCustomerNo,customerDisplayName:'测试客户',currencyCode:'CNY',statusCode:'pending_payment',outstandingAmountMinor:'100',totalAmountMinor:'100'},items:[],nextItemCursor:''}}}});
      if(number===financialInvoiceNo) replied=true;
    });
    const view=new CustomerTransactionPage(adminPage);
    try {
      await view.gotoRecord('invoice','');
      await view.openInvoiceFromList(financialInvoiceNo);
      await expect.poll(()=>started).toBe(true);
      await view.closeRecord();
      await view.openInvoiceFromList('INV_FINANCE_PAGE_1');
      await expect(view.recordDialog).toContainText('INV_FINANCE_PAGE_1');
      release!();
      await expect.poll(()=>replied).toBe(true);
      await expect(view.recordDialog).toContainText('INV_FINANCE_PAGE_1');
      await expect(view.recordDialog).not.toContainText(financialInvoiceNo);
      await view.screenshot('customer-invoice-late-response-protected');
    } finally {release?.();}
  });

});
