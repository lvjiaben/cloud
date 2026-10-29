import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./commerce-request";

const pluginID = "idc-core";

export interface ProductFulfillmentTaskSummary {
  targetEntitlementNo?: string;
  canManualConclude: boolean;
  canReviewApprove: boolean;
  canReviewReject: boolean;
  canTakeover: boolean;
  canRecoverPreparation?: boolean;
  canRequestRefund?: boolean;
  customerNo: string;
  customerCode: string;
  customerDisplayName: string;
  customerPrimaryPhone: string;
  fulfillmentModeCode: string;
  granularityCode: string;
  intentNo: string;
  orderItemNo: string;
  orderNo: string;
  productNameEnUS: string;
  productNameZhCN: string;
  quantity: number;
  statusCode: string;
  unitSequence: number;
  updatedAt: number;
  version: number;
}

export interface ProductFulfillmentEvent {
  eventCode: string;
  occurredAt: number;
  operatorDisplayName?: string;
  reasonCode?: string;
  reasonLabelEnUS?: string;
  reasonLabelZhCN?: string;
  safeNote?: string;
}

export interface ProductFulfillmentReasonOption {
  code: string;
  labelEnUS: string;
  labelZhCN: string;
}

export interface ProductFulfillmentTaskDetail extends ProductFulfillmentTaskSummary {
  managementPreparation?: {
    statusCode: "preparing" | "ready" | "needs_attention";
    attemptCount: number;
    failureCode?: string;
  };
  canRepairManagement?: boolean;
  canVerifyNonAcceptance?: boolean;
  nonAcceptanceVerifications?: Array<{
    providerTaskNo: string;
    actorUserId: number;
    note: string;
    verifiedAt: number;
  }>;
  execution?: {
    responseExcerpt?: string;
    resourceTaskNo?: string;
    statusCode: string;
    phaseCode: string;
    lastResultCode?: string;
    failureCode?: string;
    attemptCount: number;
  };
  events: ProductFulfillmentEvent[];
  recoveryConfigurations?: Array<{
    configuration: FulfillmentRecoveryConfiguration;
    occurredAt: number;
  }>;
  financiallySettled: boolean;
  manualCompletedReasons: ProductFulfillmentReasonOption[];
  manualUnableReasons: ProductFulfillmentReasonOption[];
  reviewApprovalReasons: ProductFulfillmentReasonOption[];
  reviewRejectionReasons: ProductFulfillmentReasonOption[];
  takeoverReasons: ProductFulfillmentReasonOption[];
  recoveryReasons?: ProductFulfillmentReasonOption[];
}

export interface ProductFulfillmentTaskPage {
  total: number;
  nextCursor: string;
  tasks: ProductFulfillmentTaskSummary[];
}

export interface ProductFulfillmentTaskFilters {
  page?: number;
  cursor?: string;
  customerNo?: string;
  fulfillmentModeCode?: string;
  limit?: number;
  orderNo?: string;
  statusCode?: string;
}

export interface ProductFulfillmentActionInput {
  recoveryConfiguration?: FulfillmentRecoveryConfiguration;
  expectedVersion: number;
  idempotencyKey: string;
  note: string;
  reasonCode: string;
}

function path(pathName = "") {
  return pluginApiPath(pluginID, `admin/product-fulfillment/tasks${pathName}`);
}

export function productFulfillmentTasks(params: ProductFulfillmentTaskFilters) {
  return requestPreservingHttpStatus<ProductFulfillmentTaskPage>(
    requestClient,
    path(),
    { method: "GET", params },
  );
}

export function productFulfillmentTask(intentNo: string) {
  return requestClient.get<{ task: ProductFulfillmentTaskDetail }>(
    path(`/${encodeURIComponent(intentNo)}`),
  );
}

export function approveAndStartProductFulfillment(
  intentNo: string,
  input: ProductFulfillmentActionInput,
) {
  return requestClient.post<{ task: ProductFulfillmentTaskDetail }>(
    path(`/${encodeURIComponent(intentNo)}/review-approvals`),
    input,
  );
}

export function rejectProductFulfillmentReview(
  intentNo: string,
  input: ProductFulfillmentActionInput,
) {
  return requestClient.post<{ task: ProductFulfillmentTaskDetail }>(
    path(`/${encodeURIComponent(intentNo)}/review-rejections`),
    input,
  );
}

