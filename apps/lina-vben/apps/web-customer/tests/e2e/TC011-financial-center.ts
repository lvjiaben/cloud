import { createHash } from 'node:crypto';

import { FinancePage } from '../pages/FinancePage';
import {
  balanceInvoiceNo,
  installCustomerFinanceMock,
  mergeInvoiceNos,
  renewalEntitlementNo,
} from '../support/customer-finance-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';
import { expect, test } from '../fixtures/customer';

function signEPayCallback(values: Record<string, string>, secret: string) {
  const canonical = Object.entries(values)
    .filter(([key, value]) => value && key !== 'sign' && key !== 'sign_type')
    .toSorted(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`)
    .join('&');
  return createHash('md5').update(`${canonical}${secret}`).digest('hex');
}

test.describe('TC011 客户财务中心', () => {
  test('TC011a: 支付通道缺失时充值关闭且超大金额保持精确', async ({ page }) => {
    const state = await installCustomerFinanceMock(page);
    const finance = new FinancePage(page);
    await page.setViewportSize({ height: 844, width: 390 });
    await finance.login();
    await finance.open();
    await expect(finance.root).toContainText('CNY 9007199254740993.01');
    await expect(finance.root).toContainText('充值入口暂不可用');
    await expect(page.getByRole('button', { name: '账户充值' })).toBeDisabled();
    expect(await finance.hasHorizontalOverflow()).toBe(false);
    expect(state.unexpectedWrites).toHaveLength(0);
  });

  test('TC011b: 余额支付只提交稳定请求号并刷新权威账单', async ({ page }) => {
    const state = await installCustomerFinanceMock(page);
    const finance = new FinancePage(page);
    await finance.login();
    await finance.open(`/account/billing/invoices/${balanceInvoiceNo}`);
    await page.getByTestId('finance-balance-pay').click();
    await page.getByRole('button', { name: '确认支付' }).click();
    await expect.poll(() => state.balancePaymentBodies.length).toBe(1);
    const body = state.balancePaymentBodies[0] || {};
    expect(Object.keys(body)).toEqual(['idempotencyKey']);
    expect(String(body.idempotencyKey)).toMatch(/^customer-/u);
    await expect(finance.root).toContainText('已付清');
    await expect(finance.root).toContainText('CNY 0.00');
  });

  test('TC011c: 合并账单只提交排序后的来源编号和稳定请求号', async ({
    page,
  }) => {
    const state = await installCustomerFinanceMock(page);
    const finance = new FinancePage(page);
    await finance.login();
    await finance.open('/account/billing/invoices');
    await finance.invoice(mergeInvoiceNos[1]).getByRole('checkbox').check();
    await finance.invoice(mergeInvoiceNos[0]).getByRole('checkbox').check();
    await page.getByRole('button', { name: '合并所选账单' }).click();
    await page.getByRole('button', { name: '确认合并' }).click();
    await expect.poll(() => state.mergeBodies.length).toBe(1);
    const body = state.mergeBodies[0] || {};
    expect(body.invoiceNos).toEqual([...mergeInvoiceNos]);
    expect(String(body.idempotencyKey)).toMatch(/^customer-/u);
    expect(body).not.toHaveProperty('customerNo');
    await expect(finance.root).toContainText('INV_MERGED_0001');
  });

  test('TC011d: 英文手机显示脱敏的余额不足续费提醒', async ({ page }) => {
    await page.setViewportSize({ height: 844, width: 390 });
    await installCustomerFinanceMock(page, { locale: 'en-US' });
    const finance = new FinancePage(page);
    await finance.login();
    await finance.open('/account/billing/renewals');
    const alert = page.getByTestId(`renewal-attempt-${renewalEntitlementNo}`);
    await expect(alert).toContainText('Automatic renewal could not finish');
    await expect(finance.root).not.toContainText('balance.insufficient');
    await expect(finance.root).not.toContainText(renewalEntitlementNo);
    await expect(finance.root).toContainText('Entitlement status');
    await expect(finance.root).toContainText('Active');
    expect(await finance.hasHorizontalOverflow()).toBe(false);
    await captureCustomerScreenshot(page, 'customer-finance-renewal-en-mobile');
  });

  test('TC011e: 充值只创建支付意图，扫码返回仍等待服务端确认', async ({
    page,
  }) => {
    const state = await installCustomerFinanceMock(page, {
      paymentAvailable: true,
    });
    const finance = new FinancePage(page);
    await finance.login();
    await finance.open();
    await page.getByTestId('finance-recharge-open').click();
    await page.getByRole('spinbutton', { name: '充值金额' }).fill('12.34');
    await page.getByRole('button', { name: '继续付款' }).click();
    await expect.poll(() => state.rechargeBodies.length).toBe(1);
    expect(state.rechargeBodies[0]).toEqual({
      amountMinor: '1234',
      channelNo: 'PAYCH_WECHAT_0001',
      currencyCode: 'CNY',
      modeCode: 'wechat_native',
    });
    expect(state.rechargeBodies[0]).not.toHaveProperty('customerNo');
    await expect(
      page.getByRole('dialog', { name: '微信扫码付款' }),
    ).toContainText('微信扫码付款');
    await page.getByRole('button', { name: '查看到账状态' }).click();
    await expect(page).toHaveURL(/\/payment\/return\/PAT_RECHARGE_0001$/u);
    await expect(page.getByTestId('payment-return-page')).toContainText(
      '正在等待支付平台确认',
    );
    await expect(page.getByTestId('payment-return-page')).not.toContainText(
      '充值已经到账',
    );
  });

  test('TC011f: 安装后的本地系统完成已验签充值和真实余额入账', async ({
    page,
  }) => {
    test.skip(
      process.env.E2E_REAL_RECHARGE !== '1',
      '仅在隔离的已安装本地运行时执行真实充值证据',
    );
    await page.route(
      'https://payments.example.test/submit.php',
      async (route) => {
        await route.fulfill({
          body: '<!doctype html><title>Isolated EPay sink</title><p>submitted</p>',
          contentType: 'text/html; charset=utf-8',
          status: 200,
        });
      },
    );
    const finance = new FinancePage(page);
    await finance.login();
    await finance.open();
    const balanceCard = page.getByTestId('finance-balance-CNY');
    const balanceBeforeText =
      (await balanceCard.count()) > 0
        ? await balanceCard.locator('dd').first().textContent()
        : '';
    const balanceBefore = Number(
      String(balanceBeforeText || '0').replace(/[^0-9.-]/gu, ''),
    );
    expect(Number.isFinite(balanceBefore)).toBe(true);
    await page.getByTestId('finance-recharge-open').click();
    await page.getByRole('spinbutton', { name: '充值金额' }).fill('12.34');
    const createdResponse = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' &&
        response.url().includes('/account/balance-recharges'),
    );
    await page.getByRole('button', { name: '继续付款' }).click();
    const createdPayload = (await (await createdResponse).json()) as {
      data?: { attemptNo?: string };
    };
    const attemptNo = String(createdPayload.data?.attemptNo || '');
    expect(attemptNo).toMatch(/^PAT_[0-9a-f]{32}$/u);
    await expect(page).toHaveURL('https://payments.example.test/submit.php');

    const callback = {
      money: '12.34',
      out_trade_no: attemptNo,
      pid: 'e2e-merchant',
      sign_type: 'MD5',
      trade_no: `e2e-${attemptNo}`,
      trade_status: 'TRADE_SUCCESS',
    };
    const callbackResponse = await page.request.post(
      `${process.env.E2E_BACKEND_BASE_URL || 'http://127.0.0.1:19996'}/x/idc-payment/api/v1/callbacks/payments/PAYCH_E2E_RECHARGE_0001`,
      {
        form: {
          ...callback,
          sign: signEPayCallback(callback, 'e2e-secret'),
        },
      },
    );
    expect(callbackResponse.ok()).toBe(true);

    await page.goto(`/payment/return/${attemptNo}`);
    await expect
      .poll(
        async () => {
          const root = page.getByTestId('payment-return-page');
          if ((await root.textContent())?.includes('充值已经到账')) {
            return true;
          }
          await page.getByRole('button', { name: '刷新状态' }).click();
          return (await root.textContent())?.includes('充值已经到账') || false;
        },
        { timeout: 30_000 },
      )
      .toBe(true);
    await page.goto('/account/billing');
    await expect(finance.root).toContainText(
      `CNY ${(balanceBefore + 12.34).toFixed(2)}`,
    );
    console.log(`E2E_REAL_RECHARGE_ATTEMPT=${attemptNo}`);
  });

  for (const paymentCase of [
    {
      channel: '支付宝',
      label: '支付宝电脑网站',
      modeCode: 'alipay_page',
      suffix: 'g',
    },
    {
      channel: '支付宝',
      label: '支付宝手机网站',
      modeCode: 'alipay_wap',
      suffix: 'h',
    },
    {
      channel: '微信支付',
      label: '微信 H5',
      modeCode: 'wechat_h5',
      suffix: 'i',
    },
  ]) {
    test(`TC011${paymentCase.suffix}: ${paymentCase.label}提交精确支付模式`, async ({
      page,
    }) => {
      const state = await installCustomerFinanceMock(page, {
        paymentModes: true,
      });
      const finance = new FinancePage(page);
      await page.route('https://payments.example.test/**', (route) =>
        route.fulfill({
          body: '<p>isolated payment sink</p>',
          contentType: 'text/html',
          status: 200,
        }),
      );
      await finance.login();
      await finance.open();
      await page.getByTestId('finance-recharge-open').click();
      const dialog = page.getByRole('dialog');
      await dialog.locator('.ant-select').nth(1).click();
      await page.getByText(paymentCase.channel, { exact: true }).last().click();
      await dialog.locator('.ant-select').nth(2).click();
      await page.getByText(paymentCase.label, { exact: true }).last().click();
      await page.getByRole('spinbutton', { name: '充值金额' }).fill('12.34');
      await page.getByRole('button', { name: '继续付款' }).click();
      await expect.poll(() => state.rechargeBodies.length).toBe(1);
      expect(state.rechargeBodies[0]).toMatchObject({
        modeCode: paymentCase.modeCode,
      });
    });
  }

  test('TC011j: 微信扫码只展示二维码并等待服务端确认', async ({ page }) => {
    const state = await installCustomerFinanceMock(page, {
      paymentModes: true,
    });
    const finance = new FinancePage(page);
    await finance.login();
    await finance.open();
    await page.getByTestId('finance-recharge-open').click();
    const dialog = page.getByRole('dialog');
    await dialog.locator('.ant-select').nth(1).click();
    await page.getByText('微信支付', { exact: true }).last().click();
    await dialog.locator('.ant-select').nth(2).click();
    await page.getByText('微信扫码', { exact: true }).last().click();
    await page.getByRole('spinbutton', { name: '充值金额' }).fill('12.34');
    await page.getByRole('button', { name: '继续付款' }).click();
    await expect(
      page.getByRole('dialog', { name: '微信扫码付款' }).locator('img'),
    ).toBeVisible();
    expect(state.rechargeBodies[0]).toMatchObject({
      modeCode: 'wechat_native',
    });
    await captureCustomerScreenshot(
      page,
      'customer-payment-wechat-native-pending',
    );
  });
});
