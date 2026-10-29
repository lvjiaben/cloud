import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-core';

export interface FinancialBalance {
  accountTypeCode: string;
  availableBalanceMinor: string;
  balanceMinor: string;
  currencyCode: string;
  heldBalanceMinor: string;
  statusCode: string;
  updatedAt: number;
  withdrawable: boolean;
}

export interface FinancialOverview {
  automaticRenewalEnabled: number;
  balances: FinancialBalance[];
  openInvoiceCount: number;
  openReceivables: Array<{
    currencyCode: string;
    outstandingMinor: string;
  }>;
  rechargeAvailable: boolean;
  rechargeReasonCode: string;
}

export interface FinancialInvoice {
  canMerge: boolean;
  canPayFromBalance: boolean;
  createdAt: number;
  currencyCode: string;
  invoiceNo: string;
  issuedAt?: number;
  itemCount: number;
  orderNo?: string;
  outstandingAmountMinor: string;
  paidAmountMinor: string;
  paymentDueAt: number;
  refundedAmountMinor: string;
  refundableAmountMinor: string;
  settledAt?: number;
  sourceCode: string;
  statusCode: string;
  totalAmountMinor: string;
  waivedAmountMinor: string;
}

export interface FinancialInvoiceItem {
  descriptionEnUS: string;
  descriptionZhCN: string;
  discountMinor: string;
  invoiceItemNo: string;
  itemTypeCode: string;
  lineNumber: number;
  netAmountMinor: string;
  sourceReference: string;
  subtotalMinor: string;
  taxAmountMinor: string;
  totalAmountMinor: string;
}

export interface FinancialInvoiceDetail {
  invoice: FinancialInvoice;
  items: FinancialInvoiceItem[];
  nextItemCursor: string;
}

export interface FinancialTransaction {
  accountTypeCode: string;
  amountMinor: string;
  changeMinor: string;
  currencyCode: string;
  postedAt: number;
  relatedInvoiceNo?: string;
  transactionNo: string;
  transactionTypeCode: string;
}

export interface FinancialRenewalEntitlement {
  autoRenewEnabled: boolean;
  canConfigureAutomaticRenewal: boolean;
  currencyCode: string;
  entitlementExpiresAt: number;
  entitlementStatusCode: string;
  failureNotificationEnabled: boolean;
  latestAttemptAt?: number;
  latestAttemptFailureCode?: string;
  latestAttemptOutcomeCode?: string;
  leadSeconds: number;
  maximumAttempts: number;
  nextAttemptAt?: number;
  periodCount: number;
  periodUnitCode: string;
  policyNo?: string;
  policyVersion?: number;
  productNameEnUS: string;
  productNameZhCN: string;
  renewalCycles: number;
  retryIntervalSeconds: number;
  entitlementNo: string;
  entitlementVersion: number;
}

function path(pathName: string) {
  return pluginApiPath(pluginID, `account/billing/${pathName}`);
}

export function getFinancialOverview() {
  return requestClient.get<{ overview: FinancialOverview }>(path('overview'));
}

export function listFinancialInvoices(
  params: {
    currencyCode?: string;
    cursor?: string;
    limit?: number;
    statusCode?: string;
  } = {},
) {
  return requestClient.get<{
    invoices: FinancialInvoice[];
    nextCursor: string;
  }>(path('invoices'), { params: { limit: 20, ...params } });
}

export function getFinancialInvoice(invoiceNo: string, itemCursor = '') {
  return requestClient.get<{ detail: FinancialInvoiceDetail }>(
    path(`invoices/${encodeURIComponent(invoiceNo)}`),
    { params: { itemCursor: itemCursor || undefined, itemLimit: 50 } },
  );
}

export function listFinancialTransactions(
  params: {
    accountType?: string;
    currencyCode?: string;
    cursor?: string;
    limit?: number;
  } = {},
) {
  return requestClient.get<{
    nextCursor: string;
    transactions: FinancialTransaction[];
  }>(path('transactions'), {
    params: { accountType: 'customer_balance', limit: 20, ...params },
  });
}

export function listFinancialRenewalEntitlements(
  params: {
    cursor?: string;
    limit?: number;
    statusCode?: string;
  } = {},
) {
  return requestClient.get<{
    entitlements: FinancialRenewalEntitlement[];
    nextCursor: string;
  }>(path('renewal-entitlements'), { params: { limit: 20, ...params } });
}

export function payFinancialInvoiceFromBalance(
  invoiceNo: string,
  idempotencyKey: string,
  silentErrorMessage = false,
) {
  return requestClient.post(
    path(`invoices/${encodeURIComponent(invoiceNo)}/balance-payments`),
    { idempotencyKey },
    { silentErrorMessage },
  );
}

export function mergeFinancialInvoices(
  invoiceNos: string[],
  idempotencyKey: string,
) {
  return requestClient.post(path('invoice-merges'), {
    idempotencyKey,
    invoiceNos,
  });
}

export function updateFinancialRenewalPolicy(
  entitlementNo: string,
  input: {
    autoRenewEnabled: boolean;
    expectedEntitlementVersion: number;
    failureNotificationEnabled: boolean;
    leadSeconds: number;
    maximumAttempts: number;
    renewalCycles: number;
    retryIntervalSeconds: number;
  },
) {
  return requestClient.put(
    path(`renewal-entitlements/${encodeURIComponent(entitlementNo)}/policy`),
    input,
  );
}
