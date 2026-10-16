import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./commerce-request";

const pluginID = "idc-core";

export interface ProductFulfillmentTaskSummary {
  targetEntitlementNo?: string;
  canManualConclude: boolean;
  canReviewApprove: boolean;
  canReviewReject: boolean;
  canTakeover: boolean;
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

export interface ProductFulfillmentTaskDetail
  extends ProductFulfillmentTaskSummary {
  events: ProductFulfillmentEvent[];
  financiallySettled: boolean;
  manualCompletedReasons: ProductFulfillmentReasonOption[];
  manualUnableReasons: ProductFulfillmentReasonOption[];
  reviewApprovalReasons: ProductFulfillmentReasonOption[];
  reviewRejectionReasons: ProductFulfillmentReasonOption[];
  takeoverReasons: ProductFulfillmentReasonOption[];
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
  expectedVersion: number;
  idempotencyKey: string;
  note: string;
  reasonCode: string;
}

function path(pathName = "") {
  return pluginApiPath(
    pluginID,
    `admin/product-fulfillment/tasks${pathName}`,
  );
}

export function productFulfillmentTasks(
  params: ProductFulfillmentTaskFilters,
) {
  return requestPreservingHttpStatus<ProductFulfillmentTaskPage>(requestClient, path(), { method: "GET", params });
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
