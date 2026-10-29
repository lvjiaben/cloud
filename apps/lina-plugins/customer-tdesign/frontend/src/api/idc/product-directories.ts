// Merchants own navigation; the server remains authoritative for service ownership.
import { pluginApiPath, requestClient } from "#/api/request";

export interface CustomerProductDirectory {
  parentDirectoryNo?: string;
  navigationType?: "product" | "system" | "custom" | "plugin";
  targetUrl?: string;
  icon?: string;
  directoryNo: string;
  nameZhCn: string;
  nameEnUs: string;
  sortOrder: number;
  serviceCount: number;
}
export type ServiceProvider = { pluginId: string; total: number };
export interface DirectoryService {
  customerRemark?: string;
  customerRemarkRevision?: number;
  currentPriceAmountMinor?: string | null;
  currencyCode?: string;
  periodUnitCode?: string;
  periodCount?: number;
  chargeCycles?: number;
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
  keyword = "",
  providerId = "",
) {
  return requestClient.get<{ items: DirectoryService[]; total: number; providers: ServiceProvider[] }>(
    pluginApiPath(
      "idc-core",
      `account/product-directories/${encodeURIComponent(directoryNo)}/services`,
    ),
    { params: { page, pageSize, keyword, providerId } },
  );
}

export type CustomerServiceNote = {
  entitlementNo: string;
  remark: string;
  revision: number;
};
const notePath = (entitlementNo: string) =>
  pluginApiPath(
    "idc-core",
    `account/service-entitlements/${encodeURIComponent(entitlementNo)}/customer-note`,
  );
export function getCustomerServiceNote(entitlementNo: string) {
  return requestClient.get<{
    note: CustomerServiceNote;
    supportAssociationReference: string;
  }>(notePath(entitlementNo));
}
export function saveCustomerServiceNote(
  entitlementNo: string,
  remark: string,
  expectedRevision: number,
) {
  return requestClient.put<{ note: CustomerServiceNote }>(
    notePath(entitlementNo),
    { remark, expectedRevision },
  );
}
