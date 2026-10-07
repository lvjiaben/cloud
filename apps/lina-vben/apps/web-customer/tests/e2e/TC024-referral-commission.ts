import { expect, test } from '../fixtures/customer';

import { installCustomerSalesChannelMock } from '../support/customer-sales-channel-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

test.describe('TC024 客户推荐与佣金', () => {
  test('TC024a: 推荐链接和归因统计仅展示当前客户，且佣金结算先冻结金额', async ({ page }) => {
    const state = await installCustomerSalesChannelMock(page);
    await page.goto('/auth/login');
    await page.locator('input').nth(0).fill('customer');
    await page.locator('input').nth(1).fill('customer-password');
    await page.getByRole('button', { name: 'login' }).click();
    await page.waitForURL(/\/account/u);

    await page.goto('/account/referrals');
    const root = page.getByTestId('customer-sales-channel-page');
    await expect(root).toContainText('我的推荐关系');
    await expect(root).toContainText('REF-CUSTOMER-001');
    await expect(root).toContainText('已推荐客户');
    await captureCustomerScreenshot(page, 'customer-referrals-zh');
    await page.getByRole('button', { name: '我的佣金' }).click();
    await expect(page).toHaveURL(/\/account\/commissions$/u);
    await expect(root).toContainText('可结算佣金');
    await expect(root).toContainText('CNY');
    await captureCustomerScreenshot(page, 'customer-commissions-zh');

    await page.getByPlaceholder('申请金额（主币种）').fill('999');
    await page.getByTestId('customer-commission-settlement').click();
    expect(state.settlementRequests).toBe(0);
    await page.getByPlaceholder('申请金额（主币种）').fill('50');
    await page.getByTestId('customer-commission-settlement').click();
    await expect(root).toContainText('结算申请记录');
    await expect(root).toContainText('SETTLE-001');
    await captureCustomerScreenshot(page, 'customer-settlement-requested-zh');
    expect(state.settlementRequests).toBe(1);
  });

  test('TC024b: English mobile page renders translated referral and commission labels', async ({ page }) => {
    await installCustomerSalesChannelMock(page, { locale: 'en-US' });
    await page.goto('/auth/login');
    await page.locator('input').nth(0).fill('customer');
    await page.locator('input').nth(1).fill('customer-password');
    await page.getByRole('button', { name: 'login' }).click();
    await page.waitForURL(/\/account/u);
    await page.goto('/account/commissions');
    const root = page.getByTestId('customer-sales-channel-page');
    await expect(root).toContainText('My commissions');
    await expect(root).toContainText('Available to settle');
    await expect(root).not.toContainText('salesChannel.customer.');
    await captureCustomerScreenshot(page, 'customer-commissions-en-mobile');
  });
});
