import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-core';

export interface CustomerReferralOverview {
  referralCode: string;
  referralUrl: string;
  attributionPolicyCode: string;
  attributionExpiresAt?: number;
  referredCustomerCount: number;
  attributedOrderCount: number;
  qualifiedOrderCount: number;
  rejectedAttributionCount: number;
}

export interface CustomerCommissionAccount {
  currencyCode: string;
  availableAmountMinor: string;
  pendingAmountMinor: string;
  maturedAmountMinor: string;
  settledAmountMinor: string;
  frozenAmountMinor: string;
  statusCode: string;
}

export interface CustomerCommissionEntry {
  entryNo: string;
  currencyCode: string;
  amountMinor: string;
  statusCode: string;
  sourceOrderNo?: string;
  maturityAt?: number;
  reversedAmountMinor?: string;
  createdAt: number;
}

export interface CustomerCommissionOverview {
  accounts: CustomerCommissionAccount[];
  entries: CustomerCommissionEntry[];
  nextCursor: string;
}

export interface CustomerCommissionSettlement {
  settlementNo: string;
  currencyCode: string;
  amountMinor: string;
  statusCode: string;
  submittedAt: number;
  reviewedAt?: number;
  paidAt?: number;
  failureReasonCode?: string;
}

function path(pathName: string) {
  return pluginApiPath(pluginID, pathName);
}

export function getCustomerReferralOverview() {
  return requestClient.get<CustomerReferralOverview>(
    path('account/sales-channel/referral'),
  );
}

export function createCustomerReferralLink(input: {
  policyCode: 'first_touch' | 'last_touch';
  attributionWindowSeconds: number;
  expiresAt?: number;
  idempotencyKey: string;
}) {
  return requestClient.post<{
    item: { linkNo: string; policy: string; status: string };
    reused: boolean;
  }>(path('account/sales-channel/referral-links'), input);
}

export function getCustomerCommissionOverview(
  params: { cursor?: string; currencyCode?: string; limit?: number } = {},
) {
  return requestClient.get<CustomerCommissionOverview>(
    path('account/sales-channel/commission/overview'),
    { params: { limit: 20, ...params } },
  );
}

export function listCustomerCommissionSettlements(
  params: { cursor?: string; currencyCode?: string; limit?: number } = {},
) {
  return requestClient.get<{
    items: CustomerCommissionSettlement[];
    nextCursor: string;
  }>(path('account/sales-channel/commission/settlements'), {
    params: { limit: 20, ...params },
  });
}

export function createCustomerCommissionSettlementRequest(input: {
  amountMinor: string;
  currencyCode: string;
  idempotencyKey: string;
}) {
  return requestClient.post<{
    item: CustomerCommissionSettlement;
    reused: boolean;
  }>(path('account/sales-channel/commission/settlement-requests'), input);
}
