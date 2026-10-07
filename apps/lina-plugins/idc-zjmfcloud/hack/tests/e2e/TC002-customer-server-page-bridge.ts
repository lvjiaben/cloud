import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { CloudServerPage } from '../pages/CloudServerPage';

test.describe('TC-2 云服务器真实客户桥接', () => {
  test('TC-2a: 真实云服务器列表与详情使用客户拥有方票据，非法票据和结束协助均拒绝', async ({ authenticatedPage: admin }) => {
    const customerCode = process.env.E2E_CLOUD_CUSTOMER_ID;
    test.skip(!customerCode, '需要显式指定已有演示客户；测试不创建或操作服务器。');
    const live = new CloudServerPage(admin);
    let customer: Awaited<ReturnType<typeof live.openCustomer>> | undefined;
    try {
      customer = await live.openCustomer(customerCode!);
      const frame = await live.openServers(customer);
      await expect(frame.getByTestId('server-error')).toBeHidden();
      await expect(frame.getByTestId('server-refresh')).toHaveText('刷新');
      await expect(frame.locator('[data-testid="server-empty"]:visible, [data-testid="server-content"]:visible')).toHaveCount(1);
      await expect(customer.locator('iframe[data-testid="plugin-sandbox-frame"]')).not.toHaveAttribute('sandbox', /allow-same-origin/u);
      const listPath = 'account/plugin-servers';
      expect((await live.grant(customer, listPath, 'anonymous')).code).not.toBe(0);
      expect((await live.grant(customer, listPath, 'invalid')).code).not.toBe(0);
      expect(await live.grant(customer, listPath)).toEqual({ status: 200, code: 0, owner: 'idc-resource' });
      expect((await live.grant(customer, 'customer-servers')).code).not.toBe(0);
      expect((await live.grant(customer, 'account/servers')).code).not.toBe(0);
      expect((await live.read(customer, listPath, undefined, 'missing')).code).not.toBe(0);
      expect((await live.read(customer, listPath, undefined, 'tampered')).code).not.toBe(0);
      const listing = await live.read(customer, listPath);
      expect(listing.code).toBe(0);
      expect(Array.isArray(listing.data?.items)).toBe(true);
      const direct = await live.read(customer, 'account/servers', undefined, 'missing');
      expect(direct.code).toBe(0);
      expect(listing.data).toEqual(direct.data);
      const items = listing.data.items as Array<{ serverNo: string; productNameZhCn: string }>;
      if (items.length > 0) {
        const first = items[0]!;
        const detailPath = `${listPath}/${first.serverNo}`;
        await frame.locator(`[data-server-no="${first.serverNo}"]`).click();
        await expect(frame.getByTestId('server-detail-error')).toBeHidden();
        await expect(frame.getByTestId('server-detail-grid')).toContainText(first.productNameZhCn);
        expect((await live.read(customer, detailPath, listPath)).code).not.toBe(0);
        expect((await live.grant(customer, detailPath)).owner).toBe('idc-resource');
        const detail = await live.read(customer, detailPath);
        expect(detail.code).toBe(0);
        expect(detail.data.item.serverNo).toBe(first.serverNo);
        const injected = await live.read(customer, `${detailPath}?serverNo=SVR-${'F'.repeat(32)}`, detailPath);
        expect(injected.code).toBe(0);
        expect(injected.data.item.serverNo).toBe(first.serverNo);
      }
      const directory = resolve(process.cwd(), '../../temp', new Date().toISOString().slice(0, 10).replaceAll('-', ''));
      await mkdir(directory, { recursive: true });
      await customer.screenshot({ path: resolve(directory, `${new Date().toISOString().slice(11, 19).replaceAll(':', '')}-cloud-real-customer-page.png`) });
      await live.endAssistance();
      expect((await live.grant(customer, listPath)).code).not.toBe(0);
      expect((await live.read(customer, listPath)).code).not.toBe(0);
    } finally { await live.endAssistance(); await customer?.close(); }
  });
});
