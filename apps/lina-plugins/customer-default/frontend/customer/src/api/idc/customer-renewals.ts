// Ordinary service renewals reuse core's pricing and invoice settlement owners.
import { pluginApiPath, requestClient } from '#/api/request';
export interface ServiceRenewalQuote {
  quoteNo: string;
  currencyCode: string;
  totalAmountMinor: string;
  taxAmountMinor: string;
  projectedExpiresAt: number;
  expiresAt: number;
}
const path = (value: string) => pluginApiPath('idc-core', `account/${value}`);
export function quoteServiceRenewal(entitlementNo: string, renewalCycles: number, idempotencyKey: string) {
  return requestClient.post<{quote: ServiceRenewalQuote}>(path(`entitlements/${encodeURIComponent(entitlementNo)}/renewal-quotes`), {renewalCycles, idempotencyKey}, {silentErrorMessage: true});
}
export function confirmServiceRenewal(quoteNo: string, idempotencyKey: string) {
  return requestClient.post<{operation: {operationNo: string; invoiceNo: string; statusCode: string}}>(path(`renewal-quotes/${encodeURIComponent(quoteNo)}/confirmations`), {idempotencyKey}, {silentErrorMessage: true});
}
