import { expect, test } from '@playwright/test';

import { PublicSitePage } from '../pages/PublicSitePage';
import { capturePublicScreenshot } from '../support/screenshot';

test.describe('TC003 官网双语和窄屏', () => {
  test('TC003a: 英文首页显示完整公开导航', async ({ page }) => {
    const site = new PublicSitePage(page);
    await site.goto('/en-US');

    await expect(site.home).toContainText('Choose the service that fits your needs');
    await expect(site.shell.getByRole('link', { name: 'Knowledge base' })).toBeVisible();
    await expect(site.localeSwitch).toHaveText('切换到中文');
    await expect(site.document).toHaveAttribute('lang', 'en-US');
    await capturePublicScreenshot(page, 'public-site-home-en');
  });

  test('TC003b: 中文窄屏没有横向溢出', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const site = new PublicSitePage(page);
    await site.goto('/');

    expect(await site.hasHorizontalOverflow()).toBe(false);
    await expect(site.customerCenter).toBeVisible();
    await capturePublicScreenshot(page, 'public-site-home-mobile');
  });
});
