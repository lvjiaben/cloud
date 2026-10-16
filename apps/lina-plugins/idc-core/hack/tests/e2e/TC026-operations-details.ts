import { mkdir } from 'node:fs/promises';
import path from 'node:path';

import type { Page } from '@host-tests/support/playwright';
import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { waitForRouteReady } from '../../../../../../hack/tests/support/ui';
import { installFinancialManagementMock } from '../support/financial-management-mock';
import { installSupportDeskMock, supportTickets } from '../support/support-desk-mock';

type LiveOverview = {
  periods: Array<{ key: string; paidOrders: number; incomeMinor: string }>;
  customers: { total: number; online: number };
  expiringServers: number;
  tickets: { pending: number; processed: number };
};

const startAt = '1790784000000';
const endAt = '1793462400000';
const asOf = '1791619200000';
const endpoint = '**/x/idc-core/api/v1/admin/operations/data/records**';

async function screenshot(page: Page, name: string) {
  await waitForRouteReady(page);
  // Visibility changes before the overlay's CSS fade ends.
  await expect.poll(() => page.locator('.bg-overlay-content').evaluateAll((nodes) => nodes.every((node) => Number(getComputedStyle(node).opacity) === 0))).toBe(true);
  const now = new Date();
  const directory = path.resolve(process.cwd(), '../../temp', now.toISOString().slice(0, 10).replaceAll('-', ''));
  await mkdir(directory, { recursive: true });
  await page.screenshot({ path: path.join(directory, `${now.toISOString().slice(11, 23).replaceAll(':', '').replaceAll('.', '')}-${name}.png`), fullPage: false });
}

function record(id = 'DETAIL_001') {
  return { id, label: '运营统计明细', customerNo: 'CUS_DETAIL_001', customerName: '运营测试客户', currencyCode: 'CNY', amountMinor: '900719925474099301', quantity: 3, statusCode: 'paid', occurredAt: 1790870400000 };
}

