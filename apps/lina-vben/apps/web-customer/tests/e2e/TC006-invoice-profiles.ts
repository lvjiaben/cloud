import { InvoiceProfilesPage } from '../pages/InvoiceProfilesPage';
import { installInvoiceProfileMock } from '../support/invoice-profile-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';
import { expect, test } from '../fixtures/customer';

test.describe('TC006 客户发票抬头', () => {
  test.beforeEach(async ({ page }) => {
    await installInvoiceProfileMock(page);
    const profiles = new InvoiceProfilesPage(page);
    await profiles.login();
    await profiles.open();
  });

  test('TC006a: 新增和编辑只显示银行账号掩码，默认抬头停用后不自动改选', async ({
    page,
  }) => {
    const profiles = new InvoiceProfilesPage(page);
    await expect(profiles.root).toContainText('银行账号不会在页面回显');

    await profiles.addEnterprise({
      bankAccount: '6222000012349999',
      bankName: '客户银行',
      invoiceTitle: '新建客户企业',
      taxNo: 'TAX-CREATED',
    });
    const created = profiles.profile('ivp-created-3');
    await expect(created).toContainText('****9999');
    await expect(created).not.toContainText('6222000012349999');

    await profiles.replaceBank('ivp-created-3', '6222000088887777');
    await expect(created).toContainText('****7777');
    await expect(created).not.toContainText('6222000088887777');

    await profiles.setDefault('ivp-created-3');
    await expect(created).toContainText('默认');
    await profiles.deactivate('ivp-created-3');
    await expect(created).toContainText('已停用');
    await expect(page.getByText('默认', { exact: true })).toHaveCount(0);
    await captureCustomerScreenshot(page, 'customer-invoice-profile-deactivated-zh-cn');
  });
});

test.describe('TC006 customer invoice profiles on English mobile', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC006b: English mobile layout has no untranslated keys or horizontal overflow', async ({
    page,
  }) => {
    await installInvoiceProfileMock(page, { locale: 'en-US' });
    const profiles = new InvoiceProfilesPage(page);
    await profiles.login();
    await profiles.open();
    await expect(profiles.root).toContainText('Manage reusable invoice profiles');
    await expect(profiles.root).not.toContainText('invoiceProfiles.');
    expect(await profiles.hasHorizontalOverflow()).toBe(false);
    await captureCustomerScreenshot(page, 'customer-invoice-profiles-mobile-en-us');
  });
});
