import type { Route } from '@playwright/test';
import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { mockCustomerAPIs } from '../../../../../lina-vben/apps/web-customer/tests/support/customer-api-mock';
import { ChannelCustomerTokenPage } from '../pages/ChannelCustomerTokenPage';

const channelTokenPath = '/x/idc-channel/api/v1/account/channel/tokens';

test.use({
  baseURL:
    process.env.E2E_CUSTOMER_BASE_URL?.trim() || 'http://127.0.0.1:6032',
});

type MockToken = {
  createdAt: number;
  expiresAt?: number;
  id: number;
  revokedAt?: number;
  scopes: string[];
  sourceCidrs: string[];
};

async function fulfillEnvelope(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    contentType: 'application/json',
    status,
    body: JSON.stringify({
      code: status >= 400 ? status : 0,
      data,
      message: status >= 400 ? 'channel account is not eligible' : 'success',
    }),
  });
}

async function mockChannelCustomerSurface(route: Route) {
  await fulfillEnvelope(route, {
    list: [
      {
        id: 'idc-channel',
        installed: 1,
        enabled: 1,
        version: 'v0.1.0',
        generation: 1,
        statusKey: 'sys_plugin.status:idc-channel',
        runtimeState: 'normal',
        type: 'source',
        customerSurfaces: [
          {
            pluginId: 'idc-channel',
            pluginType: 'source',
            key: 'channel-tokens',
            version: 'v1',
            title: '渠道访问令牌',
            icon: 'lucide:key-round',
            slot: 'customer.products',
            routePath: 'channel-tokens',
            entry: 'customer/channel-tokens/index.html',
            status: 'active',
            bridgeCapabilities: ['channel.account.v1'],
          },
        ],
      },
    ],
  });
}

test.describe('TC006 customer channel token page', () => {
  test('TC006-1: approved enterprise channel can create, inspect, and revoke its own token', async ({ page }) => {
    const tokens: MockToken[] = [
      {
        id: 42,
        scopes: ['catalog.read', 'order.create'],
        sourceCidrs: ['192.0.2.0/24'],
        createdAt: Date.parse('2026-08-31T08:00:00Z'),
        expiresAt: Date.parse('2026-12-31T16:00:00Z'),
      },
    ];

    await mockCustomerAPIs(page);
    await page.route('**/api/v1/plugins/dynamic**', mockChannelCustomerSurface);
    await page.route(`**${channelTokenPath}**`, async (route) => {
      const url = new URL(route.request().url());
      const method = route.request().method();
      if (method === 'DELETE') {
        const id = Number(url.pathname.split('/').at(-1));
        const index = tokens.findIndex((item) => item.id === id);
        expect(index).toBeGreaterThanOrEqual(0);
        tokens[index] = {
          ...tokens[index]!,
          revokedAt: Date.parse('2026-08-31T09:00:00Z'),
        };
        await fulfillEnvelope(route, { id, revoked: true });
        return;
      }
      if (method === 'POST' && url.pathname.endsWith('/tokens')) {
        const payload = route.request().postDataJSON();
        expect(payload.scopes).toEqual([
          'catalog.read',
          'order.create',
          'order.read',
          'service.read',
          'settlement.read',
        ]);
        tokens.push({
          id: 77,
          scopes: payload.scopes,
          sourceCidrs: payload.sourceCidrs,
          createdAt: Date.parse('2026-08-31T09:01:00Z'),
        });
        await fulfillEnvelope(route, { token: 'channel-token-shown-once' });
        return;
      }
      if (method === 'GET' && url.pathname.endsWith('/tokens')) {
        await fulfillEnvelope(route, { list: tokens });
        return;
      }
      await route.abort();
    });

    const tokenPage = new ChannelCustomerTokenPage(page);
    await tokenPage.login();
    await tokenPage.goto();

    await expect(tokenPage.root).toBeVisible();
    await expect(tokenPage.root).toContainText('渠道访问令牌');
    await expect(tokenPage.root).toContainText('查看可售商品');
    await expect(tokenPage.root).not.toContainText('catalog.read');
    await expect(tokenPage.tokenItems).toHaveCount(1);
    await tokenPage.screenshot('channel-customer-tokens-loaded');
    await tokenPage.createToken();
    await expect(tokenPage.oneTimeToken).toContainText(
      'channel-token-shown-once',
    );
    await expect(tokenPage.tokenItems).toHaveCount(2);
    await tokenPage.screenshot('channel-customer-token-created');
    await tokenPage.revokeFirstToken();
    await expect(tokenPage.root).toContainText('已撤销');
    await tokenPage.screenshot('channel-customer-token-revoked');
  });

  test('TC006-2: unapproved account is denied without exposing token controls', async ({ page }) => {
    await mockCustomerAPIs(page);
    await page.route('**/api/v1/plugins/dynamic**', mockChannelCustomerSurface);
    await page.route(`**${channelTokenPath}**`, async (route) => {
      await fulfillEnvelope(route, null, 403);
    });

    const tokenPage = new ChannelCustomerTokenPage(page);
    await tokenPage.login();
    await tokenPage.goto();

    await expect(tokenPage.root).toBeVisible();
    await expect(tokenPage.root).toContainText('此页面只对获批企业渠道开放');
    await expect(tokenPage.createButton).toHaveCount(0);
    await tokenPage.screenshot('channel-customer-token-denied');
  });
});
