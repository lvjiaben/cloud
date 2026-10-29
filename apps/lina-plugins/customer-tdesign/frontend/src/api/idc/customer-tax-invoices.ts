import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-compliance";

export type CustomerTaxInvoiceStatus =
  | "approved_pending_submission"
  | "canceled"
  | "delivered"
  | "draft"
  | "failed"
  | "issued"
  | "pending_review"
  | "queued"
  | "red_letter_pending"
  | "red_lettered"
  | "rejected"
  | "result_unknown"
  | "submitting";

export interface CustomerTaxInvoiceProfile {
  bankAccountMasked?: string;
  bankName?: string;
  invoiceTitle: string;
  isDefault: boolean;
  profileNo: string;
  registeredAddress?: string;
  registeredPhone?: string;
  taxRegistrationNo?: string;
  titleTypeCode: "enterprise" | "individual";
  version: number;
}

export interface CustomerTaxInvoiceSource {
  alreadyAllocatedMinor: string;
  billingInvoiceNo: string;
  currencyCode: string;
  orderNo: string;
  remainingGrossMinor: string;
  settledAt: number;
  sourceGrossMinor: string;
}

export interface CustomerTaxInvoiceItem {
  currencyCode: string;
  decisionReasonCode?: string;
  deliveredAt?: number;
  grossAmountMinor: string;
  invoiceTypeCode: string;
  issuedAt?: number;
  maskedInvoiceNo?: string;
  statusCode: CustomerTaxInvoiceStatus;
  submittedAt: number;
  taxInvoiceNo: string;
  version: number;
}

export interface CustomerTaxInvoiceOptions {
  invoices: CustomerTaxInvoiceSource[];
  observedAt: number;
  profiles: CustomerTaxInvoiceProfile[];
}

export interface CustomerTaxInvoicePage {
  items: CustomerTaxInvoiceItem[];
  nextCursor: string;
}

export interface CustomerTaxInvoiceCreateInput {
  idempotencyKey: string;
  profileNo: string;
  selections: Array<{
    billingInvoiceNo: string;
    requestedGrossMinor: string;
  }>;
}

function path(pathName: string) {
  return pluginApiPath(pluginID, pathName);
}

export function getCustomerTaxInvoiceOptions() {
  return requestClient.get<CustomerTaxInvoiceOptions>(
    path("account/compliance/tax-invoice-options"),
  );
}

export function createCustomerTaxInvoice(input: CustomerTaxInvoiceCreateInput) {
  return requestClient.post<{
    item: CustomerTaxInvoiceItem;
    reused: boolean;
  }>(path("account/compliance/tax-invoices"), input);
}

export function listCustomerTaxInvoices(
  params: {
    cursor?: string;
    limit?: number;
  } = {},
) {
  return requestClient.get<CustomerTaxInvoicePage>(
    path("account/compliance/tax-invoices"),
    { params },
  );
}
