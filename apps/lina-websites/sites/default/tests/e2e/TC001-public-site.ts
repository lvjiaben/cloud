import { expect, test } from '@playwright/test';

import { PublicSitePage } from '../pages/PublicSitePage';
import { testRuntime } from '../support/runtime-config';
import { capturePublicScreenshot } from '../support/screenshot';

test.describe('TC001 公开官网外壳', () => {
  test('TC001a: 中文首页只展示公开入口', async ({ page }) => {
    const site = new PublicSitePage(page);
    await site.goto('/');

    await expect(site.home).toContainText('找到适合你的商品和服务');
    await expect(site.home).toContainText('不再假设所有商品都是服务器');
    await expect(site.shell.getByRole('link', { name: '商品', exact: true })).toBeVisible();
    await expect(site.shell).not.toContainText('购物车');
    await expect(site.shell).not.toContainText('我的订单');
    await expect(site.shell).not.toContainText('实名认证');
    await expect(site.document).toHaveAttribute('lang', 'zh-CN');
    await capturePublicScreenshot(page, 'public-site-home-zh');
  });

  test('TC001b: 登录和客户中心只指向独立客户应用', async ({ page }) => {
    const site = new PublicSitePage(page);
    await site.goto('/');

    await expect(site.signIn).toHaveAttribute(
      'href',
      new URL('/auth/login', testRuntime.customerURL).toString(),
    );
    await expect(site.customerCenter).toHaveAttribute(
      'href',
      new URL('/account', testRuntime.customerURL).toString(),
    );
  });
});
