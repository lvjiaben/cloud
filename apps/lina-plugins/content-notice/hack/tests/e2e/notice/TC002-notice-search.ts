import { test, expect } from '@host-tests/fixtures/auth';
import { ensureSourcePluginEnabled } from '@host-tests/fixtures/plugin';
import { NoticePage } from '../../pages/NoticePage';

test.describe('TC002 通知公告搜索筛选', () => {
  test.beforeEach(async ({ adminPage }) => {
    await ensureSourcePluginEnabled(adminPage, 'content-notice');
  });

  test('TC002a: 按标题搜索', async ({ adminPage }) => {
    const noticePage = new NoticePage(adminPage);
    await noticePage.goto();
    await noticePage.fillSearchField('公告标题', '系统升级');
    await noticePage.clickSearch();

    const hasNotice = await noticePage.hasNotice('系统升级通知');
    expect(hasNotice).toBeTruthy();
  });

  test('TC002c: 重置搜索条件', async ({ adminPage }) => {
    const noticePage = new NoticePage(adminPage);
    await noticePage.goto();

    const totalBefore = await noticePage.getTotalCount();

    await noticePage.fillSearchField('公告标题', 'NONEXISTENT');
    await noticePage.clickSearch();

    const totalAfter = await noticePage.getTotalCount();
    expect(totalAfter).toBeLessThanOrEqual(totalBefore);

    await noticePage.clickReset();
    const totalReset = await noticePage.getTotalCount();
    expect(totalReset).toBe(totalBefore);
  });
});
