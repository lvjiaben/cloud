import { test, expect } from '@host-tests/fixtures/auth';
import { ensureSourcePluginEnabled } from '@host-tests/fixtures/plugin';
import { waitForTableReady } from '@host-tests/support/ui';

const untranslatedKeyPattern = /\b(?:plugin|pages)\.[A-Za-z0-9_.:-]+\b/g;

test.describe('TC010 通知公告静态文案不再泄漏原始 i18n key', () => {
  test('TC-6a: 英文环境下通知公告页展示翻译后的静态文案', async ({
    adminPage,
    mainLayout,
  }) => {
    await ensureSourcePluginEnabled(adminPage, 'content-notice');
    await mainLayout.switchLanguage('English');

    await adminPage.goto('/system/notice');
    await waitForTableReady(adminPage);

    const bodyText = await adminPage.locator('body').innerText();
    expect(bodyText).toContain('Announcement Title');
    expect(bodyText).toContain('Created By');
    expect([...new Set(bodyText.match(untranslatedKeyPattern) || [])]).toEqual([]);
  });

  test('TC-6b: 运行时语言包接口返回通知公告业务 i18n 资源', async ({
    adminPage,
  }) => {
    const response = await adminPage.request.get(
      '/api/v1/i18n/runtime/messages?lang=en-US',
      {
        headers: {
          'Accept-Language': 'en-US',
        },
      },
    );
    expect(response.ok()).toBeTruthy();
    const payload = await response.json();
    const messages = payload?.data?.messages ?? payload?.messages;

    expect(messages?.plugin?.['content-notice']?.fields?.title).toBe(
      'Announcement Title',
    );
  });
});
