import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

type HistoryItem = {
  currencyCode: string;
  decisionReasonCode?: string;
  grossAmountMinor: string;
  invoiceTypeCode: string;
  statusCode: string;
  submittedAt: number;
  taxInvoiceNo: string;
  version: number;
};

function envelope(data: unknown) {
  return JSON.stringify({ code: 0, data, message: 'OK' });
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({
    body: envelope(data),
    contentType: 'application/json',
    status: 200,
  });
}

export async function installTaxInvoiceMock(
  page: Page,
  options: { locale?: 'en-US' | 'zh-CN' } = {},
) {
  await mockCustomerAPIs(page, options);
  const invoices = [
    {
      alreadyAllocatedMinor: '2000',
      billingInvoiceNo: 'BILL-CNY-001',
      currencyCode: 'CNY',
      orderNo: 'ORDER-CNY-001',
      remainingGrossMinor: '8000',
      settledAt: 1_786_320_000_000,
      sourceGrossMinor: '10000',
    },
    {
      alreadyAllocatedMinor: '0',
      billingInvoiceNo: 'BILL-CNY-002',
      currencyCode: 'CNY',
      orderNo: 'ORDER-CNY-002',
      remainingGrossMinor: '5000',
      settledAt: 1_786_321_000_000,
      sourceGrossMinor: '5000',
    },
    {
      alreadyAllocatedMinor: '0',
      billingInvoiceNo: 'BILL-USD-001',
      currencyCode: 'USD',
      orderNo: 'ORDER-USD-001',
      remainingGrossMinor: '4000',
      settledAt: 1_786_322_000_000,
      sourceGrossMinor: '4000',
    },
  ];
  const profiles = [
    {
      bankAccountMasked: '****5678',
      bankName: '测试银行',
      invoiceTitle: '客户默认企业',
      isDefault: true,
      profileNo: 'PROFILE-DEFAULT',
      registeredAddress: '上海',
      registeredPhone: '021-12345678',
      taxRegistrationNo: 'TAX-DEFAULT',
      titleTypeCode: 'enterprise',
      version: 2,
    },
    {
      invoiceTitle: '个人备用抬头',
      isDefault: false,
      profileNo: 'PROFILE-PERSONAL',
      titleTypeCode: 'individual',
      version: 1,
    },
  ];
  let history: HistoryItem[] = [
    {
      currencyCode: 'CNY',
      decisionReasonCode: 'amount_or_billing_invoice_adjustment_required',
      grossAmountMinor: '3000',
      invoiceTypeCode: 'pending_confirmation',
      statusCode: 'rejected',
      submittedAt: 1_786_300_000_000,
      taxInvoiceNo: 'TINV-REJECTED',
      version: 2,
    },
  ];

  await page.route(
    '**/account/compliance/tax-invoice-options**',
    async (route) => {
      await fulfill(route, { invoices, observedAt: Date.now(), profiles });
    },
  );
  await page.route('**/account/compliance/tax-invoices**', async (route) => {
    if (route.request().method() === 'GET') {
      await fulfill(route, { items: history, nextCursor: '' });
      return;
    }
    if (route.request().method() === 'POST') {
      const input = route.request().postDataJSON() as {
        selections: Array<{ requestedGrossMinor: string }>;
      };
      const total = input.selections.reduce(
        (sum, selection) => sum + BigInt(selection.requestedGrossMinor),
        0n,
      );
      const item: HistoryItem = {
        currencyCode: 'CNY',
        grossAmountMinor: String(total),
        invoiceTypeCode: 'pending_confirmation',
        statusCode: 'pending_review',
        submittedAt: Date.now(),
        taxInvoiceNo: 'TINV-CREATED',
        version: 1,
      };
      history = [item, ...history];
      await fulfill(route, { item, reused: false });
      return;
    }
    await route.fallback();
  });
}
