import { resolve } from 'node:path';
import { test, expect } from '@host-tests/fixtures/auth';
import { FinancialManagementPage } from '../pages/FinancialManagementPage';
import { installFinancialManagementMock } from '../support/financial-management-mock';
import { CustomerWorkspacePage } from '../pages/CustomerWorkspacePage';
import { installCustomerWorkspaceMock, workspaceCustomerNo } from '../support/customer-workspace-mock';

test.describe('TC-56 客户详情余额操作', () => {
 test('TC-56a: 充值与赠送分开、主单位金额、冻结保护与服务端余额变化拒绝', async ({adminPage}) => {
  await installCustomerWorkspaceMock(adminPage);
  await adminPage.route('**/x/idc-core/api/v1/currencies', r => r.fulfill({json:{code:0,data:{currencies:['CNY']}}}));
  const proofs: Record<string,unknown>[] = [], writes: Record<string,unknown>[] = [];
  let failDebit = false;
  await adminPage.route('**/admin/billing/{manual-receipts,balance-adjustments}{,/proofs}', async r => {
   const body = r.request().postDataJSON();
   if (r.request().url().endsWith('/proofs')) {
    proofs.push(body); await r.fulfill({json:{code:0,data:{proof:{proofToken:'test-exact-preview',expiresAt:Date.now()+60000}}}}); return;
   }
   writes.push(body);
   if (failDebit) {await r.fulfill({status:409,json:{code:51,errorCode:'IDC_BILLING_BALANCE_INSUFFICIENT',message:'可用余额不足，请刷新客户余额后重试。'}});return;}
   await r.fulfill({json:{code:0,data:{adjustment:{adjustmentNo:'balance-test',adjustmentTypeCode:r.request().url().includes('manual-receipts')?'manual_receipt':body.deduct?'balance_debit':'balance_credit',statusCode:'posted',customerNo:workspaceCustomerNo,currencyCode:body.currencyCode,amountMinor:body.amountMinor,approvalRequired:false,reused:false}}}});
  });
  const view = new CustomerWorkspacePage(adminPage);
  await view.goto(); await view.screenshot('balance-entry-list'); await view.openOverview(workspaceCustomerNo);
  await expect(adminPage.getByTestId('customer-balance-recharge')).toHaveText('人工充值');
  await expect(adminPage.getByTestId('customer-balance-gift')).toHaveText('赠送余额');
  await expect(adminPage.getByTestId('customer-balance-deduct')).toHaveText('扣减余额');
  await adminPage.getByTestId('customer-balance-gift').click();
  let modal = adminPage.getByRole('dialog').filter({has:adminPage.getByTestId('customer-balance-modal')});
  await expect(modal).toContainText('赠送金额单独进入赠送余额账户，不计作实收，也不可提现。');
  await adminPage.getByTestId('customer-balance-amount').fill('5');
  await expect(modal).toContainText('备注（选填）');
  await expect(modal).toContainText('提交后直接生效，系统自动记录操作人、金额和流水。');
  await view.screenshot('balance-gift-confirm');
  await modal.getByRole('button',{name:/^确\s*认$/}).click({timeout:10000});
  await expect.poll(()=>writes.length).toBe(1);
  expect(writes[0]).toMatchObject({customerNo:workspaceCustomerNo,currencyCode:'CNY',amountMinor:'500',deduct:false,reasonCode:'manual_gift'});
  expect(proofs).toHaveLength(0);
  expect(writes[0]).not.toHaveProperty('proofToken');
  await expect(adminPage.getByText('余额操作已入账，可在资金流水中查看。',{exact:true})).toBeVisible();
  await view.screenshot('balance-gift-posted');
  await expect(modal).toBeHidden();
  await adminPage.getByTestId('customer-balance-deduct').click();
  modal = adminPage.getByRole('dialog').filter({has:adminPage.getByTestId('customer-balance-modal')});
  await expect(modal).toContainText('CNY · 可用余额 170.00');
  await adminPage.getByTestId('customer-balance-amount').fill('170.01');
  await adminPage.getByTestId('customer-balance-reason').fill('更正错误入账');
  await modal.getByRole('button',{name:/^确\s*认$/}).click({timeout:10000});
  await expect(adminPage.getByText('可用现金余额不足；不能扣减冻结金额或其他账户余额。',{exact:true})).toBeVisible();
  expect(writes).toHaveLength(1);
  failDebit=true;
  await adminPage.getByTestId('customer-balance-amount').fill('3');
  await modal.getByRole('button',{name:/^确\s*认$/}).click({timeout:10000});
  await expect.poll(()=>writes.length).toBe(2);
  expect(writes[1]).toMatchObject({deduct:true,amountMinor:'300'});
  await expect(modal).toBeVisible();
  await expect(adminPage.getByTestId('customer-balance-amount')).toHaveValue('3');
  await view.screenshot('balance-server-rejected');
  await modal.getByRole('button',{name:/^取\s*消$/}).click({timeout:10000});
  await expect(modal).toBeHidden();
  await adminPage.getByTestId('customer-balance-recharge').click();
  modal = adminPage.getByRole('dialog').filter({has:adminPage.getByTestId('customer-balance-modal')});
  await expect(modal).toContainText('仅登记已实际到账的线下收款，计入客户现金余额和实收流水。');
  await adminPage.getByTestId('customer-balance-amount').fill('5.01');
  await expect(modal).toContainText('付款证明（选填）');
  await expect(modal).toContainText('收款账户（选填）');
  failDebit=false;
  await modal.getByRole('button',{name:/^确\s*认$/}).click({timeout:10000});
  await expect.poll(()=>writes.length).toBe(3);
  expect(writes[2]).toMatchObject({amountMinor:'501',businessPurposeCode:'balance_recharge',reasonCode:'offline_transfer',proofReference:'',receivingAccountReference:''});
  expect(writes[2]).not.toHaveProperty('deduct');
 });
 test('TC-56b: 财务记录分开显示赠送与扣减，金额及筛选正确',async({adminPage})=>{
  await installFinancialManagementMock(adminPage);
  const requests:string[]=[];
  await adminPage.route('**/admin/billing/adjustments?*',async r=>{
   const kind=new URL(r.request().url()).searchParams.get('adjustmentTypeCode');requests.push(kind||'');
   const rows=[{adjustmentNo:'BAR_GIFT_TEST',adjustmentTypeCode:'balance_credit',amountMinor:'500'},
    {adjustmentNo:'BAR_DEBIT_TEST',adjustmentTypeCode:'balance_debit',amountMinor:'300'}]
    .filter(row=>!kind||row.adjustmentTypeCode===kind)
    .map(row=>({...row,approvalRequired:true,createdAt:1786406400000,updatedAt:1786406400000,currencyCode:'CNY',customerNo:'CUS_FINANCE_0001',customerCode:'10006',customerDisplayName:'财务测试客户',customerPrimaryPhone:'13800138006',requestedBy:1001,statusCode:'pending_review',reasonCode:'manual_adjustment'}));
   await r.fulfill({json:{code:0,data:{adjustments:rows,nextCursor:'',total:rows.length}}});
  });
  const finance=new FinancialManagementPage(adminPage);await finance.goto();
  await expect(finance.pageRoot).toContainText('CNY 9007199254740993.01');
  await finance.pageRoot.getByRole('tab',{name:'调账记录',exact:true}).click();
  const rows=finance.pageRoot.locator('.vxe-table--main-wrapper .vxe-body--row');
  await expect(rows.filter({hasText:'BAR_GIFT_TEST'})).toContainText('人工赠送余额');
  await expect(rows.filter({hasText:'BAR_GIFT_TEST'})).toContainText('CNY 5.00');
  await expect(rows.filter({hasText:'BAR_DEBIT_TEST'})).toContainText('扣减可用余额');
  await expect(rows.filter({hasText:'BAR_DEBIT_TEST'})).toContainText('CNY 3.00');
  await finance.pageRoot.getByRole('combobox',{name:'类型',exact:true}).click();
  const options=adminPage.locator('.ant-select-dropdown:visible');
  await expect(options).toContainText('人工赠送余额');await expect(options).toContainText('扣减可用余额');
  await options.getByText('人工赠送余额',{exact:true}).click();
  await finance.pageRoot.getByRole('button',{name:/搜\s*索/}).click();
  await expect.poll(()=>requests.at(-1)).toBe('balance_credit');
  await expect(rows.filter({hasText:'BAR_DEBIT_TEST'})).toHaveCount(0);
  await expect(adminPage.getByTestId('finance-adjustment-review').filter({visible:true})).toBeVisible();
  await adminPage.screenshot({path:resolve('../../temp/20261018/122000-gift-adjustment-review.png'),fullPage:true});
 });

});
