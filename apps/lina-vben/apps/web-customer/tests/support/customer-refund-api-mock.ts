import type { Page } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

export const refundOrderNo = 'ORD_REFUND_0001';
export const refundInvoiceNo = 'INV_REFUND_0001';
export const refundInvoiceItemNo = 'IVI_REFUND_0001';
export const originalRefundNo = 'RFD_ORIGINAL_0001';
export const balanceRefundNo = 'RFD_BALANCE_0001';
const now = 1_786_406_400_000;

type Body = Record<string, unknown>;

export interface CustomerRefundMockState {
  consentBodies: Body[];
  previewBodies: Body[];
  refunds: Body[];
  submitBodies: Body[];
}

function envelope(data: unknown) {
  return { code: 0, data, message: 'OK' };
}

function refund(
  refundNo: string,
  statusCode: string,
  destinationCode: string,
  version: number,
) {
  return {
    amountMinor: '5000',
    approvalModeCode: 'manual',
    approvedAt: statusCode === 'succeeded' ? now + 60_000 : undefined,
    completedAt: statusCode === 'succeeded' ? now + 120_000 : undefined,
    currencyCode: 'CNY',
    destinationCode,
    hasDangerousOperation: false,
    hasEntitlementImpact: true,
    highAmount: false,
    impactDigest: 'a'.repeat(64),
    invoiceNo: refundInvoiceNo,
    orderNo: refundOrderNo,
    reasonCode: 'wrong_configuration',
    reasonDetail: '',
    refundNo,
    requiresEntitlementTermination: true,
    reused: false,
    statusCode,
    submittedAt: now,
    version,
  };
}

function preview(body: Body) {
  const selections = Array.isArray(body.selections) ? body.selections : [];
  const selectedQuantity = Number((selections[0] as Body | undefined)?.quantity || 1);
  const entitlementSelected = selections.some(
	(item) => Array.isArray((item as Body).entitlementNos) &&
		((item as Body).entitlementNos as unknown[]).length > 0,
  );
  return {
    amountMinor: '5000',
    approvalModeCode: 'manual',
    currencyCode: 'CNY',
    hasDangerousOperation: false,
    hasEntitlementImpact: entitlementSelected,
    highAmount: false,
    impactDigest: 'a'.repeat(64),
    invoiceNo: refundInvoiceNo,
    items: [
      {
        amountMinor: '5000',
        descriptionEnUS: 'Production cloud server',
        descriptionZhCN: '生产云服务器',
        impacts: [
          {
            entitlementVersion: 7,
            productNameEnUS: 'Production cloud service',
            productNameZhCN: '生产云服务',
            requiresTermination: true,
            statusCode: 'active',
          },
        ].filter(() => entitlementSelected),
        invoiceItemNo: refundInvoiceItemNo,
        orderItemNo: 'ORI_REFUND_0001',
        quantity: selectedQuantity,
        refundableAmountMinor: '10912',
      },
    ],
    orderNo: refundOrderNo,
    requiresEntitlementTermination: entitlementSelected,
  };
}

export async function installCustomerRefundMock(
  page: Page,
  options: { locale?: 'en-US' | 'zh-CN' } = {},
) {
  await mockCustomerAPIs(page, options);
  const state: CustomerRefundMockState = {
    consentBodies: [],
    previewBodies: [],
    refunds: [
      refund(balanceRefundNo, 'awaiting_balance_consent', 'mixed', 2),
      refund(originalRefundNo, 'succeeded', 'original_channel', 4),
    ],
    submitBodies: [],
  };

  await page.route('**/x/idc-core/api/v1/account/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();

    if (
      path.endsWith(`/commerce/orders/${refundOrderNo}`) &&
      method === 'GET'
    ) {
      await route.fulfill({
        json: envelope({
          order: {
            deliveries: [],
            invoice: { invoiceNo: refundInvoiceNo, statusCode: 'paid' },
            items: [
              {
                orderItemNo: 'ORI_REFUND_0001',
                quantity: 2,
              },
            ],
            reviews: [],
            serviceEntitlements: [
              {
                entitlementNo: 'ENT_REFUND_0001',
                orderItemNo: 'ORI_REFUND_0001',
                productNameEnUS: 'Production cloud service',
                productNameZhCN: '生产云服务',
                startedAt: now - 60_000,
                statusCode: 'active',
              },
            ],
            summary: {
              canCancel: false,
              canPreparePayment: false,
              createdAt: now,
              currencyCode: 'CNY',
              itemCount: 1,
              orderNo: refundOrderNo,
              requiresContract: false,
              riskDecisionCode: 'approved',
              statusCode: 'paid',
              totalAmountMinor: '10912',
            },
          },
        }),
      });
      return;
    }
    if (
      path.endsWith(`/billing/invoices/${refundInvoiceNo}`) &&
      method === 'GET'
    ) {
      await route.fulfill({
        json: envelope({
          detail: {
            invoice: {
              currencyCode: 'CNY',
              invoiceNo: refundInvoiceNo,
              orderNo: refundOrderNo,
              refundableAmountMinor: '10912',
              statusCode: 'paid',
              totalAmountMinor: '10912',
            },
            items: [
              {
                descriptionEnUS: 'Production cloud server',
                descriptionZhCN: '生产云服务器',
                invoiceItemNo: refundInvoiceItemNo,
                lineNumber: 1,
                sourceReference: 'ORI_REFUND_0001',
                totalAmountMinor: '10912',
              },
            ],
            nextItemCursor: '',
          },
        }),
      });
      return;
    }
    if (path.endsWith('/refunds/preview') && method === 'POST') {
      const body = request.postDataJSON() as Body;
      state.previewBodies.push(body);
      await route.fulfill({ json: envelope({ preview: preview(body) }) });
      return;
    }
    if (path.endsWith('/balance-consent') && method === 'POST') {
      const body = request.postDataJSON() as Body;
      state.consentBodies.push(body);
      const index = state.refunds.findIndex(
        (item) => item.refundNo === balanceRefundNo,
      );
      const current = state.refunds[index] as Body;
      const consent = body.consent === true;
      state.refunds[index] = {
        ...current,
        destinationCode: consent ? 'mixed' : current.destinationCode,
        statusCode: consent ? 'approved' : 'awaiting_balance_consent',
        version: Number(current.version) + 1,
      };
      await route.fulfill({
        json: envelope({ refund: state.refunds[index] }),
      });
      return;
    }
    if (path.endsWith('/refunds') && method === 'POST') {
      const body = request.postDataJSON() as Body;
      state.submitBodies.push(body);
      const created = {
        ...refund('RFD_CREATED_0001', 'pending_review', 'original_channel', 1),
        amountMinor: '5000',
        reasonCode: body.reasonCode,
      };
      state.refunds.unshift(created);
      await route.fulfill({ json: envelope({ refund: created }) });
      return;
    }
    if (path.endsWith('/refunds') && method === 'GET') {
      await route.fulfill({
        json: envelope({ nextCursor: '', refunds: state.refunds }),
      });
      return;
    }
    await route.fulfill({ status: 404, json: envelope({}) });
  });
  return state;
}
