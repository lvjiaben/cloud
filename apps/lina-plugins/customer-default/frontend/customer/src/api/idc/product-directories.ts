// Merchants own navigation; the server remains authoritative for service ownership.
import { pluginApiPath, requestClient } from "#/api/request";

export interface CustomerProductDirectory {
  navigationType?: "product" | "system" | "custom" | "plugin";
  targetUrl?: string;
  icon?: string;
  directoryNo: string;
  nameZhCn: string;
  nameEnUs: string;
  sortOrder: number;
  serviceCount: number;
}
export interface DirectoryService {
  entitlementOwnerId?: string;
  managementPreparation?: "preparing" | "ready" | "needs_attention";
  entitlementNo: string;
  productNo: string;
  productNameZhCn: string;
  productNameEnUs: string;
  pluginId: string;
  statusCode: string;
  requiresManualProcessing: boolean;
  startedAt: number;
  expiresAt: null | number;
  orderNo: string;
}
export function listCustomerProductDirectories(
  page = 1,
  pageSize = 100,
  navigationType?: CustomerProductDirectory["navigationType"],
) {
  return requestClient.get<{
    items: CustomerProductDirectory[];
    total: number;
  }>(pluginApiPath("idc-core", "account/product-directories"), {
    params: { page, pageSize, navigationType },
  });
}
export function listDirectoryServices(
  directoryNo: string,
  page = 1,
  pageSize = 20,
) {
  return requestClient.get<{ items: DirectoryService[]; total: number }>(
    pluginApiPath(
      "idc-core",
      `account/product-directories/${encodeURIComponent(directoryNo)}/services`,
    ),
    { params: { page, pageSize } },
  );
}
