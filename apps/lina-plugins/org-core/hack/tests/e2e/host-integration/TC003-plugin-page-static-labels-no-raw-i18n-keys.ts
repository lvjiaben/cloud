import { test, expect } from '@host-tests/fixtures/auth';
import { ensureSourcePluginEnabled } from '@host-tests/fixtures/plugin';
import { waitForTableReady } from '@host-tests/support/ui';

const untranslatedKeyPattern = /\b(?:plugin|pages)\.[A-Za-z0-9_.:-]+\b/g;

const pluginAuditCases = [
  {
    path: '/system/dept',
    visibleTexts: ['Dept Name', 'Actions'],
  },
  {
    path: '/system/post',
    visibleTexts: ['Position List', 'Actions'],
  },
] as const;

test.describe('TC003 源插件静态文案不再泄漏原始 i18n key', () => {
  test('TC-3a: 英文环境下组织插件页面展示翻译后的静态文案', async ({
    adminPage,
    mainLayout,
  }) => {
    test.setTimeout(180_000);
    await ensureSourcePluginEnabled(adminPage, 'org-core');
    await mainLayout.switchLanguage('English');

    for (const pluginAuditCase of pluginAuditCases) {
      await test.step(pluginAuditCase.path, async () => {
        await adminPage.goto(pluginAuditCase.path);
        await waitForTableReady(adminPage);

        const bodyText = await adminPage.locator('body').innerText();
        for (const text of pluginAuditCase.visibleTexts) {
          expect(bodyText).toContain(text);
        }

        const rawKeys = [...new Set(bodyText.match(untranslatedKeyPattern) || [])];
        expect(
          rawKeys,
          `${pluginAuditCase.path} still shows raw i18n keys: ${rawKeys.join(', ')}`,
        ).toEqual([]);
      });
    }
  });

  test('TC-3b: 运行时语言包接口返回层级化业务 i18n 资源', async ({
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

    expect(messages?.config?.sys?.auth?.pageTitle?.name).toBe(
      'Login - Page Title',
    );
    expect(messages?.plugin?.['org-core']?.dept?.fields?.name).toBe(
      'Dept Name',
    );
  });
});

