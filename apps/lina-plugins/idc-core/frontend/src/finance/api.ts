import type { PluginHostMountContext } from "../host-context";

import { createPluginRequest, PluginRequestError } from "../runtime/request";

const base = "/x/idc-core/api/v1/admin/billing/credit";

export type CreditApplication = {
  applicationNo: string;
  createdAt: number;
  creditAccountNo?: string;
  currencyCode: string;
  customerNo: string;
  requestedLimitMinor: string;
  requestedValidFrom: number;
  requestedValidUntil: number;
  requiredApprovalCount: number;
  reviewCount: number;
  statusCode: string;
};

export type CreditAccount = {
  availableLimitMinor: string;
  billedPrincipalMinor: string;
  creditAccountNo: string;
  currencyCode: string;
  customerNo: string;
  delinquentPrincipalMinor: string;
  freezeReasonCode?: string;
  statusCode: string;
  totalLimitMinor: string;
  unbilledPrincipalMinor: string;
};

export type CreditStatement = {
  currencyCode: string;
  customerNo: string;
  outstandingAmountMinor: string;
  paymentDueAt: number;
  statementNo: string;
  statusCode: string;
  totalAmountMinor: string;
};

export type CreditEvidence = {
  fileName: string;
  mediaType: string;
  sizeBytes: number;
  uploadNo: string;
};

export function createFinanceAPI(currentContext: () => PluginHostMountContext) {
  const request = createPluginRequest(currentContext);
  return {
    applications(page = 1, limit = 10) {
      return request<{
        applications: CreditApplication[];
        nextCursor: string;
        total: number;
      }>(`${base}/applications?page=${page}&limit=${limit}`);
    },
    application(applicationNo: string) {
      return request<{
        detail: {
          application: CreditApplication;
          reviews: Array<{
            createdAt: number;
            decisionCode: string;
            reviewNo: string;
            reviewSequence: number;
          }>;
        };
        evidence: CreditEvidence[];
      }>(`${base}/applications/${encodeURIComponent(applicationNo)}`);
    },
    accounts(page = 1, limit = 10) {
      return request<{ accounts: CreditAccount[]; nextCursor: string; total: number }>(
        `${base}/accounts?page=${page}&limit=${limit}`,
      );
    },
    statements(page = 1, limit = 10) {
      return request<{ nextCursor: string; statements: CreditStatement[]; total: number }>(
        `${base}/statements?page=${page}&limit=${limit}`,
      );
    },
    async review(
      applicationNo: string,
      input: {
        commentDigest: string;
        decisionCode: "approved" | "rejected";
        evidenceDigest: string;
        requestNo: string;
      },
    ) {
      const path = `${base}/applications/${encodeURIComponent(applicationNo)}`;
      const proof = await request<{ proof: { proofToken: string } }>(
        `${path}/review-proofs`,
        { body: input, method: "POST" },
      );
      return request<{ application: CreditApplication }>(`${path}/reviews`, {
        body: { ...input, proofToken: proof.proof.proofToken },
        method: "POST",
      });
    },
    async evidence(applicationNo: string, item: CreditEvidence): Promise<Blob> {
      const context = currentContext();
      const response = await fetch(
        `${base}/applications/${encodeURIComponent(applicationNo)}/evidence/${encodeURIComponent(item.uploadNo)}`,
        {
          credentials: "same-origin",
          headers: context.accessToken
            ? { Authorization: `Bearer ${context.accessToken}` }
            : {},
        },
      );
      if (!response.ok) throw new PluginRequestError(response.status);
      return response.blob();
    },
  };
}
