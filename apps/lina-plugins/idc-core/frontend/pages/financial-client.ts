import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./commerce-request";

const pluginID = "idc-core";

export interface FinancialReceivable {
  currencyCode: string;
  outstandingMinor: string;
}

export interface FinancialOverview {
  openInvoiceCount: number;
  overdueInvoiceCount: number;
  pendingAdjustmentCount: number;
  receivables: FinancialReceivable[];
}

export interface FinancialInvoice {
  createdAt: number;
  currencyCode: string;
  customerNo: string;
  customerCode: string;
  customerDisplayName: string;
  customerPrimaryPhone: string;
  invoiceNo: string;
  itemCount: number;
  orderNo?: string;
  outstandingAmountMinor: string;
  paidAmountMinor: string;
  paymentDueAt: number;
  statusCode: string;
  totalAmountMinor: string;
  waivedAmountMinor: string;
}

export interface FinancialInvoiceItem {
  descriptionEnUS: string;
  descriptionZhCN: string;
  invoiceItemNo: string;
  lineNumber: number;
  totalAmountMinor: string;
}

export interface FinancialInvoiceDetail {
  invoice: FinancialInvoice;
  items: FinancialInvoiceItem[];
  nextItemCursor: string;
}

export interface FinancialTransaction {
  amountMinor: string;
  currencyCode: string;
  customerNo?: string;
  customerCode?: string;
  customerDisplayName?: string;
  customerPrimaryPhone?: string;
  entryCount: number;
  postedAt: number;
  relatedInvoiceNo?: string;
  transactionNo: string;
  transactionTypeCode: string;
}

export interface FinancialTransactionEntry {
  accountNo: string;
  accountTypeCode: string;
  amountMinor: string;
  directionCode: string;
  entryNo: string;
  entryRoleCode: string;
  lineNumber: number;
  ownerReference: string;
  ownerTypeCode: string;
  reasonCode: string;
}

export interface FinancialTransactionDetail {
  amountMinor: string;
  currencyCode: string;
  entries: FinancialTransactionEntry[];
  entryCount: number;
  nextEntryCursor: string;
  originalTransactionNo?: string;
  postedAt: number;
  postedBy: number;
  sourceReference: string;
  sourceTypeCode: string;
  transactionNo: string;
  transactionTypeCode: string;
}

export interface FinancialAdjustment {
  adjustmentNo: string;
  adjustmentTypeCode: string;
  amountMinor: string;
  approvalRequired: boolean;
  createdAt: number;
  currencyCode: string;
  customerNo: string;
  customerCode: string;
  customerDisplayName: string;
  customerPrimaryPhone: string;
  invoiceNo?: string;
  ledgerTransactionNo?: string;
  reasonCode: string;
  requestedBy: number;
  statusCode: string;
  updatedAt: number;
}

export interface FinancialAdjustmentDetail {
  adjustment: FinancialAdjustment;
  businessPurposeCode?: string;
  decidedAt?: number;
  decisionCode?: string;
  evidenceDigest: string;
  policyNo?: string;
  policyVersion?: number;
  postedAt?: number;
  proofReference?: string;
  receivingAccountReference?: string;
  reviewerId?: number;
  version: number;
}

export interface FinancialActionProof {
  expiresAt: number;
  proofToken: string;
}

export interface FinancialAdjustmentMutation {
  adjustmentNo: string;
  adjustmentTypeCode: string;
  amountMinor: string;
  approvalRequired: boolean;
  currencyCode: string;
  customerNo: string;
  invoiceNo?: string;
  ledgerTransactionNo?: string;
  reused: boolean;
  statusCode: string;
}

export interface FinancialManualReceiptIntent {
  amountMinor: string;
  businessPurposeCode: string;
  currencyCode?: string;
  customerNo?: string;
  evidenceDigest: string;
  invoiceNo?: string;
  proofReference: string;
  reasonCode: string;
  receivingAccountReference: string;
  requestNo: string;
}

export interface FinancialInvoiceWaiverIntent {
  amountMinor: string;
  evidenceDigest: string;
  invoiceNo: string;
  reasonCode: string;
  requestNo: string;
}

export interface FinancialAdjustmentReviewIntent {
  commentDigest: string;
  decisionCode: "approved" | "rejected";
  evidenceDigest: string;
  requestNo: string;
}

