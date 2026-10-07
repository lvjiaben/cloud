import { expect, test } from '../fixtures/customer';
import { mockCustomerAPIs } from '../support/customer-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

async function loginCustomer(page: import('@playwright/test').Page) {
  await page.goto('/auth/login');
  await page.locator('input').nth(0).fill('customer');
  await page.locator('input').nth(1).fill('customer-password');
  await page.getByRole('button', { name: 'login' }).click();
  await page.waitForURL(/\/account(?:$|\/)/u);
}

test.describe('TC026 客户后台路由与会话边界', () => {
  test('TC026a: 深层客户路由登录后可直接刷新，插件静态入口带不可变版本', async ({
    page,
  }) => {
    await mockCustomerAPIs(page);
    await loginCustomer(page);

    await page.goto('/account/products');
    await page.waitForURL(/\/account\/products\/catalog$/u);
    await expect(page.locator('body')).toContainText('商品');
    await page.reload();
    await expect(page).toHaveURL(/\/account\/products\/catalog$/u);
    await captureCustomerScreenshot(page, 'customer-route-deep-refresh');
  });

  test('TC026b: 密码重置令牌只在客户后台消费，缺少令牌不会提交重置请求', async ({
    page,
  }) => {
    await mockCustomerAPIs(page);
    const resetRequests: unknown[] = [];
    await page.route('**/api/v1/auth/reset-password', async (route) => {
      resetRequests.push(route.request().postDataJSON());
      await route.fulfill({
        json: { code: 0, data: { reset: true }, message: 'OK' },
      });
    });

    await page.goto('/auth/reset-password?token=opaque-reset-token');
    await expect(page.getByTestId('reset-password-page')).toBeVisible();
    const inputs = page.getByTestId('reset-password-page').locator('input');
    await inputs.nth(0).fill('New-password-1!');
    await inputs.nth(1).fill('New-password-1!');
    // The shared Vben form intentionally exposes the stable aria label
    // "submit"; assert the visible localized copy inside the reset page.
    await page
      .getByTestId('reset-password-page')
      .locator('button')
      .filter({ hasText: /确认重置|Confirm Reset/u })
      .click();
    await expect(page).toHaveURL(/\/auth\/login$/u);
    expect(resetRequests).toEqual([
      { password: 'New-password-1!', token: 'opaque-reset-token' },
    ]);

    await page.goto('/auth/reset-password');
    await page.waitForURL(/\/auth\/forget-password$/u);
    expect(resetRequests).toHaveLength(1);
  });

  test('TC026c: 员工误入客户应用的后台路径不装配管理接口，也不产生写入', async ({
    page,
  }) => {
    await mockCustomerAPIs(page);
    // The boundary is checked after authentication: an anonymous visitor
    // should still be allowed to use the ordinary customer login flow.
    await loginCustomer(page);
    const writes: string[] = [];
    page.on('request', (request) => {
      const pathname = new URL(request.url()).pathname;
      if (
        pathname.startsWith('/x/') &&
        ['POST', 'PUT', 'PATCH', 'DELETE'].includes(request.method())
      ) {
        writes.push(`${request.method()} ${pathname}`);
      }
    });

    await page.goto('/admin/customers');
    await expect(page.getByText(/未找到页面|Page Not Found/u)).toBeVisible();
    expect(writes).toEqual([]);
    await expect(page.locator('body')).not.toContainText('客户管理');
    await captureCustomerScreenshot(page, 'customer-admin-route-denied');
  });
});
