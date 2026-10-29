import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./commerce-request";

const pluginID = "idc-core";

export interface RefundSummary {
  amountMinor: string;
  approvalModeCode: string;
  approvedAt?: number;
  completedAt?: number;
  currencyCode: string;
  customerNo: string;
  customerCode: string;
  customerDisplayName: string;
  customerPrimaryPhone: string;
  destinationCode: string;
  hasDangerousOperation: boolean;
  hasResourceImpact: boolean;
  highAmount: boolean;
  impactDigest: string;
  invoiceNo: string;
  orderNo: string;
  reasonCode: string;
  reasonDetail: string;
  refundNo: string;
  requiresServiceTermination: boolean;
  reused: boolean;
  statusCode: string;
  submittedAt: number;
  version: number;
}

export interface RefundImpact {
	entitlementNo: string;
	entitlementVersion: number;
	requiresTermination: boolean;
	statusCode: string;
}

export interface RefundItem {
	amountMinor: string;
  descriptionEnUS: string;
  descriptionZhCN: string;
	impacts: RefundImpact[];
	invoiceItemNo: string;
	orderItemNo: string;
	quantity: number;
	refundableAmountMinor: string;
	refundItemNo: string;
}

export interface RefundDetail {
  items: RefundItem[];
  refund: RefundSummary;
}

export interface RefundReviewIntent {
  commentDigest: string;
  decisionCode: "approve" | "reject";
  evidenceDigest: string;
  expectedVersion: number;
  requestNo: string;
}

function path(pathName = "") {
  return pluginApiPath(pluginID, `admin/billing/refunds${pathName}`);
}

export function listRefunds(
  params: Record<string, number | string | undefined>,
) {
  return requestPreservingHttpStatus<{
    total: number;
    nextCursor: string;
    refunds: RefundSummary[];
  }>(requestClient, path(), { method: "GET", params });
}

export async function getRefundDetail(refundNo: string) {
  const response = await requestClient.get<{ detail: RefundDetail }>(
    path(`/${encodeURIComponent(refundNo)}`),
  );
  return response.detail;
}

export async function reviewRefund(
  refund: RefundSummary,
  intent: RefundReviewIntent,
) {
  const response = await requestClient.post<{ refund: RefundSummary }>(
    path(`/${encodeURIComponent(refund.refundNo)}/reviews`), intent,
  );
  return response.refund;
}