export interface FinancialAdjustmentPolicyIntent {
  currencyCode: string;
  requestNo: string;
  reviewThresholdMinor: string;
}

export interface FinancialDeliveryPreview {
  invoiceNo: string;
  invoiceVersion: number;
  maskedEmail: string;
}

export interface FinancialInternalNote {
  content: string;
  correctsNoteNo?: string;
  createdAt: number;
  createdBy: number;
  noteNo: string;
  targetNo: string;
  targetType: "invoice" | "order" | "service";
}

export interface FinancialSavedFilter {
  displayName: string;
  filter: Record<string, unknown>;
  filterNo: string;
  updatedAt: number;
  version: number;
  viewCode: "adjustment" | "invoice" | "receivable" | "transaction";
}

export interface FinancialExportStatus {
  expiresAt?: number;
  exportType: "adjustment" | "invoice" | "receivable" | "transaction";
  failureCode?: string;
  generatedAt?: number;
  requestNo: string;
  requestedAt: number;
  reused: boolean;
  rowCount: number;
  status: string;
}

export async function financialOverview() {
  const response = await requestClient.get<{ overview: FinancialOverview }>(
    pluginApiPath(pluginID, "admin/billing/overview"),
  );
  return response.overview;
}

export function financialInvoices(
  params: Record<string, number | string | undefined>,
) {
  return requestPreservingHttpStatus<{
    total: number;
    invoices: FinancialInvoice[];
    nextCursor: string;
  }>(requestClient, pluginApiPath(pluginID, "admin/billing/invoices"), {
    method: "GET",
    params,
  });
}

export async function financialInvoiceDetail(invoiceNo: string) {
  const response = await requestClient.get<{ detail: FinancialInvoiceDetail }>(
    pluginApiPath(
      pluginID,
      `admin/billing/invoices/${encodeURIComponent(invoiceNo)}`,
    ),
    { params: { itemLimit: 100 } },
  );
  return response.detail;
}

export function financialTransactions(
  params: Record<string, number | string | undefined>,
) {
  return requestPreservingHttpStatus<{
    total: number;
    nextCursor: string;
    transactions: FinancialTransaction[];
  }>(requestClient, pluginApiPath(pluginID, "admin/billing/transactions"), {
    method: "GET",
    params,
  });
}

export async function financialTransactionDetail(transactionNo: string) {
  const response = await requestClient.get<{
    detail: FinancialTransactionDetail;
  }>(
    pluginApiPath(
      pluginID,
      `admin/billing/transactions/${encodeURIComponent(transactionNo)}`,
    ),
    { params: { entryLimit: 100 } },
  );
  return response.detail;
}

export function financialAdjustments(
  params: Record<string, number | string | undefined>,
) {
  return requestPreservingHttpStatus<{
    total: number;
    adjustments: FinancialAdjustment[];
    nextCursor: string;
  }>(requestClient, pluginApiPath(pluginID, "admin/billing/adjustments"), {
    method: "GET",
    params,
  });
}

export async function financialAdjustmentDetail(adjustmentNo: string) {
  const response = await requestClient.get<{
    detail: FinancialAdjustmentDetail;
  }>(
    pluginApiPath(
      pluginID,
      `admin/billing/adjustments/${encodeURIComponent(adjustmentNo)}`,
    ),
  );
  return response.detail;
}

async function issueProof(path: string, intent: object) {
  const response = await requestClient.post<{ proof: FinancialActionProof }>(
    pluginApiPath(pluginID, path),
    intent,
  );
  return response.proof;
}

async function executeAdjustment(
  path: string,
  intent: object,
  proofToken?: string,
) {
  const response = await requestClient.post<{
    adjustment: FinancialAdjustmentMutation;
  }>(pluginApiPath(pluginID, path), { ...intent, proofToken });
  return response.adjustment;
}

export async function createFinancialManualReceipt(
  intent: FinancialManualReceiptIntent,
) {
  return executeAdjustment("admin/billing/manual-receipts", intent);
}

export async function createFinancialInvoiceWaiver(
  intent: FinancialInvoiceWaiverIntent,
) {
  return executeAdjustment("admin/billing/invoice-waivers", intent);
}

