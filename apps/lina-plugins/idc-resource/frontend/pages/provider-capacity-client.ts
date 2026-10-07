import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./provider-request";

const pluginID = "idc-resource";

export type ProviderCapacityKind = "image" | "line" | "plan" | "region" | "stock";
export type ProviderCapacityStatus =
  | "collecting"
  | "complete"
  | "failed"
  | "superseded";

export interface ProviderCapacityChannelOption {
  channelNo: string;
  displayName: string;
}

export interface ProviderCapacityOperation {
  channelName: string;
  channelNo: string;
  completedAt: null | number;
  failureCode: string;
  itemCount: number;
  kind: ProviderCapacityKind;
  lastObservedAt: number;
  pageCount: number;
  requestedAt: number;
  observationNo: string;
  status: ProviderCapacityStatus;
  trigger: "manual" | "system";
}

export interface ProviderCapacityOperationListParams {
	page?: number;
  channelNo?: string;
  cursor?: string;
  kind?: "" | ProviderCapacityKind;
  limit?: number;
  status?: "" | ProviderCapacityStatus;
}

export interface ProviderCapacityOperationListResult {
	total: number;
  channels: ProviderCapacityChannelOption[];
  items: ProviderCapacityOperation[];
  nextCursor: string;
}

export function providerCapacityOperationList(params?: ProviderCapacityOperationListParams) {
  return requestPreservingHttpStatus<ProviderCapacityOperationListResult>(requestClient,
    pluginApiPath(pluginID, "admin/resource/provider-capacity-observations"),
    { method: "GET", params },
  );
}

export function providerCapacitySyncCreate(
  channelNo: string,
  input: { kind: ProviderCapacityKind },
) {
  return requestClient.post<{ observationNo: string; status: ProviderCapacityStatus }>(
    pluginApiPath(
      pluginID,
      `admin/resource/channels/${encodeURIComponent(channelNo)}/provider-capacity-observations`,
    ),
    input,
  );
}