export function takeoverProductFulfillment(
  intentNo: string,
  input: ProductFulfillmentActionInput,
) {
  return requestClient.post<{ task: ProductFulfillmentTaskDetail }>(
    path(`/${encodeURIComponent(intentNo)}/takeovers`),
    input,
  );
}

export function concludeManualProductFulfillment(
  intentNo: string,
  input: ProductFulfillmentActionInput & {
    outcomeCode: "completed" | "unable_to_complete";
  },
) {
  return requestClient.post<{ task: ProductFulfillmentTaskDetail }>(
    path(`/${encodeURIComponent(intentNo)}/manual-conclusions`),
    input,
  );
}

export function newFulfillmentActionKey() {
  return `fulfillment-${crypto.randomUUID()}`;
}

export function recoverProductFulfillmentPreparation(
  intentNo: string,
  input: ProductFulfillmentActionInput,
) {
  return requestClient.post<{ task: ProductFulfillmentTaskDetail }>(
    path(`/${encodeURIComponent(intentNo)}/recoveries`),
    input,
  );
}

// Retry only the stopped local resource-owner link with the original command key.
export function repairProductFulfillmentManagement(
  intentNo: string,
  input: Pick<
    ProductFulfillmentActionInput,
    "expectedVersion" | "idempotencyKey"
  >,
) {
  return requestClient.post<{ task: ProductFulfillmentTaskDetail }>(
    path(`/${encodeURIComponent(intentNo)}/management-repairs`),
    input,
  );
}

// Staff refund creation reuses the financial permission and original payment sources.
export function refundFailedFulfillment(
  intentNo: string,
  input: Omit<ProductFulfillmentActionInput, "reasonCode">,
) {
  return requestClient.post<{ refund: { refundNo: string } }>(
    pluginApiPath(
      pluginID,
      `admin/billing/failed-fulfillments/${encodeURIComponent(intentNo)}/refunds`,
    ),
    input,
  );
}

// Operational recovery selections deliberately omit all billable quantities.
export interface FulfillmentRecoveryConfiguration {
  generationLength?: number;
  generationCharacters?: string;
  interfaceInstanceNo: string;
  interfaceConfigurationRevision: string;
  region: string;
  line: string;
  operatingSystem: string;
  interfaceLabel: string;
  regionLabel: string;
  lineLabel: string;
  operatingSystemLabel: string;
}
export interface FulfillmentRecoveryEditor {
  original: FulfillmentRecoveryConfiguration;
  selection: FulfillmentRecoveryConfiguration;
  specifications: Array<{ key: string; value: string }>;
  interfaces: Array<{ value: string; labelZhCN: string; labelEnUS: string }>;
  regions: Array<{ value: string; labelZhCN: string; labelEnUS: string }>;
  lines: Array<{ value: string; labelZhCN: string; labelEnUS: string }>;
  operatingSystems: Array<{
    value: string;
    labelZhCN: string;
    labelEnUS: string;
  }>;
}
export async function inspectFulfillmentRecoveryConfiguration(
  intentNo: string,
  selection?: FulfillmentRecoveryConfiguration,
) {
  const response = await requestClient.get<{
    task: ProductFulfillmentTaskDetail;
    recoveryConfiguration: FulfillmentRecoveryEditor;
  }>(path(`/${encodeURIComponent(intentNo)}`), {
    params: {
      includeRecoveryConfiguration: true,
      ...(selection
        ? {
            recoveryInterfaceNo: selection.interfaceInstanceNo,
            recoveryRegion: selection.region,
            recoveryLine: selection.line,
            recoveryOperatingSystem: selection.operatingSystem,
          }
        : {}),
    },
  });
  return response.recoveryConfiguration;
}

// Save a staff verification fact; recovery and refund remain separate actions.
export function verifyFulfillmentNonAcceptance(
  intentNo: string,
  input: {
    expectedVersion: number;
    idempotencyKey: string;
    note: string;
    confirmedNoCreation: boolean;
  },
) {
  return requestClient.post<{ task: ProductFulfillmentTaskDetail }>(
    path(`/${encodeURIComponent(intentNo)}/non-acceptance-verifications`),
    input,
  );
}
