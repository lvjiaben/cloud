import type { Page } from '@playwright/test';

export async function installCustomerAssistanceMock(
  page: Page,
  options: { unavailable?: boolean } = {},
) {
  const requests: Array<{ body: Record<string, unknown>; url: string }> = [];
  await page.route('**/public/customer-assistance/access-verifications', async (route) => {
    const request = route.request();
    requests.push({
      body: (request.postDataJSON() || {}) as Record<string, unknown>,
      url: request.url(),
    });
    if (options.unavailable) {
      await route.fulfill({
        contentType: 'application/json',
        status: 403,
        body: JSON.stringify({ code: 403, message: 'unavailable' }),
      });
      return;
    }
    await route.fulfill({
      contentType: 'application/json',
      status: 200,
      body: JSON.stringify({
        code: 0,
        data: {
          accountStatus: 'active',
          countryCode: 'CN',
          customerNo: 'CUS_0123456789abcdef0123456789abcdef',
          customerType: 'enterprise',
          displayName: '上海云舟科技',
          expiresAt: Date.now() + 15 * 60 * 1000,
          forbiddenActions: [
            'payment',
            'refund',
            'secret',
            'console',
            'reinstall',
            'change_ip',
            'terminate',
            'destructive_service_operation',
          ],
          preferredLocale: 'zh-CN',
          readOnly: true,
          sessionNo: 'AST_0123456789abcdef0123456789abcdef',
          startedAt: Date.now(),
          verification: 'verified',
        },
      }),
    });
  });
  return { requests };
}
