import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./commerce-request";

const pluginID = "idc-core";

export type AssistedQuoteStatus =
  | "approved"
  | "ordered"
  | "pending_approval"
  | "rejected";

export type AssistedPaymentTerm = "extended_prepay" | "standard_prepay";

export interface AssistedQuoteItem {
  lineNumber: number;
  productNameEnUS: string;
  productNameZhCN: string;
  purchaseCycles: number;
  requestItemNo: string;
  quantity: number;
}

export interface AssistedQuote {
  canApprove: boolean;
  canCheckout: boolean;
  canReject: boolean;
  createdAt: number;
  currencyCode: string;
  contractPolicyCode: "inherit" | "none" | "required";
  contractTemplateNo: string;
  customerNo: string;
  customerCode: string;
  customerDisplayName: string;
  customerPrimaryPhone: string;
  decisionAt?: number;
  decisionBy?: number;
  decisionCode?: string;
  decisionReason?: string;
  discountMinor: string;
  expiresAt: number;
  internalNote: string;
  items: AssistedQuoteItem[];
  manualDiscountMinor: string;
  orderNo?: string;
  orderStatusCode?: string;
  paymentDueSeconds: number;
  paymentTermCode: AssistedPaymentTerm;
  quoteNo?: string;
  reason: string;
  requestNo: string;
  requestedBy: number;
  reused: boolean;
  statusCode: AssistedQuoteStatus;
  subtotalMinor: string;
  totalAmountMinor: string;
}

export interface AssistedQuotePage {
  total: number;
  nextCursor: string;
  requests: AssistedQuote[];
}

export function assistedQuotes(params: {
  page?: number;
  cursor?: string;
  customerNo?: string;
  limit?: number;
  statusCode?: string;
}) {
  return requestPreservingHttpStatus<AssistedQuotePage>(requestClient,
    pluginApiPath(pluginID, "admin/commerce/assisted-quotes"),
    { method: "GET", params },
  );
}

export async function assistedQuoteDetail(requestNo: string) {
  const response = await requestClient.get<{ request: AssistedQuote }>(
    pluginApiPath(
      pluginID,
      `admin/commerce/assisted-quotes/${encodeURIComponent(requestNo)}`,
    ),
  );
  return response.request;
}

export async function decideAssistedQuote(
  requestNo: string,
  decision: "approval" | "rejection",
  input: {
    decisionReason: string;
    idempotencyKey: string;
  },
) {
  const response = await requestClient.put<{ request: AssistedQuote }>(
    pluginApiPath(
      pluginID,
      `admin/commerce/assisted-quotes/${encodeURIComponent(requestNo)}/${decision}`,
    ),
    input,
  );
  return response.request;
}

export async function checkoutAssistedQuote(requestNo: string) {
  const response = await requestClient.post<{ request: AssistedQuote }>(
    pluginApiPath(
      pluginID,
      `admin/commerce/assisted-quotes/${encodeURIComponent(requestNo)}/checkout`,
    ),
    { idempotencyKey: crypto.randomUUID() },
  );
  return response.request;
}
