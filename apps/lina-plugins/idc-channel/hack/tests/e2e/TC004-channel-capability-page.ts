import { readFileSync } from 'node:fs';
import { extname, resolve } from 'node:path';
import type { Route } from '@playwright/test';
import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { serveChannelVbenRuntime } from '../support/channel-standalone';
import { ChannelVbenPage } from '../pages/ChannelVbenPage';

const messages = JSON.parse(
  readFileSync(new URL('../../../manifest/i18n/zh-CN/plugin.json', import.meta.url), 'utf8'),
) as Record<string, string>;
const assetRoot = resolve(process.cwd(), '../../apps/lina-plugins/idc-channel/frontend/pages');

function nestedMessages(catalog: Record<string, string>) {
  const root: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(catalog)) {
    const parts = key.split('.');
    let cursor = root;
    for (const part of parts.slice(0, -1)) {
      if (!cursor[part] || typeof cursor[part] !== 'object') cursor[part] = {};
      cursor = cursor[part] as Record<string, unknown>;
    }
    cursor[parts.at(-1)!] = value;
  }
  return root;
}

test.describe('TC004 downstream channel operations page', () => {
  test('TC004-1: admin can inspect channel, publication, finance, health, and verified capabilities', async ({ adminPage }) => {
    await serveChannelVbenRuntime(adminPage);
    const envelope = async (route: Route, data: unknown) => route.fulfill({ json: { code: 0, data, message: 'success' } });
    await adminPage.route('**/api/v1/user/info', async (route) => {
      const response = await route.fetch();
      const payload = await response.json();
      payload.data ??= {};
      payload.data.permissions = ['idc:admin:channel:token'];
      await route.fulfill({ json: payload, response });
    });
    await adminPage.route('**/api/v1/i18n/runtime/locales**', (route) => envelope(route, { enabled: true, locale: 'zh-CN', items: [{ locale: 'zh-CN', nativeName: '简体中文', name: 'Simplified Chinese', direction: 'ltr', isDefault: true }] }));
    await adminPage.route('**/api/v1/i18n/runtime/messages**', (route) => envelope(route, { messages: nestedMessages(messages) }));
    await adminPage.route('**/api/v1/plugins/page-bridge/grant', (route) => envelope(route, { grant: 'channel-page-grant', ownerPluginId: 'idc-channel', expiresAt: Date.now() + 120_000 }));
    await adminPage.route('**/api/v1/menus/all', (route) => envelope(route, { list: [{ component: 'BasicLayout', name: 'IDC', path: '/idc', children: [{ component: 'PluginSandboxView', name: 'IDCChannel', path: '/idc/channel', meta: { title: 'plugin.idc-channel.title', pluginSandboxSrc: '/x-assets/idc-channel/v0.1.0/index.html', pluginSurfaceCapabilities: ['channel.admin.v1'], pluginSurfaceHost: 'admin', pluginSurfaceId: 'idc-channel', pluginSurfaceKey: 'channel', pluginSurfaceVersion: 'v1' } }] }] }));
    await adminPage.route('**/x-assets/idc-channel/v0.1.0/**', async (route) => {
      const suffix = new URL(route.request().url()).pathname.split('/v0.1.0/')[1] ?? '';
      if (!/^[\w./-]+$/.test(suffix) || suffix.includes('..')) { await route.abort(); return; }
      const path = resolve(assetRoot, suffix);
      const contentType = extname(path) === '.html' ? 'text/html; charset=utf-8' : 'text/javascript; charset=utf-8';
      await route.fulfill({ body: readFileSync(path), contentType, status: 200 });
    });
    await adminPage.route('**/x/idc-channel/api/v1/admin/channel/accounts', (route) => envelope(route, { total: 1, page: 1, pageSize: 10, list: [{ channelId: 'channel_primary', customerNo: 'CUS_CHANNEL', displayName: '主渠道', statusCode: 'approved', newSalesEnabled: true, updatedAt: Date.parse('2026-08-31T08:00:00Z') }] }));
    await adminPage.route('**/x/idc-channel/api/v1/admin/channel/capabilities', (route) => envelope(route, { genericApi: true, adapters: { mofang_finance: 'disabled_unverified' }, operations: { catalog: 'enabled', create_order: 'enabled', reconfigure: 'enabled_core_guarded', change_ip: 'enabled_core_guarded' } }));
    await adminPage.route('**/x/idc-channel/api/v1/admin/channel/publications**', (route) => envelope(route, { total: 1, page: 1, pageSize: 10, list: [{ productVersionId: 101, currency: 'CNY', priceMinor: 8800, stock: 4, status: 'published', publicSlug: 'server-basic' }] }));
    await adminPage.route('**/x/idc-channel/api/v1/admin/channel/finance**', (route) => envelope(route, { invoiceTotal: 0, statementTotal: 0, repaymentTotal: 0, currencies: [{ currencyCode: 'CNY', availableBalanceMinor: 120000, creditAvailableMinor: 500000, receivableMinor: 8800, paidMinor: 17600, refundedMinor: 0, serviceCostMinor: 9000 }], invoices: [], creditStatements: [], creditRepayments: [] }));
    await adminPage.route('**/x/idc-channel/api/v1/admin/channel/health**', (route) => envelope(route, { statusCode: 'approved', newSalesEnabled: true, activeTokenCount: 2, publicationCount: 1, pendingWebhookCount: 0, lastRequestAt: Date.parse('2026-08-31T08:00:00Z') }));
    await adminPage.route('**/x/idc-channel/api/v1/admin/channel/requests**', (route) => envelope(route, { total: 1, page: 1, pageSize: 10, list: [{ requestId: 'req-1', operation: 'quote', status: 'accepted', resultCode: 'ok', createdAt: Date.parse('2026-08-31T08:00:00Z'), completedAt: Date.parse('2026-08-31T08:00:01Z') }] }));

    await adminPage.goto('/admin/idc/channel', { waitUntil: 'domcontentloaded' });
    const root = adminPage.frameLocator('[data-testid="plugin-sandbox-frame"]').getByTestId('idc-channel-admin-page');
    await expect(root).toBeVisible();
    await expect(root).toContainText('主渠道');
    await root.getByTestId('channel-selector').selectOption('channel_primary');
    await root.getByRole('tab', { name: '商品发布' }).click();
    await expect(root).toContainText('CNY 88.00');
    await root.getByTestId('add-publication-value').click();
    await expect(root.locator('[name="fieldToken"]')).toBeVisible();
    await expect(root).not.toContainText('配置值 JSON 数组');
    await root.getByRole('tab', { name: '额度与结算' }).click();
    await expect(root).toContainText('CNY 1200.00');
    await root.getByRole('tab', { name: '健康与审计' }).click();
    await expect(root).toContainText('req-1');
    await root.getByRole('tab', { name: '能力清单' }).click();
    await expect(root).toContainText('enabled_core_guarded');
    await expect(root).toContainText('disabled_unverified');
    await new ChannelVbenPage(adminPage).snapshot('channel-operations-page');
  });
});
