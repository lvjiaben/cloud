import { requestPreservingHttpStatus } from "./compliance-request";
import type { RequestClientConfig } from "@vben/request";
import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-compliance";
const invoicePath = "admin/compliance/tax-invoices";
const redLetterPath = "admin/compliance/red-letter-tasks";

export interface TaxInvoice {
  taxInvoiceNo: string; customerNo: string; statusCode: string; invoiceTypeCode: string;
  providerCode: string; decisionReasonCode: string; decisionInternalNote: string;
  grossAmountMinor: number; netAmountMinor: number; taxAmountMinor: number; currencyCode: string;
  maskedInvoiceNumber: string; version: number; submittedAt: number; reviewedAt?: number;
  issuedAt: number; deliveredAt: number; canceledAt: number; providerNotIssuedAt: number;
  capabilityReady: boolean; capabilityReasonCode: string; supportedInvoiceTypes: string[];
}
export interface TaxInvoiceAllocation { allocationNo: string; billingInvoiceNo: string; orderNo: string; grossAmountMinor: number; netAmountMinor: number; taxAmountMinor: number; refundedGrossMinor: number; redLetteredGrossMinor: number }
export interface TaxInvoiceBuyerProfile { profileNo: string; titleTypeCode: string; invoiceTitle: string; taxRegistrationNo: string; registeredAddress: string; registeredPhone: string; bankName: string; bankAccountMasked: string; version: number }
export interface TaxInvoiceLine { itemNo: string; allocationNo: string; billingInvoiceItemNo: string; lineNo: number; taxClassificationCode: string; taxRatePpm: number; grossAmountMinor: number; netAmountMinor: number; taxAmountMinor: number }
export interface TaxInvoiceFile { fileNo: string; fileTypeCode: string; contentSha256: string; mediaType: string; byteSize: number; statusCode: string; immutable: boolean; archivedAt: number; createdAt: number }
export interface TaxInvoiceDetail { item: TaxInvoice; buyer: TaxInvoiceBuyerProfile; allocations: TaxInvoiceAllocation[]; items: TaxInvoiceLine[]; files: TaxInvoiceFile[]; historyTruncated: boolean }

export interface RedLetterTask {
  taskNo: string; refundOperationNo: string; taxInvoiceNo: string; customerNo: string; currencyCode: string;
  statusCode: string; manualReasonCode: string; allocationResolved: boolean; customerConfirmationRequired: boolean;
  grossAmountMinor: number; netAmountMinor: number; taxAmountMinor: number; version: number;
  createdAt: number; completedAt: number; candidateCount: number; allocationCount: number; completionEvidenceNo: string;
}
export interface RedLetterCandidate { refundAllocationNo: string; taxInvoiceNo: string; taxInvoiceAllocationNo: string; taxInvoiceItemNo: string; requestedGrossMinor: number; availableGrossMinor: number; availableNetMinor: number; availableTaxMinor: number }
export interface RedLetterAllocation { refundAllocationNo: string; taxInvoiceNo: string; taxInvoiceAllocationNo: string; taxInvoiceItemNo: string; grossAmountMinor: number; netAmountMinor: number; taxAmountMinor: number }
export interface RedLetterEvidence { evidenceNo: string; evidenceReference: string; evidenceSha256: string; mediaType: string; byteSize: number; occurredAt: number }
export interface RedLetterDetail { item: RedLetterTask; candidates: RedLetterCandidate[]; allocations: RedLetterAllocation[]; evidence?: RedLetterEvidence; historyTruncated: boolean }

export function taxInvoiceList(params: { page?: number; cursor?: string; customerNo?: string; statusCode?: string; pageSize?: number }) { return readPage<{items: TaxInvoice[]; total: number; nextCursor: string}>(pluginApiPath(pluginID, invoicePath), { params }); }
export function taxInvoiceGet(no: string) { return requestClient.get<TaxInvoiceDetail>(pluginApiPath(pluginID, `${invoicePath}/${no}`)); }
export function taxInvoiceApprove(no: string, invoiceTypeCode: string, expectedVersion: number) { return requestClient.post<{result: {item: TaxInvoice; applied: boolean; reused: boolean}}>(pluginApiPath(pluginID, `${invoicePath}/${no}/approvals`), { invoiceTypeCode, expectedVersion, idempotencyKey: `invoice-approve-${crypto.randomUUID()}` }); }
export function taxInvoiceReject(no: string, reasonCode: string, internalNote: string, expectedVersion: number) { return requestClient.post<{result: {item: TaxInvoice; applied: boolean; reused: boolean}}>(pluginApiPath(pluginID, `${invoicePath}/${no}/rejections`), { reasonCode, internalNote, expectedVersion, idempotencyKey: `invoice-reject-${crypto.randomUUID()}` }); }
export function taxInvoiceSubmit(no: string, expectedVersion: number) { return requestClient.post<{result: {item: TaxInvoice; applied: boolean; reused: boolean}}>(pluginApiPath(pluginID, `${invoicePath}/${no}/submissions`), { expectedVersion, idempotencyKey: `invoice-submit-${crypto.randomUUID()}` }); }
export function taxInvoiceCancel(no: string, reasonCode: string, internalNote: string, expectedVersion: number) { return requestClient.post<{result: {item: TaxInvoice; applied: boolean; reused: boolean}}>(pluginApiPath(pluginID, `${invoicePath}/${no}/cancellations`), { reasonCode, internalNote, expectedVersion, idempotencyKey: `invoice-cancel-${crypto.randomUUID()}` }); }

export function redLetterList(params: { page?: number; cursor?: string; customerNo?: string; statusCode?: string; pageSize?: number }) { return readPage<{items: RedLetterTask[]; total: number; nextCursor: string}>(pluginApiPath(pluginID, redLetterPath), { params }); }
export function redLetterGet(no: string) { return requestClient.get<RedLetterDetail>(pluginApiPath(pluginID, `${redLetterPath}/${no}`)); }
export function redLetterResolve(no: string, expectedVersion: number, selections: Array<{refundAllocationNo: string; taxInvoiceAllocationNo: string; grossAmountMinor: number}>) { return requestClient.post<{item: RedLetterTask}>(pluginApiPath(pluginID, `${redLetterPath}/${no}/allocation-resolutions`), { expectedVersion, selections, idempotencyKey: `red-letter-allocation-${crypto.randomUUID()}` }); }
export function redLetterComplete(no: string, input: { expectedVersion: number; evidenceReference: string; evidenceSha256: string; occurredAt: number }) { return requestClient.post<{item: RedLetterTask}>(pluginApiPath(pluginID, `${redLetterPath}/${no}/manual-completions`), { ...input, idempotencyKey: `red-letter-completion-${crypto.randomUUID()}` }); }
function readPage<T>(url: string, config: RequestClientConfig = {}) { return requestPreservingHttpStatus<T>(requestClient,url,{...config,method:"GET"}); }
