import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./provider-request";

const corePluginID = "idc-core";

export type StaffAccessoryAction =
  | "backup_create"
  | "backup_delete"
  | "backup_restore"
  | "nat_create"
  | "nat_delete"
  | "security_group_attach"
  | "security_group_detach"
  | "snapshot_create"
  | "snapshot_delete"
  | "snapshot_restore"
  | "traffic_add"
  | "virtual_media_mount"
  | "virtual_media_unmount";

export interface StaffServerAccessory {
  kind: string;
  name: string;
  objectNo: string;
  projection: Record<string, unknown>;
  state: string;
}

export interface StaffServerAccessoryDetail {
  accessories: StaffServerAccessory[];
  accessoriesExpiresAt?: number;
  accessoriesObservedAt?: number;
  activeOperationNo: string;
  availableActions: StaffAccessoryAction[];
  customerNo: string;
  entitlementStatus: string;
  productNameEnUS: string;
  productNameZhCN: string;
  providerTaskStatus: string;
  runtimeStatus: string;
  serviceNo: string;
}

export type ServerBatchAction =
  | "classification_set"
  | "ip_change"
  | "note_append"
  | "reboot"
  | "reinstall"
  | "renewal_reminder"
  | "suspend"
  | "tag_add"
  | "tag_remove"
  | "terminate";

export interface ServerServiceSummary {
  customerNo: string;
  entitlementExpiresAt?: number;
  entitlementStatus: string;
  productNameEnUS: string;
  productNameZhCN: string;
  providerStatus: string;
  runtimeStatus: string;
  serviceNo: string;
}

export interface ServerBatchItem extends ServerServiceSummary {
  actionAmountMinor: number;
  executionStatus: string;
  failureCode: string;
  irreversible: boolean;
  previewStatus: string;
  resourceSummary: string;
  unpaidAmountMinor: number;
}

export interface ServerBatch {
  action: ServerBatchAction;
  batchNo: string;
  dangerous: boolean;
  items: ServerBatchItem[];
  previewDigest: string;
  status: string;
}

const batchBase = "admin/resource/server-batches";

export async function listStaffServerServices(input: {
	page?: number;
  cursor?: string;
  keyword?: string;
  limit?: number;
}) {
	return requestPreservingHttpStatus<{
		total: number;
    items: ServerServiceSummary[];
    nextCursor: string;
  }>(requestClient, pluginApiPath(corePluginID, "admin/resource/server-services"), {
	method: "GET",
    params: input,
  });
}

export async function previewStaffServerBatch(input: {
  action: ServerBatchAction;
  items: Array<{ parameters: Record<string, unknown>; serviceNo: string }>;
  requestNo: string;
}) {
  const dangerous = [
    "ip_change",
    "reboot",
    "reinstall",
    "suspend",
    "terminate",
  ].includes(input.action);
  return requestClient
    .post<{ batch: ServerBatch }>(
      pluginApiPath(corePluginID, `${batchBase}/${dangerous ? "dangerous" : "safe"}/previews`),
      input,
    )
    .then(({ batch }) => batch);
}

export async function confirmStaffServerBatch(batch: ServerBatch) {
  return requestClient.post<{ expiresAt: number; proofToken: string }>(
    pluginApiPath(corePluginID, `${batchBase}/${encodeURIComponent(batch.batchNo)}/confirmation`),
    { previewDigest: batch.previewDigest },
  );
}

export async function executeStaffServerBatch(
  batch: ServerBatch,
  proofToken = "",
) {
  return requestClient
    .post<{ batch: ServerBatch }>(
      pluginApiPath(
        corePluginID,
        `${batchBase}/${encodeURIComponent(batch.batchNo)}/${batch.dangerous ? "dangerous" : "safe"}-executions`,
      ),
      { previewDigest: batch.previewDigest, proofToken },
    )
    .then(({ batch: result }) => result);
}

export async function getStaffServerAccessories(serviceNo: string) {
  return requestClient.get<{ detail: StaffServerAccessoryDetail }>(
    pluginApiPath(
      corePluginID,
      `admin/resource/server-accessories/${encodeURIComponent(serviceNo)}`,
    ),
  );
}

export async function createStaffServerAccessoryOperation(
  serviceNo: string,
  input: {
    action: StaffAccessoryAction;
    packageNo?: string;
    packageVersion?: number;
    confirmed: true;
    parameters: Record<string, unknown>;
    requestNo: string;
  },
) {
  const operationPath = input.action.replaceAll("_", "-");
  return requestClient.post<{
    action: StaffAccessoryAction;
    amountMinor: string;
    currencyCode: string;
    invoiceNo: string;
    operationNo: string;
    orderNo: string;
    paymentRequired: boolean;
    status: string;
  }>(
    pluginApiPath(
      corePluginID,
      `admin/resource/server-accessories/${encodeURIComponent(serviceNo)}/operations/${operationPath}`,
    ),
    {
      confirmed: input.confirmed,
      packageNo: input.packageNo,
      packageVersion: input.packageVersion,
      parameters: input.parameters,
      requestNo: input.requestNo,
    },
  );
}

export interface StaffTrafficPackage {
  packageNo: string;
  version: number;
  nameZhCN: string;
  nameEnUS: string;
  trafficGb: number;
  currencyCode: string;
  amountMinor: number;
  available: boolean;
}

export function listStaffTrafficPackages(serviceNo: string, params: { page: number; pageSize: number; keyword?: string }) {
  return requestClient.get<{ items: StaffTrafficPackage[]; total: number }>(
    pluginApiPath(corePluginID, `admin/resource/server-accessories/${encodeURIComponent(serviceNo)}/traffic-packages`), { params },
  );
}
