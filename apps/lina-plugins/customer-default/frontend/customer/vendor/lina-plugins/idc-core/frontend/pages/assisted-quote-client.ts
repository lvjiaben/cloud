import { pluginApiPath, requestClient } from "#/api/request";
import type { PurchaseValue } from "./assisted-purchase-client";

const pluginID = "idc-core";

export interface AssistedQuoteCreateItem {
  publicSlug: string;
  configurationRevision: string;
  billingTermToken: string;
  quantity: number;
  purchaseCycles: number;
  values: PurchaseValue[];
}
export interface AssistedQuoteCreateInput {
  customerNo: string;
  currencyCode: string;
  manualDiscountMinor: string;
  paymentDueAt: number;
  contractPolicyCode: "inherit" | "none" | "required";
  contractTemplateNo: string;
  reason: string;
  internalNote: string;
  idempotencyKey: string;
  items: AssistedQuoteCreateItem[];
}
export async function createAssistedQuote(input: AssistedQuoteCreateInput) {
  const response = await requestClient.post<{ request: AssistedQuote }>(
    pluginApiPath(pluginID, "admin/commerce/assisted-quotes"), input,
  );
  return response.request;
}

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

export function assistedOrderOptions(customerNo: string) {
  return requestClient.get<{ currencies: string[] }>(pluginApiPath(pluginID, `admin/customers/${encodeURIComponent(customerNo)}/assisted-order-options`));
}
