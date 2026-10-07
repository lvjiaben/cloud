import { requestPreservingHttpStatus } from "./compliance-request";
import type { RequestClientConfig } from "@vben/request";
import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-compliance";
const basePath = "admin/compliance/enterprise-verifications";

export type EnterpriseCaseStatus =
  | "pending"
  | "verifying"
  | "review_required"
  | "verified"
  | "rejected"
  | "expired"
  | "canceled";

export interface EnterpriseCaseItem {
  applicantRoleCode: "authorized_agent" | "legal_representative";
  caseNo: string;
  customerNo: string;
  decisionReasonCode: string;
  maskedLegalName: string;
  maskedRegistrationId: string;
  reverification: boolean;
  statusCode: EnterpriseCaseStatus;
  submittedAt: number;
  updatedAt: number;
  version: number;
}

export interface EnterpriseMaskedLegalProfile {
  agentIdentityNumber: string;
  agentName: string;
  agentPresent: boolean;
  applicantRoleCode: string;
  industryCode: string;
  legalName: string;
  legalRepresentativeId: string;
  legalRepresentativeName: string;
  registrationCountryCode: string;
  registrationIdentifier: string;
}

export interface EnterpriseMaterialItem {
  byteSize: number;
  legalHold: boolean;
  materialNo: string;
  materialTypeCode: string;
  mediaType: string;
  retentionUntil: null | number;
  sequenceNo: number;
  statusCode: string;
}

export interface EnterpriseCaseDetail extends EnterpriseCaseItem {
  legalProfile: EnterpriseMaskedLegalProfile;
  materials: EnterpriseMaterialItem[];
}

export interface EnterpriseReviewResult {
  caseNo: string;
  decisionReasonCode: string;
  reused: boolean;
  statusCode: EnterpriseCaseStatus;
  version: number;
}

export function enterpriseCaseList(params: {
  page?: number; cursor?: string;
  customerNo?: string;
  pageSize?: number;
  statusCode?: string;
}) {
  return readPage<{
    items: EnterpriseCaseItem[];
    total: number; nextCursor: string;
  }>(pluginApiPath(pluginID, basePath), { params });
}

export function enterpriseCaseGet(caseNo: string) {
  return requestClient.get<{ item: EnterpriseCaseDetail }>(
    pluginApiPath(pluginID, `${basePath}/${encodeURIComponent(caseNo)}`),
  );
}

function reviewCommand(
  caseNo: string,
  operation: "approvals" | "rejections" | "review-starts",
  input: Record<string, unknown>,
) {
  return requestClient.post<{ item: EnterpriseReviewResult }>(
    pluginApiPath(
      pluginID,
      `${basePath}/${encodeURIComponent(caseNo)}/${operation}`,
    ),
    input,
  );
}

export function enterpriseReviewStart(caseNo: string, version: number) {
  return reviewCommand(caseNo, "review-starts", {
    expectedVersion: version,
    idempotencyKey: crypto.randomUUID(),
  });
}

export function enterpriseApprove(caseNo: string, version: number) {
  return reviewCommand(caseNo, "approvals", {
    expectedVersion: version,
    idempotencyKey: crypto.randomUUID(),
  });
}

export function enterpriseReject(
  caseNo: string,
  version: number,
  reasonCode: string,
) {
  return reviewCommand(caseNo, "rejections", {
    expectedVersion: version,
    idempotencyKey: crypto.randomUUID(),
    reasonCode,
  });
}

export async function enterpriseMaterialDownload(
  caseNo: string,
  materialNo: string,
  mediaType: string,
) {
  const root = `${basePath}/${encodeURIComponent(caseNo)}/materials/${encodeURIComponent(materialNo)}`;
  const issued = await requestClient.post<{
    accessContext: string;
    expiresAt: number;
  }>(pluginApiPath(pluginID, `${root}/access-contexts`));
  const blob = await requestClient.download<Blob>(
    pluginApiPath(pluginID, `${root}/content`),
    {
      data: { accessContext: issued.accessContext },
      method: "POST",
      timeout: 30_000,
    },
  );
  const extension =
    mediaType === "application/pdf"
      ? "pdf"
      : mediaType === "image/jpeg"
        ? "jpg"
        : "png";
  const url = window.URL.createObjectURL(blob);
  try {
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `enterprise-material.${extension}`;
    anchor.click();
  } finally {
    window.URL.revokeObjectURL(url);
  }
}
function readPage<T>(url: string, config: RequestClientConfig = {}) { return requestPreservingHttpStatus<T>(requestClient,url,{...config,method:"GET"}); }
