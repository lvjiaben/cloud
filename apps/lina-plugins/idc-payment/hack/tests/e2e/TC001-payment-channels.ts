import { createHash } from 'node:crypto';

import type { Page } from '@playwright/test';

import { expect, test } from '../../../../../../hack/tests/fixtures/auth';

import { PaymentChannelsPage } from '../pages/PaymentChannelsPage';
import { capturePaymentScreenshot } from '../support/screenshot';

const channelNo = 'PAYCH_E2E_RECHARGE_0001';
const backendOrigin = process.env.E2E_BACKEND_BASE_URL || 'http://127.0.0.1:19996';

function sign(values: Record<string, string>) {
  const canonical = Object.entries(values)
    .filter(([key, value]) => value && key !== 'sign' && key !== 'sign_type')
    .toSorted(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join('&');
  return createHash('md5').update(`${canonical}e2e-secret`).digest('hex');
}

async function prepare(page: Page) {
  await page.route('https://payments.example.test/submit.php', async (route) => {
    await route.fulfill({
      body: '<!doctype html><title>Isolated EPay sink</title><p>submitted</p>',
      contentType: 'text/html; charset=utf-8',
      status: 200,
    });
  });
  const payment = new PaymentChannelsPage(page);
  await payment.login();
  return payment;
}

test.describe('TC-1 支付通道与幂等入账', () => {
  test.skip(process.env.E2E_REAL_RECHARGE !== '1', '需要隔离的本地已安装运行时');

  test('TC-1a: 已启用的人民币易支付通道可选', async ({ page }) => {
    const payment = await prepare(page);
    await payment.openRecharge();
    await expect(page.getByRole('dialog')).toContainText('易支付');
    await expect(page.getByRole('dialog')).toContainText('CNY');
    await capturePaymentScreenshot(page, 'payment-channel-epay-cny');
  });

  test('TC-1b: 人民币易支付只创建安全跳转', async ({ page }) => {
    const payment = await prepare(page);
    await payment.openRecharge();
    const { attemptNo } = await payment.createRecharge();
    expect(attemptNo).toMatch(/^PAT_[0-9a-f]{32}$/u);
    await expect(page).toHaveURL('https://payments.example.test/submit.php');
  });

  test('TC-1c: 支付页返回不会直接入账', async ({ page }) => {
    const payment = await prepare(page);
    await payment.openRecharge();
    const { attemptNo } = await payment.createRecharge();
    const result = await payment.openReturn(attemptNo);
    await expect(result).toContainText('正在等待支付平台确认');
    await expect(result).not.toContainText('充值已经到账');
    await capturePaymentScreenshot(page, 'payment-return-confirming');
  });

  test('TC-1d: 重复成功通知只入账一次', async ({ page }) => {
    const payment = await prepare(page);
    const before = await payment.balance();
    await payment.openRecharge();
    const { attemptNo } = await payment.createRecharge();
    const callback = {
      money: '12.34',
      out_trade_no: attemptNo,
      pid: 'e2e-merchant',
      sign_type: 'MD5',
      trade_no: `e2e-${attemptNo}`,
      trade_status: 'TRADE_SUCCESS',
    };
    const body = { ...callback, sign: sign(callback) };
    const target = `${backendOrigin}/x/idc-payment/api/v1/callbacks/payments/${channelNo}`;
    const first = await page.request.post(target, { form: body });
    const second = await page.request.post(target, { form: body });
    expect(first.ok()).toBe(true);
    expect(second.ok()).toBe(true);
    const result = await payment.openReturn(attemptNo);
    await expect.poll(async () => {
      if ((await result.textContent())?.includes('充值已经到账')) return true;
      await page.getByRole('button', { name: '刷新状态' }).click();
      return (await result.textContent())?.includes('充值已经到账') || false;
    }, { timeout: 30_000 }).toBe(true);
	await expect
	  .poll(async () => Math.round((await payment.balance()) * 100), { timeout: 30_000 })
	  .toBe(Math.round(before * 100) + 1234);
    await capturePaymentScreenshot(page, 'payment-duplicate-callback-single-posting');
  });
});
