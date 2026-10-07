import { pluginApiPath, requestClient } from "#/api/request";

const compliancePluginID = "idc-compliance";

export interface ElectronicContractTemplateCandidate {
  languageCode: "en-US" | "zh-CN";
  scopeCode: "all_customers" | "enterprise" | "personal";
  signingOrderCode: "customer_first" | "merchant_first" | "parallel";
  templateName: string;
  templateNo: string;
  templateVersion: number;
}

export interface ElectronicContractPreviewLine {
  amountMinor: string;
  orderItemNo: string;
  periodCount: number;
  periodUnitCode: string;
  purchaseCycles: number;
  quantity: number;
  serviceName: string;
}

export interface ElectronicContractPreview {
  contractNo: string;
  createdAt: number;
  currencyCode: string;
  customerName: string;
  customerType: "enterprise" | "individual";
  languageCode: "en-US" | "zh-CN";
  lines: ElectronicContractPreviewLine[];
  merchantName: string;
  orderNo: string;
  requiredForFulfillment: boolean;
  reused: boolean;
  signingDate: string;
  signingDateNoticeCode: string;
  signingExpiresAt: number;
  signingOrderCode: "customer_first" | "merchant_first" | "parallel";
  signerName: string;
  signerNo: string;
  signerPhoneMasked: string;
  statusCode: "pending_launch";
  templateName: string;
  templateNo: string;
  templateVersion: number;
  totalAmountMinor: string;
  version: number;
}

export interface ElectronicContractSignerCandidate {
  authorizationVersion: number;
  identityVerificationVersion: number;
  identityVerifiedAt: number;
  maskedName: string;
  maskedPhone: string;
  phoneVerificationVersion: number;
  phoneVerifiedAt: number;
  signerNo: string;
  version: number;
}

export interface ElectronicContractSigningSubmission {
  contractNo: string;
  flowNo: string;
  reused: boolean;
  statusCode: "result_unknown" | "signing";
  version: number;
}

export async function electronicContractTemplateCandidates(orderNo: string) {
  return requestClient.get<{ items: ElectronicContractTemplateCandidate[] }>(
    pluginApiPath(
      compliancePluginID,
      "admin/compliance/electronic-contracts/template-candidates",
    ),
    { params: { orderNo } },
  );
}

export async function createOptionalElectronicContract(input: {
  idempotencyKey: string;
  orderNo: string;
  signerNo?: string;
  templateNo: string;
}) {
  return requestClient.post<{ item: ElectronicContractPreview }>(
    pluginApiPath(
      compliancePluginID,
      "admin/compliance/electronic-contracts",
    ),
    input,
  );
}

export async function electronicContractSignerCandidates(customerNo: string) {
  return requestClient.get<{ items: ElectronicContractSignerCandidate[] }>(
    pluginApiPath(
      compliancePluginID,
      `admin/compliance/customers/${encodeURIComponent(customerNo)}/contract-signer-candidates`,
    ),
  );
}

export async function submitElectronicContractSigning(input: {
  contractNo: string;
  expectedVersion: number;
  idempotencyKey: string;
}) {
  return requestClient.post<{ item: ElectronicContractSigningSubmission }>(
    pluginApiPath(
      compliancePluginID,
      `admin/compliance/electronic-contracts/${encodeURIComponent(input.contractNo)}/signing-submissions`,
    ),
    {
      expectedVersion: input.expectedVersion,
      idempotencyKey: input.idempotencyKey,
    },
  );
}
