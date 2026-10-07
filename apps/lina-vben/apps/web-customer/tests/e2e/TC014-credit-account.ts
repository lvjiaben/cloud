import { Buffer } from 'node:buffer';

import { CreditPage } from '../pages/CreditPage';
import { FinancePage } from '../pages/FinancePage';
import {
  creditInvoiceNo,
  creditStatementNo,
  installCustomerCreditMock,
} from '../support/customer-credit-api-mock';
import {
  balanceInvoiceNo,
  installCustomerFinanceMock,
} from '../support/customer-finance-api-mock';
import { expect, test } from '../fixtures/customer';

test.describe('TC014 客户授信账户', () => {
  test('TC014f: 信用目录被拒绝时清空对应行及总数，恢复后可重新读取', async ({ page }) => {
    const state = await installCustomerCreditMock(page);
    state.rowCount = 23;
    const credit = new CreditPage(page);
    await credit.login();
    await credit.open();
    for (const [directory, label] of [['applications', '授信申请'], ['statements', '信用账单']] as const) {
      await page.getByRole('tab', { name: label, exact: true }).click();
      const grid = credit.root.locator('.vxe-grid:visible');
      const rows = grid.locator('.vxe-table--main-wrapper .vxe-body--row');
      await expect(rows).toHaveCount(10);
      state.deniedDirectories[directory] = 403;
      await grid.locator('.vxe-pager--next-btn').click();
      await expect(rows).toHaveCount(0);
      await expect(grid.locator('.vxe-pager--total')).toContainText('0');
      delete state.deniedDirectories[directory];
      await credit.root.getByRole('button', { name: '重新加载' }).click();
      await expect(rows).toHaveCount(10);
      await expect(grid.locator('.vxe-pager--total')).toContainText('23');
    }
    await credit.screenshot('customer-credit-access-restored');
  });

  test('TC014g: 还款权限拒绝后清除账单和还款金额，不提交资金操作', async ({ page }) => {
    const state = await installCustomerCreditMock(page);
    const credit = new CreditPage(page);
    await credit.login();
    await credit.open();
    await page.getByRole('tab', { name: '信用账单', exact: true }).click();
    await page.getByTestId('credit-repay-open').click();
    await page.getByTestId('credit-repayment-amount').fill('88.00');
    state.deniedRepayment = true;
    await page.getByRole('button', { name: /确\s*[认定]/u }).click();
    await expect(page.getByTestId('credit-repayment-modal')).toBeHidden();
    await expect.poll(async () => {
      const input = page.getByTestId('credit-repayment-amount');
      return await input.count() ? input.inputValue() : '';
    }).toBe('');
    await expect(credit.root.locator('.vxe-grid:visible .vxe-body--row')).toHaveCount(0);
    await expect(credit.root.locator('.vxe-grid:visible .vxe-pager--total')).toContainText('0');
    await page.getByRole('tab', { name: '授信申请', exact: true }).click();
    await expect(credit.root.locator('.vxe-grid:visible')).toContainText('CAP_CREDIT_0001');
    expect(state.proofBodies).toHaveLength(0);
    expect(state.repaymentBodies).toHaveLength(0);
    await credit.screenshot('customer-credit-repayment-access-cleared');
  });
  test('TC014e: 申请和账单使用准确总数完整分页并可重试失败页', async ({ page }) => {
    const pageErrors: string[] = [];
    page.on('pageerror', (error) => pageErrors.push(error.message));
    const state = await installCustomerCreditMock(page);
    state.rowCount = 123;
    const credit = new CreditPage(page);
    await credit.login();
    await credit.open();
    for (const [directory, label, number] of [
      ['applications', '授信申请', 'CAP_CREDIT_0001'],
      ['statements', '信用账单', creditStatementNo],
    ] as const) {
      await page.getByRole('tab', { name: label, exact: true }).click();
      const grid = credit.root.locator('.vxe-grid:visible');
      const rows = grid.locator('.vxe-table--main-wrapper .vxe-body--row');
      await expect(grid.locator('.vxe-pager--total')).toContainText('123');
      await expect(rows).toHaveCount(10);
      await grid.locator('.vxe-pager--end-btn').click();
      await expect(rows.first()).toContainText(`${number}_121`);
      await expect(rows).toHaveCount(3);
      await expect.poll(() => state.listRequests.filter((request) => request.directory === directory).at(-1)).toMatchObject({ page: 13, limit: 10 });
      state.failLists = true;
      await grid.locator('.vxe-pager--prev-btn').click();
      await expect(credit.root.getByRole('alert')).toBeVisible();
      await expect(grid.locator('.vxe-pager--num-btn.is--active')).toHaveText('13');
      await expect(rows.first()).toContainText(`${number}_121`);
      state.failLists = false;
      await credit.root.getByRole('button', { name: '重新加载' }).click();
      await expect(rows.first()).toContainText(`${number}_111`);
      for (const size of [20, 50, 100, 10]) {
        await grid.locator('.vxe-pager--sizes').click();
        await page.locator('.vxe-select-option:visible').filter({ hasText: new RegExp(`^${size}(?:\\s|条|/)`) }).click({ timeout: 10_000 });
        await expect.poll(() => state.listRequests.filter((request) => request.directory === directory).at(-1)).toMatchObject({ page: 1, limit: size });
        await expect(rows.first()).toContainText(number);
      }
    }
    await credit.root.locator('.vxe-grid:visible .vxe-pager').scrollIntoViewIfNeeded();
    await expect(credit.root.locator('.vxe-grid:visible .vxe-pager')).toBeInViewport();
    await credit.screenshot('customer-credit-complete-pagination');
    state.rowCount = 0;
    await credit.open();
    await page.getByRole('tab', { name: '信用账单', exact: true }).click();
    await expect(credit.root.locator('.vxe-grid:visible .vxe-body--row')).toHaveCount(0);
    await expect(credit.root.locator('.vxe-grid:visible .vxe-pager--total')).toContainText('0');
    await credit.root.locator('.vxe-grid:visible .vxe-pager').scrollIntoViewIfNeeded();
    await expect(credit.root.locator('.vxe-grid:visible .vxe-pager')).toBeInViewport();
    await credit.screenshot('customer-credit-pagination-empty');
    expect(pageErrors).toEqual([]);
  });
  test('TC014a: 客户上传私有材料并提交不含客户编号的授信申请', async ({
    page,
  }) => {
    const state = await installCustomerCreditMock(page);
    const credit = new CreditPage(page);
    await credit.login();
    await credit.open();
    await page.getByTestId('credit-apply-open').click();
    await page.getByTestId('credit-application-limit').fill('1000.00');
    await page.locator('input[type="file"]').setInputFiles({
      buffer: Buffer.from('%PDF-1.4\n%%EOF'),
      mimeType: 'application/pdf',
      name: 'bank-statement.pdf',
    });
    await page.getByRole('button', { name: /确\s*认/ }).click();
    await expect.poll(() => state.applicationBodies.length).toBe(1);
    expect(state.reserveBodies).toEqual([
      { fileName: 'bank-statement.pdf', sizeBytes: 14 },
    ]);
    expect(state.uploadBodies[0]?.byteLength).toBe(14);
    expect(state.applicationBodies[0]).toMatchObject({
      currencyCode: 'CNY',
      requestedLimitMinor: '100000',
      uploadNos: ['CEU_CREDIT_1'],
    });
    expect(state.applicationBodies[0]).not.toHaveProperty('customerNo');
    await expect(page.getByTestId('credit-application-modal')).toBeHidden();
    await credit.screenshot('customer-credit-application-zh');
  });

  test('TC014b: 余额还款的预览和执行使用同一请求且释放欠款', async ({
    page,
  }) => {
    const state = await installCustomerCreditMock(page);
    const credit = new CreditPage(page);
    await credit.login();
    await credit.open();
    // The sidebar and the credit page both expose this label; target the tab
    // explicitly so the repayment panel is opened in the statement view.
    await page.getByRole('tab', { name: '信用账单' }).click();
    await page.getByTestId('credit-repay-open').click();
    await page.getByTestId('credit-repayment-amount').fill('88.00');
    // The shared modal renders the localized action as “确 认” (with a
    // typographic space); accept both legacy “确定” and current wording.
    await page.getByRole('button', { name: /确\s*[认定]/u }).click();
    await expect.poll(() => state.repaymentBodies.length).toBe(1);
    expect(state.proofBodies[0]).toMatchObject({ amountMinor: '8800' });
    expect(state.repaymentBodies[0]).toMatchObject({
      amountMinor: '8800',
      proofToken: 'opaque-credit-repayment-proof',
    });
    expect(state.repaymentBodies[0]?.requestNo).toBe(
      state.proofBodies[0]?.requestNo,
    );
    expect(state.repaymentBodies[0]).not.toHaveProperty('customerNo');
    await expect(
      page.locator(`tr:has-text("${creditStatementNo}")`),
    ).toContainText('已还清');
  });

  test('TC014c: 授信支付确认显示金额和剩余额度', async ({ page }) => {
    await installCustomerFinanceMock(page);
    const state = await installCustomerCreditMock(page, { baseMock: false });
    const finance = new FinancePage(page);
    await finance.login();
    await finance.open(`/account/billing/invoices/${balanceInvoiceNo}`);
    await page.getByTestId('finance-credit-pay').click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toContainText('CNY 88.00');
    await expect(dialog).toContainText('CNY 412.00');
    await dialog.getByRole('button', { name: '确认信用支付' }).click();
    await expect.poll(() => state.creditPayments.length).toBe(1);
    expect(state.creditPayments).toEqual([creditInvoiceNo]);
    await expect(
      page.getByText('信用支付成功，订单已取得财务结清资格。'),
    ).toBeVisible();
  });

  test('TC014d: 英文手机端明确显示逾期账单和冻结账户', async ({ page }) => {
    await page.setViewportSize({ height: 844, width: 390 });
    await installCustomerCreditMock(page, { frozen: true, locale: 'en-US' });
    const credit = new CreditPage(page);
    await credit.login();
    await credit.open();
    await expect(credit.root).toContainText('Frozen');
    await page.getByText('Statements', { exact: true }).click();
    await expect(credit.root).toContainText('Overdue');
    expect(await credit.hasHorizontalOverflow()).toBe(false);
    await credit.screenshot('customer-credit-overdue-en-mobile');
  });
});
