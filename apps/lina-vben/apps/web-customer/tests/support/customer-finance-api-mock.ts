import type { Page } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

const now = 1_786_406_400_000;
export const balanceInvoiceNo = 'INV_BALANCE_0001';
export const mergeInvoiceNos = ['INV_MERGE_0001', 'INV_MERGE_0002'] as const;
export const renewalEntitlementNo = 'ENT_RENEWAL_0001';

export interface FinanceMockState {
  balancePaymentBodies: Array<Record<string, unknown>>;
  mergeBodies: Array<Record<string, unknown>>;
  merged: boolean;
  paid: boolean;
  rechargeBodies: Array<Record<string, unknown>>;
  unexpectedWrites: string[];
}

function invoice(
  invoiceNo: string,
  totalAmountMinor: string,
  options: { canMerge?: boolean; paid?: boolean } = {},
) {
  const paid = options.paid ?? false;
  return {
    canMerge: options.canMerge ?? false,
    canPayFromBalance: invoiceNo === balanceInvoiceNo && !paid,
    createdAt: now,
    currencyCode: 'CNY',
    invoiceNo,
    issuedAt: now,
    itemCount: 1,
    orderNo: `ORD_${invoiceNo}`,
    outstandingAmountMinor: paid ? '0' : totalAmountMinor,
    paidAmountMinor: paid ? totalAmountMinor : '0',
    paymentDueAt: now + 86_400_000,
    refundedAmountMinor: '0',
    refundableAmountMinor: paid ? totalAmountMinor : '0',
    settledAt: paid ? now + 1_000 : undefined,
    sourceCode: 'order',
    statusCode: paid ? 'paid' : 'pending_payment',
    totalAmountMinor,
    waivedAmountMinor: '0',
  };
}

function invoiceDetail(paid: boolean) {
  return {
    invoice: invoice(balanceInvoiceNo, '8800', { paid }),
    items: [
      {
        descriptionEnUS: 'Cloud server renewal',
        descriptionZhCN: '云服务器续费',
        discountMinor: '0',
        invoiceItemNo: 'INVITEM_BALANCE_0001',
        itemTypeCode: 'service_renewal',
        lineNumber: 1,
        netAmountMinor: '8800',
        sourceReference: renewalEntitlementNo,
        subtotalMinor: '8800',
        taxAmountMinor: '0',
        totalAmountMinor: '8800',
      },
    ],
    nextItemCursor: '',
  };
}

function renewalEntitlement() {
  return {
    autoRenewEnabled: true,
    canConfigureAutomaticRenewal: true,
    currencyCode: 'CNY',
    entitlementExpiresAt: now + 7 * 86_400_000,
    entitlementStatusCode: 'active',
    failureNotificationEnabled: true,
    latestAttemptAt: now - 60_000,
    latestAttemptFailureCode: 'balance.insufficient',
    latestAttemptOutcomeCode: 'insufficient_balance',
    leadSeconds: 604_800,
    maximumAttempts: 3,
    nextAttemptAt: now + 21_600_000,
    periodCount: 1,
    periodUnitCode: 'month',
    policyNo: 'POLICY_RENEWAL_0001',
    policyVersion: 2,
    productNameEnUS: 'Starter cloud server',
    productNameZhCN: '入门云服务器',
    renewalCycles: 1,
    retryIntervalSeconds: 21_600,
    entitlementNo: renewalEntitlementNo,
    entitlementVersion: 7,
  };
}

