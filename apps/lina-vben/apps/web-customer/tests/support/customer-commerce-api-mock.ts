import type { Page } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

export const commerceOrderNo = 'ORD_20260811_0001';
const now = 1_786_406_400_000;

export interface CommerceMockState {
  cancellationKeys: string[];
  checkoutKeys: string[];
  mergeAttempts: number;
  paymentReady: boolean;
  quoteRequests: number;
}

function cart(version = 4) {
  return {
    cartNo: 'CART_20260811_0001',
    currencyCode: 'CNY',
    items: [
      {
        cartItemNo: 'CARTITEM_20260811_0001',
        configuration: [
          {
            componentTypeCode: 'cpu',
            groupNameEnUS: 'CPU',
            groupNameZhCN: '处理器',
            optionNameEnUS: '2 cores',
            optionNameZhCN: '2 核',
            optionNo: 'cpu-basic',
            quantity: 1,
          },
          {
            componentTypeCode: 'memory',
            groupNameEnUS: 'Memory',
            groupNameZhCN: '内存',
            optionNameEnUS: '4 GB',
            optionNameZhCN: '4 GB',
            optionNo: 'memory-basic',
            quantity: 1,
          },
        ],
        issueCode: '',
        productNameEnUS: 'Starter cloud server',
        productNameZhCN: '入门云服务器',
        purchaseCycles: 1,
        repairAction: '',
        quantity: 1,
        statusCode: 'active',
        version: 2,
      },
    ],
    version,
  };
}

function summary(cancelled = false) {
  return {
    canCancel: !cancelled,
    canPreparePayment: !cancelled,
    createdAt: now,
    currencyCode: 'CNY',
    itemCount: 1,
    orderNo: commerceOrderNo,
    paymentDueAt: now + 1_800_000,
    requiresContract: false,
    riskDecisionCode: 'approved',
    statusCode: cancelled ? 'cancelled' : 'pending_payment',
    totalAmountMinor: '10912',
  };
}

function preparation(ready: boolean) {
  return {
    amountDueMinor: '10912',
    contextNo: 'PAYCTX_20260811_0001',
    currencyCode: 'CNY',
    customerTypeCode: 'individual',
    identityStatusCode: ready ? 'verified' : 'pending',
    invoiceNo: 'INV_20260811_0001',
    lastEvaluatedAt: now,
    orderNo: commerceOrderNo,
    paymentDueAt: now + 1_800_000,
    ready,
    reasonCode: ready ? 'ready' : 'identity_required',
    reused: ready,
    statusCode: ready ? 'ready' : 'awaiting_identity',
  };
}

function detail(state: CommerceMockState) {
  const cancelled = state.cancellationKeys.length > 0;
  return {
    cancellation: cancelled
      ? {
          cancellationNo: 'CAN_20260811_0001',
          reasonCode: 'customer_request',
          statusCode: 'completed',
        }
      : undefined,
    invoice: {
      invoiceNo: 'INV_20260811_0001',
      paymentDueAt: now + 1_800_000,
      statusCode: cancelled ? 'cancelled' : 'open',
      totalAmountMinor: '10912',
    },
    items: [
      {
        orderItemNo: 'OI_20260811_0001',
        productNameEnUS: 'Starter cloud server',
        productNameZhCN: '入门云服务器',
        purchaseCycles: 1,
        quantity: 1,
        totalAmountMinor: '10912',
      },
    ],
    paymentPreparation: {
      contextNo: 'PAYCTX_20260811_0001',
      identityStatusCode: state.paymentReady ? 'verified' : 'pending',
      lastEvaluatedAt: now,
      reasonCode: state.paymentReady ? 'ready' : 'identity_required',
      statusCode: state.paymentReady ? 'ready' : 'awaiting_identity',
    },
    deliveries: [
      {
        ongoingService: true,
        productNameEnUS: 'Starter cloud server',
        productNameZhCN: '入门云服务器',
        quantity: 2,
        statusCode: 'processing',
        updatedAt: now,
      },
      {
        ongoingService: false,
        productNameEnUS: 'Certificate',
        productNameZhCN: 'SSL 证书',
        quantity: 1,
        statusCode: 'not_completed',
        updatedAt: now,
      },
    ],
    serviceEntitlements: [
      {
        productNameEnUS: 'Starter cloud server',
        productNameZhCN: '入门云服务器',
        startedAt: now,
        statusCode: 'active',
      },
    ],
    reviews: [],
    summary: summary(cancelled),
  };
}

