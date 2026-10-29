import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { PaymentPagedTablesPage } from '../pages/PaymentPagedTablesPage';
import { capturePaymentScreenshot } from '../support/screenshot';

test.describe('TC-6 支付声明字段与通道提交一致', () => {
  test('TC-6a: 小写声明的易支付地址、商户号与密钥准确提交，失败保留输入', async ({ adminPage }) => {
    const writes: Record<string, unknown>[] = [];
    const channels: Record<string, unknown>[] = [];
    const candidate = {
      providerCode: 'epay', displayName: '易支付', endpointOrigin: 'https://default.example.test',
      currencies: ['CNY'], modes: ['epay_cashier'], pluginId: 'idc-epay', releaseVersion: 'test',
      interfaceConfiguration: { version: '1', fields: [
        {key: 'endpointorigin', type: 'string', required: true, maximumLength: 2048, label: {zhCN: '接口地址', enUS: 'Endpoint'}},
        {key: 'merchantid', type: 'string', required: true, maximumLength: 128, label: {zhCN: '商户号', enUS: 'Merchant'}},
        {key: 'secret', type: 'secret', required: true, maximumLength: 256, label: {zhCN: '商户密钥', enUS: 'Secret'}},
      ]},
    };
    await adminPage.route('**/x/idc-payment/api/v1/admin/payment-channels*', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({json: {code: 0, data: {channels, providerCandidates: [candidate], total: channels.length}}}); return;
      }
      writes.push(route.request().postDataJSON());
      if (writes.length === 1) {
        await route.fulfill({status: 503, json: {code: 50, message: '测试通道暂时不可保存'}}); return;
      }
      const channel = {channelNo: 'PAYCH_FORM_TEST', displayName: '易支付', providerCode: 'epay', endpointOrigin: 'https://epay.example.test', merchantReference: '1108', modes: ['epay_cashier'], currencies: ['CNY'], statusCode: 'disabled', verifiedAt: 0, version: 1};
      channels.push(channel);
      await route.fulfill({json: {code: 0, data: {channel}}});
    });
    await new PaymentPagedTablesPage(adminPage).open('channels');
    await adminPage.getByRole('button', {name: '新增支付方式', exact: true}).click();
    const modal = adminPage.getByRole('dialog');
    await expect(modal).toContainText('易支付');
    const address = modal.getByTestId('payment-channel-endpointOrigin');
    const merchant = modal.getByTestId('payment-channel-merchantId');
    await expect(address).toHaveValue(candidate.endpointOrigin);
    await modal.getByTestId('payment-channel-number').fill('PAYCH_FORM_TEST');
    await address.fill('https://epay.example.test');
    await merchant.fill('1108');
    await modal.getByTestId('payment-channel-secret').fill('isolated-test-secret');
    await modal.getByRole('button', {name: /^确\s*认$/u}).click();
    await expect.poll(() => writes.length).toBe(1);
    expect(writes[0]).toMatchObject({providerCode: 'epay', endpointOrigin: 'https://epay.example.test', merchantId: '1108', merchantReference: '1108', secret: 'isolated-test-secret', enabledModes: ['epay_cashier']});
    await expect(modal).toBeVisible();
    await expect(address).toHaveValue('https://epay.example.test');
    await expect(merchant).toHaveValue('1108');
    await expect(modal.getByTestId('payment-channel-secret')).toHaveValue('isolated-test-secret');
    await expect(modal).not.toContainText('plugin.idc-payment.');
    await capturePaymentScreenshot(adminPage, 'epay-lowercase-fields-submit-failed');
    await modal.getByRole('button', {name: /^确\s*认$/u}).click();
    await expect(modal).toHaveCount(0);
    expect(writes).toHaveLength(2);
    expect(writes[1]).toEqual(writes[0]);
    await expect(new PaymentPagedTablesPage(adminPage).rows('channels')).toContainText('1108');
    await expect(adminPage.getByText('支付方式已保存，请测试连接后再启用', {exact: true})).toBeVisible();
    await capturePaymentScreenshot(adminPage, 'epay-lowercase-fields-created');
  });
});
