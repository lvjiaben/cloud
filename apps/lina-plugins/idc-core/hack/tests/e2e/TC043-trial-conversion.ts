import { readFileSync } from 'node:fs';
import type { Route } from '@playwright/test';
import { test, expect } from '@host-tests/fixtures/auth';
import { mockCustomerAPIs } from '../../../../../lina-vben/apps/web-customer/tests/support/customer-api-mock';
import { TrialConversionPage } from '../pages/TrialConversionPage';
const api = '/x/idc-core/api/v1/account/';
test.use({ baseURL: process.env.E2E_CUSTOMER_BASE_URL?.trim() || 'http://127.0.0.1:6032' });
async function send(route: Route, data: unknown, code = 0) {
  await route.fulfill({ json: { code, data, message: code ? '余额不足，请充值后重试' : 'success' } });
}
test.describe('TC-43 试用转正式', () => {
  test('TC-43a: 正式周期、精确报价、付款失败重试与列表刷新', async ({ page }) => {
    await mockCustomerAPIs(page);
    const flat = JSON.parse(readFileSync(new URL('../../../manifest/i18n/zh-CN/trial-conversions.json', import.meta.url), 'utf8')) as Record<string, string>;
    const messages: Record<string, any> = {};
    for (const [key, value] of Object.entries(flat)) {
      const parts = key.split('.'); let current = messages;
      for (const part of parts.slice(0, -1)) current = current[part] ??= {};
      current[parts.at(-1)!] = value;
    }
    await page.route('**/i18n/runtime/messages**', route => send(route, { messages }));
    await page.route('**/api/v1/plugins/dynamic**', route => send(route, { list: [{ id: 'idc-core', installed: 1, enabled: 1, version: 'v0.1.2', generation: 1, runtimeState: 'normal', type: 'source', customerSurfaces: [{ pluginId: 'idc-core', pluginType: 'source', key: 'trial-conversions', version: 'v1', title: 'plugin.idc-core.trials.title', slot: 'customer.products', routePath: '/account/plugins/idc-core/trial-conversions', entry: 'customer-trial-conversions.vue', status: 'active', bridgeCapabilities: ['trial-conversions.customer.v1'] }] }] }));
    let confirms = 0; let payments = 0; let paid = false;
    const quoteBodies: unknown[] = [];
    await page.route(`**${api}trial-**`, async route => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith('/quotes')) {
        const body = route.request().postDataJSON(); quoteBodies.push(body);
        expect(body.billingRuleNo).toBe('bil_month');
        await send(route, { quote: { quoteNo: 'quote_trial', currencyCode: 'CNY', netAmountMinor: '10000', taxAmountMinor: '600', totalAmountMinor: '10600', expiresAt: Date.now() + 300000 } }); return;
      }
      if (path.endsWith('/confirmations')) {
        confirms++; await send(route, { operation: { operationNo: 'op_trial', invoiceNo: 'inv_trial', statusCode: 'awaiting_financial_settlement' } }); return;
      }
      if (path.endsWith('/balance-payments')) {
        payments++;
        if (payments === 1) { await send(route, null, 500); return; }
        paid = true; await send(route, { operation: { operationNo: 'op_trial', invoiceNo: 'inv_trial', statusCode: 'succeeded' } }); return;
      }
      await send(route, { items: paid ? [] : [{ entitlementNo: 'ent_trial', productNameZhCN: '试用云服务器', productNameEnUS: 'Trial server', currencyCode: 'CNY', statusCode: 'suspended', expiresAt: 1792022400000, periods: [{ billingRuleNo: 'bil_month', periodUnitCode: 'month', periodCount: 1 }] }], total: paid ? 0 : 1 });
    });
    const trial = new TrialConversionPage(page);
    await trial.login(); await trial.goto();
    await expect(page.getByText('试用云服务器', { exact: true })).toBeVisible();
    await expect(page.getByText('已暂停', { exact: true })).toBeVisible();
    await expect(page.locator('body')).not.toContainText('plugin.idc-core.trials.');
    await trial.screenshot('list'); await trial.open();
    await expect(page.getByRole('dialog')).toContainText('正式周期');
    await expect(page.getByRole('dialog')).toContainText('1 个月');
    await trial.screenshot('period'); await trial.quote();
    await expect(page.getByTestId('trial-quote')).toContainText('应付金额：CNY 106.00');
    expect(payments).toBe(0); await trial.screenshot('quote'); await trial.pay();
    await expect(page.getByRole('dialog')).toContainText('操作未完成');
    expect(confirms).toBe(1); await trial.screenshot('failure'); await trial.pay();
    await expect(page.getByRole('dialog')).toBeHidden();
    await expect(page.getByText('试用云服务器', { exact: true })).toHaveCount(0);
    expect(confirms).toBe(1); expect(payments).toBe(2); expect(quoteBodies).toHaveLength(1);
    await trial.screenshot('paid');
  });
});
