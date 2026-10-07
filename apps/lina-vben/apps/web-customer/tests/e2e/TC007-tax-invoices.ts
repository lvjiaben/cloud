import { TaxInvoicesPage } from '../pages/TaxInvoicesPage';
import { installTaxInvoiceMock } from '../support/tax-invoice-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';
import { expect, test } from '../fixtures/customer';

test.describe('TC007 客户部分与合并开票', () => {
  test.beforeEach(async ({ page }) => {
    await installTaxInvoiceMock(page);
    const invoices = new TaxInvoicesPage(page);
    await invoices.login();
    await invoices.open();
  });

  test('TC007a: 同币种两张账单分别填写金额后进入待后台审核', async ({
    page,
  }) => {
    const invoices = new TaxInvoicesPage(page);
    await expect(invoices.root).toContainText('票种由后台确认');
    await invoices.selectBill('BILL-CNY-001', '60.00');
    await expect(
      page.getByTestId('tax-invoice-select-BILL-USD-001'),
    ).toBeDisabled();
    await invoices.selectBill('BILL-CNY-002', '20.00');
    await expect(invoices.root).toContainText('申请总额 CNY 80.00');
    await invoices.submit();
    const created = invoices.historyItem('TINV-CREATED');
    await expect(created).toContainText('待后台审核');
    await expect(created).toContainText('待后台确认票种');
    await expect(created).toContainText('CNY 80.00');
    await expect(invoices.historyItem('TINV-REJECTED')).toContainText(
      '申请金额或所选账单需要调整',
    );
    await captureCustomerScreenshot(page, 'customer-tax-invoice-created-zh-cn');
  });
});

test.describe('TC007 customer tax invoices on English mobile', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC007b: English mobile page is translated and has no horizontal overflow', async ({
    page,
  }) => {
    await installTaxInvoiceMock(page, { locale: 'en-US' });
    const invoices = new TaxInvoicesPage(page);
    await invoices.login();
    await invoices.open();
    await expect(invoices.root).toContainText(
      'Request an invoice and track its progress',
    );
    await expect(invoices.root).not.toContainText('taxInvoices.');
    expect(await invoices.hasHorizontalOverflow()).toBe(false);
    await captureCustomerScreenshot(page, 'customer-tax-invoices-mobile-en-us');
  });
});
