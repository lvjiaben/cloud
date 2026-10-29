import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { PaymentPagedTablesPage } from '../pages/PaymentPagedTablesPage';
import { capturePaymentScreenshot } from '../support/screenshot';

test.describe('TC-5 支付通知与浏览器返回地址独立维护', () => {
  test('TC-5a: 保存即时生效，失败保留输入并使用原版本重试', async ({ adminPage }) => {
    let saved = {notificationOrigin: 'https://old-notify.example.com', customerReturnOrigin: 'http://127.0.0.1:6032', version: 7};
    const writes: unknown[] = [];
    let fail = true;
    await adminPage.route('**/x/idc-payment/api/v1/admin/payment-channels?*', r => r.fulfill({json: {code: 0, data: {channels: [], providerCandidates: [], total: 0}}}));
    await adminPage.route('**/x/idc-payment/api/v1/admin/payment-public-addresses', async route => {
      if (route.request().method() === 'GET') { await route.fulfill({json: {code: 0, data: {addresses: saved}}}); return; }
      const input = route.request().postDataJSON(); writes.push(input);
      if (fail) { fail = false; await route.fulfill({status: 503, json: {code: 50, message: '测试保存暂时失败'}}); return; }
      expect(input.expectedVersion).toBe(7);
      saved = {notificationOrigin: input.notificationOrigin, customerReturnOrigin: input.customerReturnOrigin, version: 8};
      await route.fulfill({json: {code: 0, data: {addresses: saved}}});
    });
    await new PaymentPagedTablesPage(adminPage).open('channels');
    await adminPage.getByTestId('payment-public-addresses-open').click();
    const modal = adminPage.getByRole('dialog');
    await expect(modal).toContainText('通知入口可使用独立公网网关，无需与系统地址一致。保存后立即生效。');
    const notification = modal.getByPlaceholder('https://notify.example.com');
    const customer = modal.getByPlaceholder('https://customer.example.com');
    await expect(notification).toHaveValue(saved.notificationOrigin);
    await notification.fill('https://gateway.example.com');
    await customer.fill('http://localhost:6032');
    await capturePaymentScreenshot(adminPage, 'independent-public-address-edit');
    await modal.getByRole('button', {name: /^保\s*存$/u}).click();
    await expect.poll(() => writes.length).toBe(1);
    await expect(notification).toHaveValue('https://gateway.example.com');
    await expect(modal).toBeVisible();
    await capturePaymentScreenshot(adminPage, 'independent-public-address-failed');
    await modal.getByRole('button', {name: /^保\s*存$/u}).click();
    await expect(modal).toHaveCount(0);
    expect(writes).toEqual([{notificationOrigin: 'https://gateway.example.com', customerReturnOrigin: 'http://localhost:6032', expectedVersion: 7}, {notificationOrigin: 'https://gateway.example.com', customerReturnOrigin: 'http://localhost:6032', expectedVersion: 7}]);
    await adminPage.getByTestId('payment-public-addresses-open').click();
    await expect(notification).toHaveValue('https://gateway.example.com');
    await expect(customer).toHaveValue('http://localhost:6032');
    await expect(modal).not.toContainText('publicAddresses.');
    await capturePaymentScreenshot(adminPage, 'independent-public-address-saved');
  });

  test('TC-5b: 地址读取被拒绝后禁止用空值覆盖配置', async ({ adminPage }) => {
    let writes = 0;
    await adminPage.route('**/x/idc-payment/api/v1/admin/payment-channels?*', r => r.fulfill({json: {code: 0, data: {channels: [], providerCandidates: [], total: 0}}}));
    await adminPage.route('**/x/idc-payment/api/v1/admin/payment-public-addresses', async route => {
      if (route.request().method() !== 'GET') writes++;
      await route.fulfill({status: 403, json: {code: 403, message: 'Denied for fixture'}});
    });
    await new PaymentPagedTablesPage(adminPage).open('channels');
    await adminPage.getByTestId('payment-public-addresses-open').click();
    const modal = adminPage.getByRole('dialog');
    await expect(modal).toContainText('加载失败，请稍后重试');
    await expect(modal.getByRole('button', {name: /^保\s*存$/u})).toBeDisabled();
    expect(writes).toBe(0);
    await capturePaymentScreenshot(adminPage, 'independent-public-address-denied');
  });
});