export async function reviewFinancialAdjustment(
  adjustmentNo: string,
  intent: FinancialAdjustmentReviewIntent,
) {
  const target = encodeURIComponent(adjustmentNo);
  return executeAdjustment(
    `admin/billing/adjustments/${target}/reviews`,
    intent,
  );
}

export async function configureFinancialAdjustmentPolicy(
  intent: FinancialAdjustmentPolicyIntent,
) {
  const proof = await issueProof(
    "admin/billing/adjustment-policies/proofs",
    intent,
  );
  const response = await requestClient.post<{
    policy: {
      currencyCode: string;
      policyNo: string;
      reused: boolean;
      reviewThresholdMinor: string;
      version: number;
    };
  }>(pluginApiPath(pluginID, "admin/billing/adjustment-policies"), {
    ...intent,
    proofToken: proof.proofToken,
  });
  return response.policy;
}

export async function financialInvoiceDeliveryPreview(invoiceNo: string) {
  const response = await requestClient.get<{
    preview: FinancialDeliveryPreview;
  }>(
    pluginApiPath(
      pluginID,
      `admin/billing/invoices/${encodeURIComponent(invoiceNo)}/delivery-preview`,
    ),
  );
  return response.preview;
}

export async function resendFinancialInvoice(
  invoiceNo: string,
  requestNo: string,
) {
  return requestClient.post<{ intentNo: string; reused: boolean }>(
    pluginApiPath(
      pluginID,
      `admin/billing/invoices/${encodeURIComponent(invoiceNo)}/resends`,
    ),
    { requestNo },
  );
}

export async function financialInternalNotes(
  targetType: string,
  targetNo: string,
) {
  const response = await requestClient.get<{ notes: FinancialInternalNote[] }>(
    pluginApiPath(pluginID, "admin/billing/internal-notes"),
    { params: { limit: 100, targetNo, targetType } },
  );
  return response.notes;
}

export async function appendFinancialInternalNote(input: {
  content: string;
  correctsNoteNo?: string;
  requestNo: string;
  targetNo: string;
  targetType: string;
}) {
  const response = await requestClient.post<{ note: FinancialInternalNote }>(
    pluginApiPath(pluginID, "admin/billing/internal-notes"),
    input,
  );
  return response.note;
}

export async function financialSavedFilters(viewCode: string) {
  const response = await requestClient.get<{
    filters: FinancialSavedFilter[];
  }>(pluginApiPath(pluginID, "admin/billing/saved-filters"), {
    params: { viewCode },
  });
  return response.filters;
}

export async function saveFinancialFilter(input: {
  displayName: string;
  filter: Record<string, unknown>;
  viewCode: string;
}) {
  const response = await requestClient.post<{ filter: FinancialSavedFilter }>(
    pluginApiPath(pluginID, "admin/billing/saved-filters"),
    input,
  );
  return response.filter;
}

export async function createFinancialExport(input: {
  exportType: string;
  filter: Record<string, unknown>;
  requestNo: string;
}) {
  const response = await requestClient.post<{ export: FinancialExportStatus }>(
    pluginApiPath(pluginID, "admin/billing/exports"),
    input,
  );
  return response.export;
}

export async function generateFinancialExport(requestNo: string) {
  const response = await requestClient.post<{ export: FinancialExportStatus }>(
    pluginApiPath(
      pluginID,
      `admin/billing/exports/${encodeURIComponent(requestNo)}/generation`,
    ),
  );
  return response.export;
}

export async function financialExportTicket(requestNo: string) {
  return requestClient.post<{ expiresAt: number; token: string }>(
    pluginApiPath(
      pluginID,
      `admin/billing/exports/${encodeURIComponent(requestNo)}/tickets`,
    ),
  );
}

export function financialExportDownloadPath(token: string) {
  return pluginApiPath(
    pluginID,
    `admin/billing/export-downloads/${encodeURIComponent(token)}`,
  );
}

export interface FinancialBalanceAdjustmentIntent {
 requestNo: string;
 customerNo: string;
 currencyCode: string;
 amountMinor: string;
 deduct: boolean;
 reasonCode: string;
 evidenceDigest: string;
}

export async function createFinancialBalanceAdjustment(intent: FinancialBalanceAdjustmentIntent) {
 return executeAdjustment('admin/billing/balance-adjustments', intent);
}
