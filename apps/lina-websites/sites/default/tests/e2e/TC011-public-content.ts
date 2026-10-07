import type { Server } from 'node:http';

import { expect, test } from '@playwright/test';

import { PublicContentPageObject } from '../pages/PublicContentPage';
import {
  mockPublicContent,
  startContentMockServer,
} from '../support/content-mock-server';
import { capturePublicScreenshot } from '../support/screenshot';

let contentServer: Server | undefined;
let stopContentServer: (() => Promise<void>) | undefined;

test.beforeAll(async () => {
  const running = await startContentMockServer();
  contentServer = running.server;
  stopContentServer = running.close;
});

test.afterAll(async () => {
  await stopContentServer?.();
  contentServer = undefined;
});

test.describe('TC011 公开内容、SEO 与下载', () => {
  test.beforeEach(async ({ page }) => {
    await mockPublicContent(page);
  });

  test('TC011a: 新闻列表和详情由 SSR 输出后台原文', async ({ page, request }) => {
    const raw = await request.get('/pages/news/platform-update');
    expect(raw.status()).toBe(200);
    expect(await raw.text()).toContain('后台录入的 English + 中文正文');

    const contentPage = new PublicContentPageObject(page);
    await contentPage.goto('/pages/news');
    await expect(page.getByTestId('public-content-category')).toContainText(
      '平台动态',
    );
    await contentPage.goto('/en-US/pages/news/platform-update');
    await expect(contentPage.detail).toContainText('后台录入的 Platform Update 标题');
    await expect(contentPage.detail).toContainText('后台录入的 English + 中文正文');
    await expect(page.getByTestId('public-content-detail-category')).toContainText(
      'Platform updates',
    );
    await expect(page.locator('link[rel="canonical"]')).toHaveAttribute('href', /\/en-US\/pages\/news\/platform-update$/u);
    await expect(page.locator('link[hreflang="zh-CN"]')).toHaveCount(1);
    const structuredData = await page
      .locator('script[type="application/ld+json"]')
      .textContent();
    expect(structuredData).toContain('NewsArticle');
    await capturePublicScreenshot(page, 'public-news-detail');
  });

  test('TC011b: 公告不显示分类，帮助页复用公开知识内容', async ({ page }) => {
    const contentPage = new PublicContentPageObject(page);
    await contentPage.goto('/pages/announcements');
    await expect(contentPage.list).toContainText('后台录入的 Platform Update 标题');
    await expect(contentPage.list).not.toContainText('公告不应显示的分类');
    await expect(page.getByTestId('public-content-category')).toHaveCount(0);
    await contentPage.goto('/pages/help');
    await expect(contentPage.list).toContainText('后台录入的 Platform Update 标题');
    await expect(page.getByTestId('public-content-category')).toContainText(
      '未分类',
    );
    await contentPage.goto('/en-US/pages/help');
    await expect(page.getByTestId('public-content-category')).toContainText(
      'Uncategorized',
    );
    await capturePublicScreenshot(page, 'public-help-list');
  });

  test('TC011c: 下载使用内容编号换取短期令牌后打开真实下载地址', async ({ page }) => {
    let ticketCalls = 0;
    await page.route('**/x/idc-core/api/v1/public/content/items/download-file/download-tickets', async (route) => {
      ticketCalls += 1;
      await route.fulfill({
        body: JSON.stringify({
          code: 0,
          data: { expiresAt: 1_786_320_900_000, token: 'download-token' },
        }),
        contentType: 'application/json',
        status: 200,
      });
    });
    await page.route('**/x/idc-core/api/v1/public/content/downloads/download-token', async (route) => {
      await route.fulfill({
        body: 'mock-download',
        contentType: 'application/octet-stream',
        headers: { 'content-disposition': 'attachment; filename="lina-agent.iso"' },
        status: 200,
      });
    });
    const contentPage = new PublicContentPageObject(page);
    await contentPage.goto('/pages/downloads/download-file');
    const download = page.waitForEvent('download');
    await page.getByTestId('public-download-action').click();
    expect((await download).suggestedFilename()).toBe('lina-agent.iso');
    expect(ticketCalls).toBe(1);
    await capturePublicScreenshot(page, 'public-download-current-file');
  });

  test('TC011d: 下线或删除内容返回真实 404，不泄露旧正文', async ({ page, request }) => {
    const response = await request.get('/pages/news/missing');
    expect(response.status()).toBe(404);
    const html = await response.text();
    expect(html).toContain('该内容不存在或已下线');
    expect(html).not.toContain('后台录入的 Platform Update 标题');
    const browserResponse = await page.goto('/pages/news/missing');
    expect(browserResponse?.status()).toBe(404);
    await expect(page.locator('body')).toContainText('该内容不存在或已下线');
    await expect(page.locator('body')).not.toContainText('后台录入的 Platform Update 标题');
    await capturePublicScreenshot(page, 'public-content-not-found');
  });

  test('TC011e: 移动端公开内容没有横向溢出', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const contentPage = new PublicContentPageObject(page);
    await contentPage.goto('/pages/knowledge');
    expect(await contentPage.hasHorizontalOverflow()).toBe(false);
    await capturePublicScreenshot(page, 'public-content-mobile');
  });
});