test.describe('TC-26 运营数据下钻明细', () => {
  for (const [metric, pathname, title, status] of [
    ['income', '/idc/finance', '实际到账流水', ''],
    ['orders', '/idc/orders', '已付款订单', ''],
    ['products', '/idc/orders', '商品销量订单', ''],
    ['customers', '/idc/customers', '客户明细', 'online'],
    ['servers', '/idc/resource/server-accessories', '即将到期服务器', ''],
    ['tickets', '/idc/support', '工单明细', 'pending'],
  ]) {
    test(`TC-26-${metric}: 同一统计条件进入对应业务明细并采用服务端分页`, async ({ adminPage }) => {
      const requests: URL[] = [];
      await adminPage.route(endpoint, async (route) => {
        const url = new URL(route.request().url());
        requests.push(url);
        const secondPage = url.searchParams.get('page') === '2';
        await route.fulfill({ json: { code: 0, data: { items: [record(secondPage ? 'DETAIL_PAGE_2' : 'DETAIL_PAGE_1')], total: 21 } } });
      });
      const query = new URLSearchParams({ operationsMetric: metric!, startAt, endAt, asOf, currencyCode: 'CNY', ...(status ? { status } : {}), ...(metric === 'products' ? { productNo: 'PRD_DETAIL_001' } : {}) });
      await adminPage.goto(`${pathname}?${query}`);
      const panel = adminPage.getByTestId('operations-detail-panel');
      await expect(panel).toContainText(title!);
      await expect(panel).toContainText('运营测试客户');
      await expect(panel).toContainText('DETAIL_PAGE_1');
      await expect(panel).not.toContainText('plugin.idc-core');
      await expect(panel.getByRole('button', { name: '查看完整业务列表' })).toBeVisible();
      if (['income', 'orders', 'products'].includes(metric!)) await expect(panel).toContainText('CNY 9007199254740993.01');
      if (metric === 'products') await expect(panel).toContainText('销量');
      if (metric === 'servers') await expect(panel).toContainText('未来 7 天内到期');
      const pager = panel.locator('.vxe-pager');
      await expect(pager.locator('.vxe-pager--total')).toContainText('21');
      await expect(pager.locator('.vxe-pager--sizes')).toBeVisible();
      await pager.locator('.vxe-pager--next-btn').click();
      await expect(panel).toContainText('DETAIL_PAGE_2');
      const actual = requests.at(-1)!.searchParams;
      expect(actual.get('page')).toBe('2');
      expect(actual.get('pageSize')).toBe('20');
      expect(actual.get('metric')).toBe(metric);
      expect(actual.get('startAt')).toBe(startAt);
      expect(actual.get('endAt')).toBe(endAt);
      expect(actual.get('asOf')).toBe(asOf);
      expect(actual.get('status')).toBe(status || null);
      expect(actual.get('productNo')).toBe(metric === 'products' ? 'PRD_DETAIL_001' : null);
      if (metric === 'income' || metric === 'servers') await screenshot(adminPage, `operations-details-${metric}`);
    });
  }

  test('TC-26g: 到账明细打开原交易详情，退出筛选恢复完整财务列表', async ({ adminPage }) => {
    await installFinancialManagementMock(adminPage);
    await adminPage.route(endpoint, (route) => route.fulfill({ json: { code: 0, data: { items: [record('BLT_FINANCE_MERCHANT_0001')], total: 1 } } }));
    await adminPage.goto('/idc/finance?operationsMetric=income&currencyCode=CNY');
    const panel = adminPage.getByTestId('operations-detail-panel');
    await expect(panel).toContainText('BLT_FINANCE_MERCHANT_0001');
    await panel.getByText('查看详情', { exact: true }).click();
    const drawer = adminPage.getByRole('dialog');
    await expect(drawer).toContainText('BLT_FINANCE_MERCHANT_0001');
    await expect.poll(async () => (await drawer.boundingBox())?.x ?? Infinity).toBeLessThan(600);
    await screenshot(adminPage, 'operations-details-transaction-drawer');
    await adminPage.keyboard.press('Escape');
    await expect(drawer).not.toBeVisible();
    await panel.getByRole('button', { name: '查看完整业务列表' }).click();
    await expect(panel).not.toBeVisible();
    await expect(adminPage.getByTestId('financial-management-page')).toContainText('INV_FINANCE_0001');
    expect(new URL(adminPage.url()).searchParams.has('operationsMetric')).toBe(false);
  });

  test('TC-26h: 加载失败清空列表且刷新可恢复', async ({ adminPage }) => {
    let fail = true;
    await adminPage.route(endpoint, (route) => route.fulfill(fail
      ? { status: 503, json: { code: 503, message: 'Temporarily unavailable' } }
      : { json: { code: 0, data: { items: [record('DETAIL_RECOVERED')], total: 1 } } }));
    await adminPage.goto('/idc/orders?operationsMetric=orders&currencyCode=CNY');
    const panel = adminPage.getByTestId('operations-detail-panel');
    await expect(panel).toContainText('明细加载失败，请刷新后重试。');
    await expect(panel).not.toContainText('DETAIL_RECOVERED');
    await screenshot(adminPage, 'operations-details-error');
    fail = false;
    await panel.getByRole('button', { name: /刷\s*新/ }).click();
    await expect(panel).toContainText('DETAIL_RECOVERED');
    await expect(panel).not.toContainText('明细加载失败，请刷新后重试。');
  });

  test('TC-26i: 工单下钻可打开不在首屏目录的已授权工单', async ({ adminPage }) => {
    await installSupportDeskMock(adminPage);
    await adminPage.route('**/x/idc-core/api/v1/admin/support/tickets?*', (route) => route.fulfill({ json: { code: 0, data: { tickets: [], pageActions: ['query'], nextCursor: '' } } }));
    await adminPage.route(endpoint, (route) => route.fulfill({ json: { code: 0, data: { items: [record(supportTickets.readonly)], total: 1 } } }));
    await adminPage.goto('/idc/support?operationsMetric=tickets&status=processed');
    const panel = adminPage.getByTestId('operations-detail-panel');
    await expect(panel).toContainText('已解决或已关闭工单');
    await panel.getByText('查看详情', { exact: true }).click();
    await expect(adminPage.getByRole('heading', { name: '只读权限核对工单' })).toBeVisible();
  });

  test('TC-26j: 真实总后台统计卡片授权及下钻总数一致', async ({ adminPage }) => {
    test.setTimeout(120_000);
    const overviewResponse = adminPage.waitForResponse((response) => new URL(response.url()).pathname.endsWith('/admin/operations/data'));
    await adminPage.goto('/idc/operations/workbench');
    let overview = (await (await overviewResponse).json()) as { code: number; data: LiveOverview };
    expect(overview.code).toBe(0);

    for (const target of ['income', 'orders', 'customers', 'online', 'servers', 'pending', 'processed'] as const) {
      const root = adminPage.getByTestId('operations-data');
      await expect(root.getByTestId('period-today')).toBeVisible();
      const recordsResponse = adminPage.waitForResponse((response) => new URL(response.url()).pathname.endsWith('/admin/operations/data/records'));
      if (target === 'income' || target === 'orders') {
        await root.getByTestId('period-today').locator(`.${target === 'income' ? 'income' : 'orders'}-link`).click();
      } else {
        const label = { customers: '累计客户', online: '在线客户', servers: '即将到期服务器', pending: '待处理工单', processed: '已处理工单' }[target]!;
        await root.getByRole('button').filter({ hasText: label }).click();
      }
      const payload = (await (await recordsResponse).json()) as { code: number; data: { total: number; items: Array<{ amountMinor: string }> } };
      expect(payload.code, `${target} records must pass real business permissions`).toBe(0);
      const expected = {
        income: undefined,
        orders: overview.data.periods.find((period) => period.key === 'today')!.paidOrders,
        customers: overview.data.customers.total,
        online: overview.data.customers.online,
        servers: overview.data.expiringServers,
        pending: overview.data.tickets.pending,
        processed: overview.data.tickets.processed,
      }[target];
      if (expected !== undefined) expect(payload.data.total).toBe(expected);
      if (target === 'income' && payload.data.total <= 20) {
        const sum = payload.data.items.reduce((value, item) => value + BigInt(item.amountMinor), 0n);
        expect(sum.toString()).toBe(overview.data.periods.find((period) => period.key === 'today')!.incomeMinor);
      }
      const panel = adminPage.getByTestId('operations-detail-panel');
      await expect(panel).toBeVisible();
      await expect(panel).not.toContainText('明细加载失败');
      await expect(panel.locator('.vxe-pager--total')).toContainText(String(payload.data.total));
      if (target === 'customers') await screenshot(adminPage, 'operations-details-real-customers');
      const nextOverviewResponse = adminPage.waitForResponse((response) => new URL(response.url()).pathname.endsWith('/admin/operations/data'));
      await panel.getByRole('button', { name: '返回运营数据' }).click();
      overview = await (await nextOverviewResponse).json();
      expect(overview.code).toBe(0);
    }
  });
});
