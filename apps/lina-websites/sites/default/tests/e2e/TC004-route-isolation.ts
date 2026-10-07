import { expect, test } from '@playwright/test';

import { PublicSitePage } from '../pages/PublicSitePage';

test.describe('TC004 官网路由边界', () => {
  test('TC004a: 公开商品深层路由可以直接刷新', async ({ page }) => {
    const site = new PublicSitePage(page);
    await site.goto('/products/managed-ssl');
    await expect(site.shell).toBeVisible();
    await expect(page).toHaveURL(/\/products\/managed-ssl$/u);
  });

  test('TC004b: 官网没有客户私有页面', async ({ request }) => {
    for (const path of ['/account', '/auth/login', '/cart', '/checkout']) {
      const response = await request.get(path, { maxRedirects: 0 });
      expect(response.status(), path).toBe(404);
      expect(await response.text(), path).not.toContain('public-home');
    }
  });

  test('TC004c: API 和静态缺失路径不返回官网页面', async ({ request }) => {
    for (const path of ['/api/v1/public-site-probe', '/x-assets/public-site-probe.js']) {
      const response = await request.get(path);
      expect(response.status(), path).toBe(404);
      expect(await response.text(), path).not.toContain('data-testid="public-site-shell"');
    }
  });
});
