import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./provider-request";

const pluginID = "idc-resource";

export type ProviderTaskStatus =
  | "accepted"
  | "canceled"
  | "failed"
  | "manual_takeover"
  | "pending"
  | "result_unknown"
  | "retry_wait"
  | "running"
  | "submitting"
  | "succeeded";

export type ProviderTaskTakeoverReason =
  | "compensation_required"
  | "external_coordination_required"
  | "provider_result_unresolved"
  | "provider_task_stalled"
  | "resource_state_conflict";

export interface ProviderTaskManualTakeover {
  assignedUserId: number;
  createdAt: number;
  evidenceCount: number;
  fromStatus: ProviderTaskStatus;
  reasonCode: ProviderTaskTakeoverReason;
  takeoverNo: string;
  taskVersion: number;
}

export interface ProviderTaskItem {
  attemptCount: number;
  capabilityCode: string;
  channelName: string;
  channelNo: string;
  createdAt: number;
  localResourceNo: string;
  maxAttempts: number;
  nextObserveAt: null | number;
  observationAttemptCount: number;
  resultUnknown: boolean;
  status: ProviderTaskStatus;
  takeover: null | ProviderTaskManualTakeover;
  taskNo: string;
  updatedAt: number;
  version: number;
}

export interface ProviderTaskEvidence {
  evidenceKind: string;
  evidenceNo: string;
  observedAt: number;
  reference: string;
  sourceKind: string;
  summary: string;
}

export interface ProviderTaskUpstream {
  externalResourceRef: string;
  externalTaskRef: string;
  observedAt: number;
  progress: number;
  statusCode: string;
}

export interface ProviderTaskDetail {
  evidence: ProviderTaskEvidence[];
  residualResources: ProviderTaskResidualResource[];
  task: ProviderTaskItem;
  upstream: null | ProviderTaskUpstream;
}

export interface ProviderTaskResidualResource {
  reasonCode: string;
  resourceKind: string;
  resourceRef: string;
  status: "present" | "release_failed" | "release_pending" | "unknown";
}

export interface ProviderTaskListResult {
	total: number;
  items: ProviderTaskItem[];
  nextCursor: string;
}

export function providerTaskList(params?: {
	page?: number;
  channelNo?: string;
  cursor?: string;
  limit?: number;
  status?: ProviderTaskStatus | "";
}) {
  return requestPreservingHttpStatus<ProviderTaskListResult>(requestClient,
    pluginApiPath(pluginID, "admin/resource/tasks"),
    { method: "GET", params },
  );
}

export function providerTaskDetail(taskNo: string) {
  return requestClient.get<{ item: ProviderTaskDetail }>(
    pluginApiPath(
      pluginID,
      `admin/resource/tasks/${encodeURIComponent(taskNo)}`,
    ),
  );
}

export function providerTaskManualTakeover(
  taskNo: string,
  input: {
    evidenceNos: string[];
    expectedVersion: number;
    reasonCode: ProviderTaskTakeoverReason;
  },
) {
  return requestClient.post<{ item: ProviderTaskDetail }>(
    pluginApiPath(
      pluginID,
      `admin/resource/tasks/${encodeURIComponent(taskNo)}/manual-takeover`,
    ),
    input,
  );
}
