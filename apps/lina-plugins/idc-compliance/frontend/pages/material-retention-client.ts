import { pluginApiPath, requestClient } from "#/api/request";

const base = "admin/compliance/privacy";
const pluginID = "idc-compliance";
const path = (suffix: string) => pluginApiPath(pluginID, `${base}/${suffix}`);

export interface RetentionItem {
  policyNo: string;
  dataTypeCode: string;
  retentionDays: number;
  version: number;
  createdAt: number;
}
export interface DeletionItem {
  taskNo: string;
  dataTypeCode: string;
  targetTypeCode: string;
  targetNo: string;
  statusCode: string;
  dueAt: number;
  holdReasonCode: string;
  version: number;
}
export const listRetention = () =>
  requestClient.get<{ items: RetentionItem[] }>(path("retention-policies"));
export const setRetention = (input: {
  dataTypeCode: string;
  retentionDays: number;
  idempotencyKey: string;
}) =>
  requestClient.put<{ item: RetentionItem }>(path("retention-policies"), input);
export const listDeletion = (page = 1, pageSize = 20) =>
  requestClient.get<{ items: DeletionItem[]; total: number }>(path("deletion-tasks"), {
    params: { page, pageSize },
  });
export const scanDeletion = () =>
  requestClient.post<{ created: number }>(path("deletion-tasks/scans"), {
    limit: 100,
  });
export const confirmDeletion = (item: DeletionItem, key: string) =>
  requestClient.post<{ item: DeletionItem }>(
    path(`deletion-tasks/${item.taskNo}/confirmations`),
    { confirmed: true, expectedVersion: item.version, idempotencyKey: key },
  );
export const changeMaterialHold = (item: DeletionItem, release: boolean, key: string) =>
  requestClient.post(path(`retention-holds${release ? "/releases" : ""}`), {
    targetTypeCode: item.targetTypeCode,
    targetNo: item.targetNo,
    idempotencyKey: key,
  });
