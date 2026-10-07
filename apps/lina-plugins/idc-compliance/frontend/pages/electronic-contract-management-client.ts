import { requestPreservingHttpStatus } from "./compliance-request";
import type { RequestClientConfig } from "@vben/request";
import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-compliance";
const contractsPath = "admin/compliance/electronic-contracts";

export type ContractStatus =
  | "completed"
  | "draft"
  | "expired"
  | "failed"
  | "pending_launch"
  | "reissuing"
  | "rejected"
  | "revoked"
  | "signing"
  | "terminated";

export interface ManagementContract {
  amountMinor: string;
  completedAt: number;
  contractNo: string;
  contractTypeCode: string;
  createdAt: number;
  currencyCode: string;
  customerNo: string;
  languageCode: "en-US" | "zh-CN";
  launchedAt: number;
  orderNo: string;
  requiredForFulfillment: boolean;
  signingExpiresAt: number;
  signingOrderCode: string;
  statusCode: ContractStatus;
  subjectTypeCode: "enterprise" | "individual";
  templateName: string;
  templateNo: string;
  templateVersion: number;
  terminatedAt: number;
  updatedAt: number;
  version: number;
}

export interface ContractFlow {
  completedAt: number;
  createdAt: number;
  expiresAt: number;
  failureCode: string;
  flowNo: string;
  providerStatus: string;
  purposeCode: string;
  sequenceNo: number;
  statusCode: string;
  submittedAt: number;
}

export interface ContractEvent {
  createdAt: number;
  decisionCode: string;
  flowNo: string;
  observedAt: number;
  projectedStatus: string;
  providerStatus: string;
  reasonCode: string;
  sourceKind: string;
  supplierObservedAt: number;
}

export interface ContractFile {
  archivedAt: number;
  byteSize: number;
  contentSha256: string;
  createdAt: number;
  fileNo: string;
  fileTypeCode: string;
  flowNo: string;
  immutable: boolean;
  legalHold: boolean;
  mediaType: string;
  retentionUntil: number;
  statusCode: string;
  supplierObservedAt: number;
}

export interface ArchiveJob {
  artifactTypeCode: string;
  attemptCount: number;
  completedAt: number;
  createdAt: number;
  flowNo: string;
  jobNo: string;
  lastReasonCode: string;
  nextAttemptAt: number;
  statusCode: string;
  supplierObservedAt: number;
  updatedAt: number;
}

export interface ContractReplacement {
  createdAt: number;
  newContractNo: string;
  oldContractNo: string;
  replacementNo: string;
}

export interface ManagementContractDetail {
  archiveJobs: ArchiveJob[];
  events: ContractEvent[];
  files: ContractFile[];
  flows: ContractFlow[];
  historyTruncated: boolean;
  item: ManagementContract;
  replacements: ContractReplacement[];
}

export interface ContractTemplateCandidate {
  languageCode: "en-US" | "zh-CN";
  scopeCode: string;
  signingOrderCode: string;
  templateName: string;
  templateNo: string;
  templateVersion: number;
}

export interface ContractTemplateCandidateResult {
  customerNo: string;
  customerType: "enterprise" | "individual" | "";
  items: ContractTemplateCandidate[];
  orderNo: string;
}

export interface ContractSignerCandidate {
  authorizationVersion: number;
  identityVerificationVersion: number;
  maskedName: string;
  maskedPhone: string;
  phoneVerificationVersion: number;
  signerNo: string;
  version: number;
}

export interface ContractReplacementOptions {
  contractNo: string;
  customerNo: string;
  customerType: "enterprise" | "individual";
  orderNo: string;
  signers: Array<
    Pick<ContractSignerCandidate, "maskedName" | "maskedPhone" | "signerNo">
  >;
  templates: ContractTemplateCandidate[];
}

export interface ContractSubmissionResult {
  contractNo: string;
  flowNo: string;
  reused: boolean;
  statusCode: "result_unknown" | "signing";
  version: number;
}

export interface ContractFailurePolicy {
  createdAt: number;
  handlingDays: number;
  policyNo: string;
  policyVersion: number;
  reused: boolean;
}

export interface ContractFailureCompensationResult {
  compensationNo: string;
  orderNo: string;
  reasonCode: string;
  refundRequestNo: string;
  reused: boolean;
  statusCode: "completed" | "manual_review" | "refund_pending";
}

