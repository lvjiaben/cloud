import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { IsolatedMountPage } from '../../../../vben-adapter/hack/tests/pages/IsolatedMountPage';
import { PaymentPagedTablesPage } from '../pages/PaymentPagedTablesPage';
import { installPaymentPagesMock } from '../support/payment-pages-mock';

test.describe('TC-4 支付目录真实总数与完整 Vben 分页', () => {
  test('TC-4a: 渠道配置末页、切换条数、失败重试和权限收回', async ({ adminPage }) => {
    const state = await installPaymentPagesMock(adminPage);
    const view = new PaymentPagedTablesPage(adminPage);
    await view.open('channels');
    await expect(view.rows('channels')).toHaveCount(20);
    await expect(view.total('channels')).toContainText('121');
    await view.last('channels');
    await expect(view.rows('channels')).toHaveCount(1);
    await expect(view.active('channels')).toHaveText('7');
    await view.size('channels', 100);
    await expect(view.rows('channels')).toHaveCount(100);
    await expect(view.active('channels')).toHaveText('1');
    state.failures.channels = 503;
    await view.next('channels');
    await expect.poll(() => state.reads.at(-1)?.page).toBe(2);
    await expect(view.active('channels')).toHaveText('1');
    await view.refresh('channels');
    await expect(view.rows('channels')).toHaveCount(21);
    await expect(view.active('channels')).toHaveText('2');
    await view.screenshot('channels', 'payment-channels-pager');
    state.failures.channels = 403;
    await view.first('channels');
    await expect(view.rows('channels')).toHaveCount(0);
    await expect(view.total('channels')).toContainText('0');
    expect(state.writes).toBe(0);
  });

  test('TC-4b: 健康、批次和超过500条差异各自分页，渠道搜索重置范围', async ({ adminPage }) => {
    const state = await installPaymentPagesMock(adminPage);
    const view = new PaymentPagedTablesPage(adminPage);
    await view.open('health');
    await expect(view.total('health')).toContainText('121');
    await expect(view.total('batches')).toContainText('122');
    await expect(view.total('differences')).toContainText('521');
    await view.last('health');
    await expect(view.rows('health')).toHaveCount(1);
    await view.last('differences');
    await expect(view.rows('differences')).toHaveCount(1);
    await expect(view.rows('differences')).toContainText('TRADE-521');
    await view.screenshot('differences', 'payment-differences-last-page');
    await view.selectChannel('Payment channel 2');
    await expect(view.total('batches')).toContainText('1');
    await expect(view.active('batches')).toHaveText('1');
    expect(state.reads.some(item => item.kind === 'batches' && item.channelNo === 'channel-2' && item.page === 1)).toBe(true);
    expect(state.writes).toBe(0);
  });

  test('TC-4c: 待归属资金分页失败保留内容，权限收回移除动作', async ({ adminPage }) => {
    const state = await installPaymentPagesMock(adminPage);
    const view = new PaymentPagedTablesPage(adminPage);
    await view.open('funds');
    await expect(view.rows('funds')).toHaveCount(20);
    await view.last('funds');
    await expect(view.rows('funds')).toHaveCount(1);
    await expect(view.rows('funds')).toContainText('ORDER-121');
    await view.screenshot('funds', 'payment-funds-last-page');
    state.failures.funds = 403;
    await view.first('funds');
    await expect(view.rows('funds')).toHaveCount(0);
    expect(state.writes).toBe(0);
  });

  test('TC-4d: 发布的独立支付入口使用同一数字分页契约', async ({ adminPage }) => {
    const state = await installPaymentPagesMock(adminPage);
    const surface = new IsolatedMountPage(adminPage);
    try {
      await surface.mount('/x-assets/idc-payment/v0.1.8/mount.js', '/idc/payment-reconciliation');
      const view = new PaymentPagedTablesPage(adminPage, surface.root);
      await expect(view.total('health')).toContainText('121');
      await expect(view.total('batches')).toContainText('122');
      await expect(view.total('differences')).toContainText('521');
      await expect(view.rows('health').first()).toContainText('epay');
      await expect(view.rows('health').first()).toContainText('结果不确定');
      await expect(view.rows('batches').first()).toContainText('结果不确定');
      await expect(view.rows('batches').first().getByRole('button', { name: '差异明细', exact: true })).toHaveAttribute('aria-pressed', 'true');
      await view.last('differences');
      await expect(view.rows('differences')).toHaveCount(1);
      await expect(view.rows('differences')).toContainText('TRADE-521');
      for (const directory of ['health', 'batches', 'differences'] as const) {
        const surfaces = await view.surfaces(directory);
        expect(surfaces.headers.length).toBeGreaterThan(0);
        expect([...new Set(surfaces.headers)]).toEqual([surfaces.header]);
        expect(surfaces.panels.length).toBeGreaterThan(0);
        expect([...new Set(surfaces.panels)]).toEqual([surfaces.panel]);
        await test.info().attach(`${directory}-surface-colors`, { body: JSON.stringify(surfaces, null, 2), contentType: 'application/json' });
      }
      await surface.screenshot('payment-standalone-numbered-pages');
      expect(state.writes).toBe(0);
    } finally { await surface.dispose(); }
  });
  test('TC-4e: 对账页签和批次差异入口保持同一数据范围，分页无需滚动整页', async ({ adminPage, mainLayout }) => {
    await mainLayout.switchLanguage('简体中文');
    const state = await installPaymentPagesMock(adminPage);
    const view = new PaymentPagedTablesPage(adminPage);
    await view.open('batches');
    await expect(view.rows('batches')).toHaveCount(20);
    await view.viewDifferences();
    await expect(adminPage.getByRole('tab', { name: '差异明细', exact: true })).toHaveAttribute('aria-selected', 'true');
    await expect(view.total('differences')).toContainText('521');
    const box = await view.pager('differences').boundingBox();
    expect(box!.y + box!.height).toBeLessThanOrEqual(adminPage.viewportSize()!.height);
    await view.screenshot('differences', 'payment-differences-tab-aligned');
    await mainLayout.switchLanguage('English');
    await expect(adminPage.getByTestId('payment-reconciliation-tabs')).toContainText('Daily batches');
    await adminPage.setViewportSize({ width: 768, height: 1000 });
    await view.selectDirectory('batches');
    const narrowBox = await view.pager('batches').boundingBox();
    expect(narrowBox!.y + narrowBox!.height).toBeLessThanOrEqual(1000);
    await expect.poll(() => view.pager('batches').evaluate(pager => {
      const bounds = pager.getBoundingClientRect();
      return [...pager.querySelector('.vxe-pager--wrapper')!.children].every(item => {
        const rect = item.getBoundingClientRect();
        return rect.bottom <= bounds.bottom + 1 && rect.right <= bounds.right + 1;
      });
    })).toBe(true);
    await view.screenshot('batches', 'payment-batches-english-narrow-aligned');
    expect(state.writes).toBe(0);
  });

});
