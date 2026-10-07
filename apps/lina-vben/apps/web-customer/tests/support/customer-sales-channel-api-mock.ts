import type { Page } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

function envelope(data: unknown) {
  return { code: 0, data, message: 'OK' };
}

export async function installCustomerSalesChannelMock(
  page: Page,
  options: { locale?: 'en-US' | 'zh-CN' } = {},
) {
  await mockCustomerAPIs(page, options);
  let settlementRequests = 0;
  await page.route(
    '**/x/idc-core/api/v1/account/sales-channel/**',
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      if (path.endsWith('/referral')) {
        await route.fulfill({ json: envelope({
          referralCode: 'REF-CUSTOMER-001',
          referralUrl: 'https://example.com/r/REF-CUSTOMER-001',
          attributionPolicyCode: 'first_touch',
          attributionExpiresAt: 1_800_000_000_000,
          referredCustomerCount: 3,
          attributedOrderCount: 2,
          qualifiedOrderCount: 1,
          rejectedAttributionCount: 1,
        }) });
        return;
      }
      if (path.endsWith('/commission/overview')) {
        await route.fulfill({ json: envelope({
          accounts: [{
            currencyCode: 'CNY',
            availableAmountMinor: settlementRequests ? '7500' : '12500',
            pendingAmountMinor: '3000',
            maturedAmountMinor: '18000',
            settledAmountMinor: '5500',
            frozenAmountMinor: settlementRequests ? '5000' : '0',
            statusCode: 'active',
          }],
          entries: [{
            entryNo: 'COMM-ENTRY-001', currencyCode: 'CNY', amountMinor: '12500',
            statusCode: 'matured', sourceOrderNo: 'ORD-001',
            maturityAt: 1_790_000_000_000, createdAt: 1_789_000_000_000,
          }],
          nextCursor: '',
        }) });
        return;
      }
      if (path.endsWith('/commission/settlements')) {
        await route.fulfill({ json: envelope({
          items: settlementRequests ? [{
            settlementNo: 'SETTLE-001', currencyCode: 'CNY', amountMinor: '5000',
            statusCode: 'pending_review', submittedAt: 1_790_000_000_000,
          }] : [],
          nextCursor: '',
        }) });
        return;
      }
      if (path.endsWith('/commission/settlement-requests') && request.method() === 'POST') {
        settlementRequests += 1;
        await route.fulfill({ json: envelope({
          item: {
            settlementNo: 'SETTLE-001', currencyCode: 'CNY', amountMinor: '5000',
            statusCode: 'pending_review', submittedAt: 1_790_000_000_000,
          },
          reused: false,
        }) });
        return;
      }
      await route.fulfill({ json: { code: 404, message: 'unexpected sales-channel request' }, status: 404 });
    },
  );
  return { get settlementRequests() { return settlementRequests; } };
}
