import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { PaymentPagedTablesPage } from '../pages/PaymentPagedTablesPage';
import { capturePaymentScreenshot } from '../support/screenshot';

test.describe('TC-7 已配置通道修改支付场景', () => {
  test('TC-7a: 无须密钥即可切换支付宝与微信，失败保留选择和原版本', async ({adminPage}) => {
    let channel = {channelNo: 'EPAY_MODES_TEST', providerCode: 'epay', displayName: '易支付验收', endpointOrigin: 'https://epay.example/submit.php', merchantReference: '1108', modes: ['epay_cashier'], currencies: ['CNY'], statusCode: 'enabled', verifiedAt: 1792460000000, version: 7};
    const writes: unknown[] = [];
    await adminPage.route('**/x/idc-payment/api/v1/admin/payment-channels?*', route => route.fulfill({json: {code: 0, data: {channels: [channel], total: 1, providerCandidates: [{providerCode: 'epay', displayName: '易支付', modes: ['epay_cashier', 'epay_alipay', 'epay_wechat'], currencies: ['CNY']}]}}}));
    await adminPage.route('**/x/idc-payment/api/v1/admin/payment-channels/EPAY_MODES_TEST/payment-modes', async route => {
      const body = route.request().postDataJSON(); writes.push(body);
      if (writes.length === 1) {await route.fulfill({status: 503, json: {code: 50, message: '测试保存暂时失败'}}); return;}
      channel = {...channel, modes: body.enabledModes, version: 8};
      await route.fulfill({json: {code: 0, data: {channel}}});
    });
    const view = new PaymentPagedTablesPage(adminPage);
    await view.open('channels');
    await adminPage.getByTestId('payment-channel-modes-EPAY_MODES_TEST').click();
    const modal = adminPage.getByRole('dialog');
    await expect(modal).toContainText('无需重新填写商户密钥');
    await expect(modal.locator('input[type="password"]')).toHaveCount(0);
    const select = modal.getByTestId('payment-channel-enabled-modes');
    await select.locator('.ant-select-selection-item-remove').click();
    await modal.getByRole('button', {name: /^保\s*存$/u}).click();
    await expect(modal).toContainText('请选择至少一种支付方式');
    expect(writes).toHaveLength(0);
    await select.click();
    await adminPage.locator('.ant-select-dropdown:visible').getByText('易支付 · 支付宝', {exact: true}).click();
    await adminPage.locator('.ant-select-dropdown:visible').getByText('易支付 · 微信支付', {exact: true}).click();
    await modal.getByRole('heading').click();
    await modal.getByRole('button', {name: /^保\s*存$/u}).click();
    await expect.poll(() => writes.length).toBe(1);
    await expect(select).toContainText('易支付 · 支付宝');
    await expect(select).toContainText('易支付 · 微信支付');
    await capturePaymentScreenshot(adminPage, 'payment-modes-save-failed');
    await modal.getByRole('button', {name: /^保\s*存$/u}).click();
    await expect(modal).toHaveCount(0);
    expect(writes).toEqual([{enabledModes: ['epay_alipay', 'epay_wechat'], expectedVersion: 7}, {enabledModes: ['epay_alipay', 'epay_wechat'], expectedVersion: 7}]);
    await expect(view.rows('channels')).toContainText('易支付 · 支付宝');
    await expect(view.rows('channels')).toContainText('易支付 · 微信支付');
    await expect(adminPage.getByText('支付场景已保存', {exact: true})).toBeVisible();
    await view.screenshot('channels', 'payment-modes-saved');
  });
});