export async function installCustomerFinanceMock(
  page: Page,
  options: {
    locale?: 'en-US' | 'zh-CN';
    paymentAvailable?: boolean;
    paymentModes?: boolean;
  } = {},
) {
  await mockCustomerAPIs(page, options);
  const state: FinanceMockState = {
    balancePaymentBodies: [],
    mergeBodies: [],
    merged: false,
    paid: false,
    rechargeBodies: [],
    unexpectedWrites: [],
  };
  await page.route('**/x/idc-core/api/v1/account/billing/**', async (route) => {
    const request = route.request();
    const requestPath = new URL(request.url()).pathname;
    const method = request.method();
    let data: unknown;
    if (requestPath.endsWith('/overview') && method === 'GET') {
      data = {
        overview: {
          automaticRenewalEnabled: 1,
          balances: [
            {
              accountTypeCode: 'customer_balance',
              availableBalanceMinor: '900719925474099301',
              balanceMinor: '900719925474099301',
              currencyCode: 'CNY',
              heldBalanceMinor: '0',
              statusCode: 'active',
              updatedAt: now,
              withdrawable: false,
            },
          ],
          openInvoiceCount: 3,
          openReceivables: [{ currencyCode: 'CNY', outstandingMinor: '12800' }],
          rechargeAvailable: false,
          rechargeReasonCode: 'payment_owner_unavailable',
        },
      };
    } else if (
      requestPath.endsWith(`/invoices/${balanceInvoiceNo}/balance-payments`) &&
      method === 'POST'
    ) {
      state.balancePaymentBodies.push(
        request.postDataJSON() as Record<string, unknown>,
      );
      state.paid = true;
      data = { payment: { invoiceNo: balanceInvoiceNo, statusCode: 'paid' } };
    } else if (
      requestPath.endsWith(`/invoices/${balanceInvoiceNo}`) &&
      method === 'GET'
    ) {
      data = { detail: invoiceDetail(state.paid) };
    } else if (requestPath.endsWith('/invoice-merges') && method === 'POST') {
      state.mergeBodies.push(request.postDataJSON() as Record<string, unknown>);
      state.merged = true;
      data = { merge: { sourceInvoiceNos: [...mergeInvoiceNos] } };
    } else if (requestPath.endsWith('/invoices') && method === 'GET') {
      data = {
        invoices: state.merged
          ? [invoice('INV_MERGED_0001', '4000')]
          : [
              invoice(balanceInvoiceNo, '8800'),
              invoice(mergeInvoiceNos[0], '1500', { canMerge: true }),
              invoice(mergeInvoiceNos[1], '2500', { canMerge: true }),
            ],
        nextCursor: '',
      };
    } else if (
      requestPath.endsWith('/renewal-entitlements') &&
      method === 'GET'
    ) {
      data = { entitlements: [renewalEntitlement()], nextCursor: '' };
    } else {
      if (method !== 'GET') {
        state.unexpectedWrites.push(`${method} ${requestPath}`);
      }
      await route.fulfill({
        body: JSON.stringify({ code: 'NOT_FOUND', message: 'unexpected' }),
        contentType: 'application/json',
        status: 404,
      });
      return;
    }
    await route.fulfill({
      body: JSON.stringify({ code: 0, data, message: 'OK' }),
      contentType: 'application/json',
      status: 200,
    });
  });
  await page.route('**/x/idc-payment/api/v1/account/**', async (route) => {
    const request = route.request();
    const requestPath = new URL(request.url()).pathname;
    const method = request.method();
    let data: unknown;
    if (requestPath.endsWith('/payment-channels') && method === 'GET') {
      data = {
        channels: options.paymentModes
          ? [
              {
                channelNo: 'PAYCH_ALIPAY_0001',
                currencies: ['CNY'],
                displayName: '支付宝',
                modes: ['alipay_page', 'alipay_wap'],
                provider: 'alipay',
              },
              {
                channelNo: 'PAYCH_WECHAT_0001',
                currencies: ['CNY'],
                displayName: '微信支付',
                modes: ['wechat_native', 'wechat_h5'],
                provider: 'wechatpay',
              },
            ]
          : options.paymentAvailable
            ? [
                {
                  channelNo: 'PAYCH_WECHAT_0001',
                  currencies: ['CNY'],
                  displayName: '微信支付',
                  modes: ['wechat_native'],
                  provider: 'wechatpay',
                },
              ]
            : [],
      };
    } else if (
      requestPath.endsWith('/balance-recharges') &&
      method === 'POST'
    ) {
      state.rechargeBodies.push(
        request.postDataJSON() as Record<string, unknown>,
      );
      const body = state.rechargeBodies.at(-1) || {};
      const modeCode = String(body.modeCode || 'wechat_native');
      const attemptNo = options.paymentModes
        ? `PAT_${modeCode}_0001`
        : 'PAT_RECHARGE_0001';
      const redirectMethod =
        modeCode === 'wechat_native'
          ? 'QR'
          : modeCode === 'alipay_page'
            ? 'POST'
            : 'GET';
      data = {
        attemptNo,
        expiresAt: now + 15 * 60_000,
        redirectFields: { out_trade_no: attemptNo },
        redirectMethod,
        redirectUrl:
          modeCode === 'wechat_native'
            ? 'weixin://wxpay/bizpayurl?pr=customer-recharge'
            : `https://payments.example.test/${modeCode}`,
        statusCode: 'confirming',
      };
    } else if (
      requestPath.endsWith('/payments/PAT_RECHARGE_0001') &&
      method === 'GET'
    ) {
      data = {
        attemptNo: 'PAT_RECHARGE_0001',
        balanceAmountMinor: '0',
        channelAmountMinor: '1234',
        channelNo: 'PAYCH_WECHAT_0001',
        currencyCode: 'CNY',
        expiresAt: now + 15 * 60_000,
        modeCode: 'wechat_native',
        purposeCode: 'balance_recharge',
        purposeReference: 'BRC_RECHARGE_0001',
        queryQueued: true,
        statusCode: 'confirming',
        totalAmountMinor: '1234',
      };
    } else {
      if (method !== 'GET') {
        state.unexpectedWrites.push(`${method} ${requestPath}`);
      }
      await route.fulfill({
        body: JSON.stringify({ code: 'NOT_FOUND', message: 'unexpected' }),
        contentType: 'application/json',
        status: 404,
      });
      return;
    }
    await route.fulfill({
      body: JSON.stringify({ code: 0, data, message: 'OK' }),
      contentType: 'application/json',
      status: 200,
    });
  });
  return state;
}
