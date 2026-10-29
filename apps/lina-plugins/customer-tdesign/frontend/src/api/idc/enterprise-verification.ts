import { pluginApiPath, requestClient } from "#/api/request";

const compliancePluginID = "idc-compliance";
const compliancePath = "account/compliance";

export type EnterpriseCaseStatus =
  | "canceled"
  | "draft"
  | "expired"
  | "pending"
  | "rejected"
  | "review_required"
  | "verified"
  | "verifying";

export interface EnterpriseMaterial {
  byteSize: number;
  materialNo: string;
  materialTypeCode: string;
  mediaType: string;
  sequenceNo: number;
  statusCode: string;
}

export interface EnterpriseCase {
  caseNo: string;
  decisionReasonCode: string;
  expiresAt: number;
  legalChangeRequestNo: string;
  materials: EnterpriseMaterial[];
  rejectedAt: number;
  reverification: boolean;
  statusCode: EnterpriseCaseStatus;
  submittedAt: number;
  updatedAt: number;
  verifiedAt: number;
  version: number;
}

export interface EnterpriseProfile {
  expiresAt: number;
  industryCode: string;
  legalName: string;
  profileVersion: number;
  registrationCountryCode: string;
  registrationIdentifierMasked: string;
  verificationCaseNo: string;
  verificationStatusCode: string;
  verifiedAt: number;
}

export interface EnterpriseLegalChange {
  currentProfileVersion: number;
  fieldCodes: string[];
  reasonCode: string;
  requestNo: string;
  statusCode: string;
  submittedAt: number;
  verificationCaseNo: string;
}

export interface EnterpriseState {
  activeLegalChange?: EnterpriseLegalChange | null;
  case?: EnterpriseCase | null;
  policyVersion: string;
  profile?: EnterpriseProfile | null;
}

export interface EnterpriseSubmissionInput {
  agentIdentityNumber: string;
  agentName: string;
  applicantRoleCode: "authorized_agent" | "legal_representative";
  idempotencyKey: string;
  industryCode: string;
  legalName: string;
  legalRepresentativeIdentityNumber: string;
  legalRepresentativeName: string;
  materialNos: string[];
  registrationCountryCode: "CN";
  registrationIdentifier: string;
}

function path(suffix: string) {
  return pluginApiPath(compliancePluginID, `${compliancePath}/${suffix}`);
}

export function getEnterpriseVerification() {
  return requestClient
    .get<{ item: EnterpriseState }>(path("enterprise-verification"))
    .then((result) => result.item);
}

export function createEnterpriseVerificationConsent(input: {
  idempotencyKey: string;
  locale: "en-US" | "zh-CN";
  policyVersion: string;
}) {
  return requestClient.post<{
    consentNo: string;
    occurredAt: number;
    policyVersion: string;
    reused: boolean;
  }>(path("enterprise-verification-consents"), {
    accepted: true,
    ...input,
  });
}

export function createEnterpriseVerificationDraft(input: {
  consentNo: string;
  idempotencyKey: string;
  legalChangeRequestNo: string;
}) {
  return requestClient
    .post<{
      item: { caseNo: string; statusCode: string };
    }>(path("enterprise-verifications"), input)
    .then((result) => result.item);
}

export function createEnterpriseLegalChange(input: {
  expectedProfileVersion: number;
  fieldCodes: string[];
  idempotencyKey: string;
  reasonCode: string;
}) {
  return requestClient
    .post<{
      item: EnterpriseLegalChange;
    }>(path("enterprise-legal-change-requests"), input)
    .then((result) => result.item);
}

export async function uploadEnterpriseMaterial(
  caseNo: string,
  materialTypeCode: string,
  file: File,
  idempotencyKey: string,
) {
  const reservation = await requestClient.post<{
    item: { materialNo: string };
  }>(
    path(`enterprise-verifications/${encodeURIComponent(caseNo)}/materials`),
    { idempotencyKey, materialTypeCode },
    { silentErrorMessage: true },
  );
  return requestClient
    .put<{ item: EnterpriseMaterial }>(
      path(
        `enterprise-verifications/${encodeURIComponent(caseNo)}/materials/${encodeURIComponent(reservation.item.materialNo)}/content`,
      ),
      file,
      {
        headers: {
          "Content-Type": file.type || "application/octet-stream",
        },
        silentErrorMessage: true,
        timeout: 120_000,
      },
    )
    .then((result) => result.item);
}

export function submitEnterpriseVerification(
  caseNo: string,
  input: EnterpriseSubmissionInput,
) {
  return requestClient
    .post<{
      item: {
        caseNo: string;
        reused: boolean;
        reverification: boolean;
        statusCode: string;
      };
    }>(
      path(`enterprise-verifications/${encodeURIComponent(caseNo)}/submission`),
      input,
      { silentErrorMessage: true },
    )
    .then((result) => result.item);
}