export interface PreparedContract {
  contractNo: string;
  createdAt: number;
  currencyCode: string;
  customerName: string;
  customerType: "enterprise" | "individual";
  languageCode: "en-US" | "zh-CN";
  lines: Array<{
    amountMinor: string;
    orderItemNo: string;
    periodCount: number;
    periodUnitCode: string;
    purchaseCycles: number;
    quantity: number;
    serviceName: string;
  }>;
  merchantName: string;
  orderNo: string;
  requiredForFulfillment: boolean;
  signerName: string;
  signerNo: string;
  signerPhoneMasked: string;
  signingExpiresAt: number;
  signingOrderCode: string;
  statusCode: "pending_launch";
  templateName: string;
  templateNo: string;
  templateVersion: number;
  totalAmountMinor: string;
  version: number;
}

export function managementContractList(params: {
  page?: number; cursor?: string;
  customerNo?: string;
  orderNo?: string;
  pageSize?: number;
  statusCode?: string;
}) {
  return readPage<{ items: ManagementContract[]; total: number; nextCursor: string }>(
    pluginApiPath(pluginID, contractsPath),
    { params },
  );
}

export function managementContractGet(contractNo: string) {
  return requestClient.get<ManagementContractDetail>(
    pluginApiPath(pluginID, `${contractsPath}/${contractNo}`),
  );
}

export function contractFailurePolicyGet() {
  return requestClient.get<{ item: ContractFailurePolicy }>(
    pluginApiPath(
      pluginID,
      "admin/compliance/electronic-contract/failure-policy",
    ),
  );
}

export function contractFailurePolicyUpdate(input: {
  expectedVersion: number;
  handlingDays: number;
  idempotencyKey: string;
}) {
  return requestClient.put<{ item: ContractFailurePolicy }>(
    pluginApiPath(
      pluginID,
      "admin/compliance/electronic-contract/failure-policy",
    ),
    input,
  );
}

export function contractFailureCancelOrder(contractNo: string) {
  return requestClient.post<{ item: ContractFailureCompensationResult }>(
    pluginApiPath(
      pluginID,
      `${contractsPath}/${contractNo}/order-cancellation`,
    ),
    { confirmed: true },
  );
}

export function manualContractOptions(orderNo: string) {
  return requestClient.get<ContractTemplateCandidateResult>(
    pluginApiPath(pluginID, `${contractsPath}/template-candidates`),
    { params: { orderNo } },
  );
}

export function contractSignerCandidates(customerNo: string) {
  return requestClient.get<{ items: ContractSignerCandidate[] }>(
    pluginApiPath(
      pluginID,
      `admin/compliance/customers/${customerNo}/contract-signer-candidates`,
    ),
  );
}

export function manualContractCreate(input: {
  idempotencyKey: string;
  orderNo: string;
  signerNo: string;
  templateNo: string;
}) {
  return requestClient.post<{ item: PreparedContract }>(
    pluginApiPath(pluginID, contractsPath),
    input,
  );
}

export function contractSubmit(
  contractNo: string,
  expectedVersion: number,
  idempotencyKey: string,
) {
  return requestClient.post<{ item: ContractSubmissionResult }>(
    pluginApiPath(
      pluginID,
      `${contractsPath}/${contractNo}/signing-submissions`,
    ),
    { expectedVersion, idempotencyKey },
  );
}

export function contractReissue(
  contractNo: string,
  expectedVersion: number,
  idempotencyKey: string,
) {
  return requestClient.post<{ item: ContractSubmissionResult }>(
    pluginApiPath(pluginID, `${contractsPath}/${contractNo}/reissues`),
    { expectedVersion, idempotencyKey },
  );
}

export function contractTerminate(
  contractNo: string,
  input: {
    confirmed: true;
    expectedVersion: number;
    idempotencyKey: string;
    originalExpenseSettlement: string;
    originalOtherSettlement: string;
    otherDeals: string;
    reason: string;
    remainInForceItem: string;
  },
) {
  return requestClient.post<{ item: ContractSubmissionResult }>(
    pluginApiPath(pluginID, `${contractsPath}/${contractNo}/terminations`),
    input,
  );
}

export function contractReplacementOptions(contractNo: string) {
  return requestClient.get<ContractReplacementOptions>(
    pluginApiPath(
      pluginID,
      `${contractsPath}/${contractNo}/replacement-options`,
    ),
  );
}

export function contractReplacementPreview(
  contractNo: string,
  input: {
    idempotencyKey: string;
    signerNo: string;
    templateNo: string;
  },
) {
  return requestClient.post<{ item: PreparedContract }>(
    pluginApiPath(
      pluginID,
      `${contractsPath}/${contractNo}/replacement-previews`,
    ),
    input,
  );
}
function readPage<T>(url: string, config: RequestClientConfig = {}) { return requestPreservingHttpStatus<T>(requestClient,url,{...config,method:"GET"}); }
