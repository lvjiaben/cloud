import type { Page } from '@playwright/test';
import type { PaymentDirectory } from '../pages/PaymentPagedTablesPage';

/** Deterministic API-owned pages; no payment/supplier operations are mocked as success. */
export async function installPaymentPagesMock(page: Page) {
  const state = { failures: {} as Partial<Record<PaymentDirectory, number>>, reads: [] as { kind: PaymentDirectory; page: number; pageSize: number; channelNo: string }[], writes: 0 };
  await page.route('**/x/idc-payment/api/v1/admin/**', async route => {
    const request = route.request();
    const url = new URL(request.url());
    if (request.method() !== 'GET') { state.writes += 1; await route.fulfill({ status: 403, json: { code: 403 } }); return; }
    const kind: PaymentDirectory | undefined = url.pathname.endsWith('/payment-channels') ? 'channels' : url.pathname.endsWith('/channels') ? 'health' : url.pathname.endsWith('/batches') ? 'batches' : url.pathname.endsWith('/differences') ? 'differences' : url.pathname.endsWith('/payment-exception-funds') ? 'funds' : undefined;
    if (!kind) { await route.fallback(); return; }
    const number = Number(url.searchParams.get('page') || 1);
    const size = Number(url.searchParams.get('pageSize') || 20);
    const channelNo = url.searchParams.get('channelNo') || '';
    state.reads.push({ kind, page: number, pageSize: size, channelNo });
    if (state.failures[kind]) { const status = state.failures[kind]!; delete state.failures[kind]; await route.fulfill({ status, json: { code: status } }); return; }
    const total = kind === 'differences' ? 521 : kind === 'batches' ? (channelNo ? 1 : 122) : 121;
    const items = Array.from({ length: Math.max(0, Math.min(size, total - (number - 1) * size)) }, (_, offset) => {
      const index = (number - 1) * size + offset + 1;
      if (kind === 'channels') return { channelNo: `channel-${index}`, displayName: `Payment channel ${index}`, providerCode: 'epay', merchantReference: `Merchant ${index}`, modes: ['epay_cashier'], currencies: ['CNY'], statusCode: 'disabled', version: 1, verifiedAt: 0 };
      if (kind === 'health') return { channelNo: `channel-${index}`, displayName: `Payment channel ${index}`, providerCode: 'epay', capabilityCode: 'unavailable', healthCode: 'never_run', resultUnknown: index === 1, unresolvedCount: 0 };
      if (kind === 'batches') return { batchNo: `batch-${index}`, channelNo: channelNo || 'channel-1', capabilityCode: 'unavailable', statusCode: 'differences', businessDate: 1788470400000, differenceCount: 521, paymentCount: 521, refundCount: 0, resultUnknown: index === 1 };
      if (kind === 'differences') return { lineNo: `line-${index}`, externalReference: `TRADE-${index}`, differenceCode: 'amount_mismatch', channelStatusCode: 'succeeded', localStatusCode: 'pending', localAmountMinor: '100', channelAmountMinor: '1250', currencyCode: 'CNY', resolutionCode: 'pending', version: 1, supplementRequestNo: '', supplementStatusCode: '' };
      return { exceptionNo: `fund-${index}`, orderNo: `ORDER-${index}`, attemptNo: `ATTEMPT-${index}`, channelNo: 'channel-1', providerCode: 'epay', currencyCode: 'CNY', amountMinor: '1250', reasonCode: 'late_success', statusCode: 'pending', actionCode: '', actionStatus: '', version: 1 };
    });
    const key = kind === 'health' || kind === 'channels' ? 'channels' : kind;
    await route.fulfill({ json: { code: 0, data: { [key]: items, total, ...(kind === 'channels' ? { providerCandidates: [] } : {}) } } });
  });
  return state;
}
