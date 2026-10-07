import type { Page, Route } from '@playwright/test';

import { expect, test } from '../fixtures/customer';
import { mockCustomerAPIs } from '../support/customer-api-mock';

const adminOrigin = 'http://127.0.0.1:6031';

function envelope(data: unknown) {
  return JSON.stringify({ code: 0, data, message: 'OK' });
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({
    body: envelope(data),
    contentType: 'application/json',
    status: 200,
  });
}

async function installCustomerShellSettings(page: Page) {
  await page.route('**/api/v1/config/public/frontend', async (route) => {
    await fulfill(route, {
      app: { name: 'IDC 客户中心' },
      auth: {},
      ui: {
        layout: 'header-nav',
        themeMode: 'dark',
        watermarkContent: 'CUSTOMER-WATERMARK',
        watermarkEnabled: true,
      },
      workspace: { basePath: '/' },
    });
  });
}

async function installAdminShellSettings(page: Page) {
  await page.route('**/api/v1/config/public/frontend', async (route) => {
    await fulfill(route, {
      app: { name: 'LinaPro 管理后台' },
      auth: {},
      ui: {
        layout: 'sidebar-nav',
        themeMode: 'light',
        watermarkContent: 'ADMIN-WATERMARK',
        watermarkEnabled: false,
      },
      workspace: { basePath: '/admin' },
    });
  });
  await page.route('**/api/v1/auth/login', async (route) => {
    await fulfill(route, {
      accessToken: 'admin-e2e-token',
      refreshToken: 'admin-e2e-refresh',
      tenants: [],
    });
  });
  await page.route('**/api/v1/user/info', async (route) => {
    await fulfill(route, {
      avatar: '',
      homePath: '/dashboard/analytics',
      id: 9001,
      realName: '测试管理员',
      roles: ['super'],
      username: 'admin',
    });
  });
  await page.route('**/api/v1/menus/all', async (route) => {
    await fulfill(route, {
      list: [
        {
          children: [
            {
              component: '/dashboard/analytics/index',
              meta: { title: 'page.dashboard.analytics' },
              name: 'Analytics',
              path: 'analytics',
            },
          ],
          component: 'BasicLayout',
          meta: { title: 'page.dashboard.title' },
          name: 'Dashboard',
          path: '/dashboard',
          redirect: '/dashboard/analytics',
        },
      ],
    });
  });
}

async function login(page: Page, url: string, username: string) {
  await page.goto(url);
  await page.locator('input').nth(0).fill(username);
  await page.locator('input').nth(1).fill(`${username}-password`);
  await page.getByRole('button', { name: 'login' }).click();
}

async function watermarkCount(page: Page) {
  return page.evaluate(() => {
    return [...document.querySelectorAll<HTMLElement>('body *')].filter(
      (element) =>
        (element as HTMLElement & { __WATERMARK__?: string }).__WATERMARK__ ===
        'watermark',
    ).length;
  });
}

test.describe('TC028 前后台运行壳隔离', () => {
  test('TC028a: 前后台布局、主题、水印和本地偏好互不继承', async ({
    context,
    page,
  }) => {
    await page.addInitScript(() => {
      const foreignPrefix = 'lina-web-antd-5.6.0-dev';
      localStorage.setItem(
        `${foreignPrefix}-preferences-theme`,
        JSON.stringify({ value: 'light' }),
      );
      localStorage.setItem(
        `${foreignPrefix}-preferences-theme-user`,
        JSON.stringify({ value: true }),
      );
    });
    await mockCustomerAPIs(page);
    await installCustomerShellSettings(page);
    await login(page, '/auth/login', 'customer');
    await page.waitForURL(/\/account(?:$|\/)/u);

    await expect(page.locator('html')).toHaveClass(/\bdark\b/u);
    await expect(page.locator('aside')).toHaveCount(0);
    await expect.poll(() => watermarkCount(page)).toBe(1);
    const customerKeys = await page.evaluate(() => Object.keys(localStorage));
    expect(
      customerKeys.some((key) => key.startsWith('lina-web-customer-')),
    ).toBe(true);

    const adminPage = await context.newPage();
    await adminPage.addInitScript(() => {
      const foreignPrefix = 'lina-web-customer-5.6.0-dev';
      localStorage.setItem(
        `${foreignPrefix}-preferences-theme`,
        JSON.stringify({ value: 'dark' }),
      );
      localStorage.setItem(
        `${foreignPrefix}-preferences-theme-user`,
        JSON.stringify({ value: true }),
      );
      localStorage.setItem(
        `${foreignPrefix}-preferences`,
        JSON.stringify({
          value: {
            app: {
              layout: 'header-nav',
              watermark: true,
              watermarkContent: 'CUSTOMER-WATERMARK',
            },
            theme: { mode: 'dark' },
          },
        }),
      );
    });
    await mockCustomerAPIs(adminPage);
    await installAdminShellSettings(adminPage);
    await login(adminPage, `${adminOrigin}/admin/auth/login`, 'admin');
    await adminPage.waitForURL(/\/admin\/dashboard\/analytics$/u);

    await expect(adminPage.locator('html')).not.toHaveClass(/\bdark\b/u);
    await expect(adminPage.locator('aside')).toHaveCount(1);
    await expect.poll(() => watermarkCount(adminPage)).toBe(0);
    const adminKeys = await adminPage.evaluate(() => Object.keys(localStorage));
    expect(adminKeys.some((key) => key.startsWith('lina-web-antd-'))).toBe(
      true,
    );

    await expect(page).toHaveURL(/^http:\/\/127\.0\.0\.1:6032\/account/u);
    await expect(adminPage).toHaveURL(
      /^http:\/\/127\.0\.0\.1:6031\/admin\/dashboard\/analytics/u,
    );
    await expect(page.locator('html')).toHaveClass(/\bdark\b/u);
    await expect.poll(() => watermarkCount(page)).toBe(1);
  });
});
