import { pluginApiPath, requestClient } from "#/api/request";
import { requestWithHttpStatus } from "./request-with-http-status";

const base = (suffix: string) =>
  pluginApiPath("idc-core", `account/billing/credit/${suffix}`);

export interface CreditAccount {
  creditAccountNo: string;
  currencyCode: string;
  statusCode: string;
  totalLimitMinor: string;
  availableLimitMinor: string;
  unbilledPrincipalMinor: string;
  billedPrincipalMinor: string;
  delinquentPrincipalMinor: string;
  validFrom: number;
  validUntil: number;
  freezeReasonCode?: string;
}
export interface CreditApplication {
  applicationNo: string;
  currencyCode: string;
  requestedLimitMinor: string;
  requestedValidFrom: number;
  requestedValidUntil: number;
  requiredApprovalCount: number;
  reviewCount: number;
  statusCode: string;
  createdAt: number;
}
export interface CreditStatement {
  statementNo: string;
  currencyCode: string;
  totalAmountMinor: string;
  paidAmountMinor: string;
  outstandingAmountMinor: string;
  statusCode: string;
  paymentDueAt: number;
  availableBalanceMinor: string;
  canRepayFromBalance: boolean;
}

export function getCreditOverview() {
  return requestWithHttpStatus<{
    overview: {
      accounts: CreditAccount[];
      pendingApplicationCount: number;
      openStatementCount: number;
      overdueStatementCount: number;
    };
  }>(requestClient, base("overview"), { method: "GET" });
}
export function listCreditApplications(page = 1, limit = 10) {
  return requestWithHttpStatus<{
    applications: CreditApplication[];
    nextCursor: string;
    total: number;
  }>(requestClient, base("applications"), {
    method: "GET",
    params: { page, limit },
  });
}
export function listCreditStatements(page = 1, limit = 10) {
  return requestWithHttpStatus<{
    statements: CreditStatement[];
    nextCursor: string;
    total: number;
  }>(requestClient, base("statements"), {
    method: "GET",
    params: { page, limit },
  });
}
export function reserveCreditEvidence(file: File) {
  return requestWithHttpStatus<{ uploadNo: string; expiresAt: number }>(
    requestClient,
    base("evidence-uploads"),
    { method: "POST", data: { fileName: file.name, sizeBytes: file.size } },
  );
}
export function uploadCreditEvidence(uploadNo: string, file: File) {
  return requestWithHttpStatus(
    requestClient,
    base(`evidence-uploads/${encodeURIComponent(uploadNo)}/content`),
    {
      method: "PUT",
      data: file,
      headers: { "Content-Type": file.type || "application/octet-stream" },
    },
  );
}
export function submitCreditApplication(input: {
  currencyCode: string;
  requestedLimitMinor: string;
  requestedValidFrom: number;
  requestedValidUntil: number;
  uploadNos: string[];
  idempotencyKey: string;
}) {
  return requestWithHttpStatus<{ application: CreditApplication }>(
    requestClient,
    base("applications"),
    { method: "POST", data: input },
  );
}
export function issueBalanceRepaymentProof(
  statementNo: string,
  input: { amountMinor: string; requestNo: string },
) {
  return requestWithHttpStatus<{ proof: { proofToken: string } }>(
    requestClient,
    base(
      `statements/${encodeURIComponent(statementNo)}/balance-repayment-proofs`,
    ),
    { method: "POST", data: input },
  );
}
export function repayStatementFromBalance(
  statementNo: string,
  input: { amountMinor: string; requestNo: string; proofToken: string },
) {
  return requestWithHttpStatus(
    requestClient,
    base(`statements/${encodeURIComponent(statementNo)}/balance-repayments`),
    { method: "POST", data: input },
  );
}
export function settleInvoiceFromCredit(invoiceNo: string) {
  return requestClient.post<{
    settlement: {
      availableLimitMinor: string;
      creditAmountMinor: string;
      statusCode: string;
    };
  }>(
    pluginApiPath(
      "idc-core",
      `account/billing/invoices/${encodeURIComponent(invoiceNo)}/credit-settlements`,
    ),
  );
}
