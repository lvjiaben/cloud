import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { IsolatedMountPage } from '../../../../vben-adapter/hack/tests/pages/IsolatedMountPage';
import { PaymentPagedTablesPage } from '../pages/PaymentPagedTablesPage';

test.describe('TC-3 独立支付页面使用官方 Vben 表格', () => {
  test('TC-3a: 差异明细白色面板、翻译与补单请求保持不变', async ({ adminPage }) => {
    let submitted = false;
    await adminPage.route('**/x/idc-payment/api/v1/**', async route => {
      const path = new URL(route.request().url()).pathname;
      if (route.request().method() === 'POST') {
        expect(path).toContain('/differences/LINE1/supplements');
        expect(route.request().postDataJSON()).toEqual({ expectedVersion: 2 });
        submitted = true;
      }
      const data = path.endsWith('/channels') ? { channels: [], total: 0 }
        : path.endsWith('/batches') ? { total: 1, batches: [{ batchNo: 'BATCH1', channelNo: 'DEMO', statusCode: 'differences', businessDate: 1788470400000, paymentCount: 1, refundCount: 0, differenceCount: 1 }] }
        : path.endsWith('/differences') ? { total: 1, differences: [{ lineNo: 'LINE1', externalReference: 'TX-DEMO', differenceCode: 'local_missing', channelStatusCode: 'succeeded', localStatusCode: 'pending', localAmountMinor: '0', channelAmountMinor: '1250', currencyCode: 'CNY', version: 2, supplementRequestNo: submitted ? 'SUP1' : '', supplementStatusCode: submitted ? 'pending_review' : '' }] } : {};
      await route.fulfill({ json: { code: 0, data } });
    });
    const surface = new IsolatedMountPage(adminPage);
    try {
      await surface.mount('/x-assets/idc-payment/v0.1.8/mount.js', '/idc/payment-reconciliation');
      const view = new PaymentPagedTablesPage(adminPage, surface.root);
      const differences = view.rows('differences');
      await expect(differences).toHaveCount(1);
      await expect(view.headers('differences')).toContainText('渠道交易号');
      await expect(differences).toContainText('CNY 12.50');
      expect(await surface.whiteSurfaces()).toEqual(expect.arrayContaining(['rgb(255, 255, 255)']));
      expect((await surface.whiteSurfaces()).every(color => color === 'rgb(255, 255, 255)')).toBe(true);
      await surface.screenshot('payment-vben-differences');
      await surface.click('提交补入账');
      await expect.poll(() => submitted).toBe(true);
      await expect(differences).toContainText('等待另一名管理员复核');
      await surface.setLocale('en-US');
      await expect(view.headers('differences')).toContainText('Provider transaction');
      await expect(surface.root.getByRole('option', { name: 'Insufficient evidence', exact: true }).first()).toHaveCount(1);
      await surface.screenshot('payment-vben-review-en');
    } finally { await surface.dispose(); }
  });

  test('TC-3b: 待归属资金保留输入与失败提示，不提交空订单号', async ({ adminPage }) => {
    const requests: unknown[] = [];
    await adminPage.route('**/x/idc-payment/api/v1/**', async route => {
      if (route.request().method() === 'POST') {
        requests.push(route.request().postDataJSON());
        await route.fulfill({ status: 403, json: { code: 403, message: 'Denied for fixture' } });
        return;
      }
      await route.fulfill({ json: { code: 0, data: { total: 1, funds: [{ exceptionNo: 'FUND1', orderNo: 'ORDER-CANCELLED', attemptNo: 'ATTEMPT1', providerCode: 'Demo', channelNo: 'CHANNEL1', currencyCode: 'CNY', amountMinor: '1250', statusCode: 'pending', actionCode: '', version: 1 }] } } });
    });
    const surface = new IsolatedMountPage(adminPage);
    try {
      await surface.mount('/x-assets/idc-payment/v0.1.8/mount.js', '/idc/payment-exception-funds');
      await expect(surface.rows).toHaveCount(1);
      await expect(surface.rows).toContainText('ORDER-CANCELLED');
      await expect(surface.rows).toContainText('处理中');
      await surface.click('校验并分配');
      await expect(surface.root.getByRole('alert')).toContainText('请输入新订单号');
      expect(requests).toHaveLength(0);
      await surface.root.getByPlaceholder('输入同一客户的未付订单号').last().fill('ORDER-NEW');
      await surface.click('校验并分配');
      await expect.poll(() => requests.length).toBe(1);
      expect(requests[0]).toEqual({ targetOrderNo: 'ORDER-NEW' });
      await expect(surface.root.getByRole('alert')).toContainText('分配失败');
      await expect(surface.root.getByPlaceholder('输入同一客户的未付订单号').last()).toHaveValue('ORDER-NEW');
      expect((await surface.whiteSurfaces()).every(color => color === 'rgb(255, 255, 255)')).toBe(true);
      await surface.screenshot('payment-vben-allocation-failure');
    } finally { await surface.dispose(); }
  });
});