export async function installCustomerCommerceMock(
  page: Page,
  options: { locale?: 'en-US' | 'zh-CN' } = {},
) {
  await mockCustomerAPIs(page, options);
  const state: CommerceMockState = {
    cancellationKeys: [],
    checkoutKeys: [],
    mergeAttempts: 0,
    paymentReady: false,
    quoteRequests: 0,
  };

  await page.route(
    '**/x/idc-core/api/v1/account/commerce/**',
    async (route) => {
      const request = route.request();
      const requestURL = new URL(request.url());
      const method = request.method();
      const requestPath = requestURL.pathname;
      let data: unknown;

      if (requestPath.endsWith('/guest-cart-merge') && method === 'POST') {
        state.mergeAttempts += 1;
        data = { merged: false };
      } else if (requestPath.endsWith('/cart') && method === 'GET') {
        data = { cart: cart() };
      } else if (requestPath.includes('/cart-items/') && method === 'DELETE') {
        data = { cart: { ...cart(5), items: [] } };
      } else if (requestPath.endsWith('/quotes') && method === 'POST') {
        state.quoteRequests += 1;
        const changed = state.quoteRequests > 1;
        data = {
          quote: {
            cartNo: 'CART_20260811_0001',
            cartVersion: 4 + state.quoteRequests,
            currencyCode: 'CNY',
            discountMinor: '0',
            expiresAt: now + 600_000,
            issuedAt: now,
            items: [],
            netAmountMinor: changed ? '10300' : '10200',
            quoteNo: 'QUOTE_20260811_0001',
            subtotalMinor: changed ? '10300' : '10200',
            taxAmountMinor: '612',
            totalAmountMinor: changed ? '10912' : '10812',
          },
        };
      } else if (requestPath.endsWith('/orders') && method === 'POST') {
        const body = request.postDataJSON() as { idempotencyKey: string };
        state.checkoutKeys.push(body.idempotencyKey);
        data = {
          checkout: {
            currencyCode: 'CNY',
            invoiceNo: 'INV_20260811_0001',
            itemCount: 1,
            orderNo: commerceOrderNo,
            paymentDueAt: now + 1_800_000,
            quoteNo: 'QUOTE_20260811_0001',
            requiresContract: false,
            reused: state.checkoutKeys.length > 1,
            statusCode: 'pending_payment',
            totalAmountMinor: '10912',
          },
        };
      } else if (
        requestPath.endsWith('/payment-preparations') &&
        method === 'POST'
      ) {
        data = { preparation: preparation(state.paymentReady) };
      } else if (
        requestPath.includes('/payment-preparations/') &&
        method === 'PUT'
      ) {
        state.paymentReady = true;
        data = { preparation: preparation(true) };
      } else if (requestPath.endsWith('/cancellation') && method === 'PUT') {
        const body = request.postDataJSON() as { idempotencyKey: string };
        state.cancellationKeys.push(body.idempotencyKey);
        data = {
          cancellation: {
            cancellationNo: 'CAN_20260811_0001',
            statusCode: 'completed',
          },
        };
      } else if (
        requestPath.endsWith(`/orders/${commerceOrderNo}`) &&
        method === 'GET'
      ) {
        data = { order: detail(state) };
      } else if (requestPath.endsWith('/orders') && method === 'GET') {
        data = { nextCursor: '', orders: [summary()] };
      } else {
        await route.fallback();
        return;
      }

      await route.fulfill({
        body: JSON.stringify({ code: 0, data, message: 'OK' }),
        contentType: 'application/json',
        status: 200,
      });
    },
  );
  return state;
}
