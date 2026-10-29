import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-core";

export interface CustomerRefundSelection {
  amountMinor: string;
  entitlementNos: string[];
  invoiceItemNo: string;
  quantity: number;
}

export interface CustomerRefundImpact {
  entitlementVersion: number;
  productNameEnUS: string;
  productNameZhCN: string;
  requiresTermination: boolean;
  statusCode: string;
}

export interface CustomerRefundPreviewItem {
  amountMinor: string;
  descriptionEnUS: string;
  descriptionZhCN: string;
  impacts: CustomerRefundImpact[];
  invoiceItemNo: string;
  orderItemNo: string;
  quantity: number;
  refundableAmountMinor: string;
}

export interface CustomerRefundPreview {
  amountMinor: string;
  approvalModeCode: "automatic" | "manual";
  currencyCode: string;
  hasDangerousOperation: boolean;
  hasEntitlementImpact: boolean;
  highAmount: boolean;
  impactDigest: string;
  invoiceNo: string;
  items: CustomerRefundPreviewItem[];
  orderNo: string;
  requiresEntitlementTermination: boolean;
}

export interface CustomerRefund {
  amountMinor: string;
  approvalModeCode: string;
  approvedAt?: number;
  completedAt?: number;
  currencyCode: string;
  destinationCode: string;
  hasDangerousOperation: boolean;
  hasEntitlementImpact: boolean;
  highAmount: boolean;
  impactDigest: string;
  invoiceNo: string;
  orderNo: string;
  reasonCode: string;
  reasonDetail: string;
  refundNo: string;
  requiresEntitlementTermination: boolean;
  reused: boolean;
  statusCode: string;
  submittedAt: number;
  version: number;
}

function path(pathName = "") {
  return pluginApiPath(pluginID, `account/refunds${pathName}`);
}

export function listCustomerRefunds(
  params: { cursor?: string; limit?: number; statusCode?: string } = {},
) {
  return requestClient.get<{
    nextCursor: string;
    refunds: CustomerRefund[];
  }>(path(), { params: { limit: 20, ...params } });
}

export function previewCustomerRefund(
  orderNo: string,
  selections: CustomerRefundSelection[],
) {
  return requestClient.post<{ preview: CustomerRefundPreview }>(
    path("/preview"),
    { orderNo, selections },
  );
}

export function submitCustomerRefund(input: {
  idempotencyKey: string;
  orderNo: string;
  previewDigest: string;
  reasonCode: string;
  reasonDetail: string;
  selections: CustomerRefundSelection[];
}) {
  return requestClient.post<{ refund: CustomerRefund }>(path(), input);
}

export function respondCustomerRefundBalance(input: {
  consent: boolean;
  expectedVersion: number;
  idempotencyKey: string;
  refundNo: string;
}) {
  const { refundNo, ...body } = input;
  return requestClient.post<{ refund: CustomerRefund }>(
    path(`/${encodeURIComponent(refundNo)}/balance-consent`),
    body,
  );
}

export function customerRefundActionKey(scope: string) {
  const storageKey = `lina-customer-tdesign-refund.action.${scope}`;
  const existing = window.sessionStorage.getItem(storageKey);
  if (existing) return existing;
  const value = `customer-refund-${crypto.randomUUID()}`;
  window.sessionStorage.setItem(storageKey, value);
  return value;
}

export function clearCustomerRefundActionKey(scope: string) {
  window.sessionStorage.removeItem(
    `lina-customer-tdesign-refund.action.${scope}`,
  );
}
