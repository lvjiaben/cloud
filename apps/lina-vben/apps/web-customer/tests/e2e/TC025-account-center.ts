import { expect, test } from '../fixtures/customer';
import { mockCustomerAPIs } from '../support/customer-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

/**
 * Account-center smoke coverage for the Vben customer shell.  The numbered
 * TC025 file keeps the existing module-local sequence intact; it is the
 * implementation of the OpenSpec 15.9 account-center scenario.
 */
test.describe('TC025 客户中心统一入口', () => {
  test('TC025a: 登录后显示分组导航并可进入概览', async ({ page }) => {
    await mockCustomerAPIs(page);
    await page.goto('/auth/login');
    await page.locator('input').nth(0).fill('customer');
    await page.locator('input').nth(1).fill('customer-password');
    await page.getByRole('button', { name: 'login' }).click();
    await page.waitForURL(/\/account(?:$|\/)/u);

    await expect(page.locator('body')).toContainText('产品与服务');
    await expect(page.locator('body')).toContainText('财务管理');
    await expect(page.locator('body')).toContainText('账户管理');
    await expect(page.locator('body')).toContainText('认证与合规');
    await expect(page.locator('body')).toContainText('技术支持');
    await expect(page.locator('body')).toContainText('客户概览');
    await captureCustomerScreenshot(page, 'customer-account-center-zh');
  });

  test('TC025b: 英文移动端保持分组导航且无横向溢出', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await mockCustomerAPIs(page, { locale: 'en-US' });
    await page.goto('/auth/login');
    await page.locator('input').nth(0).fill('customer');
    await page.locator('input').nth(1).fill('customer-password');
    await page.getByRole('button', { name: 'login' }).click();
    await page.waitForURL(/\/account(?:$|\/)/u);

    await expect(page.locator('body')).toContainText('Browse products');
    await expect(page.locator('body')).toContainText('Financial overview');
    await expect(page.locator('body')).toContainText('Account profile');
    await expect(page.locator('body')).toContainText('Privacy & Authorization');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await captureCustomerScreenshot(page, 'customer-account-center-en-mobile');
  });
});
