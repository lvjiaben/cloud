import { pluginApiPath, requestClient } from "#/api/request";
export interface ServerCommercialRecord {
  operationNo: string;
  quoteNo: string;
  orderNo: string;
  invoiceNo: string;
  currencyCode: string;
  totalAmountMinor: string;
  action: string;
  status: string;
  lastErrorCode: string;
  createdAt: number;
  updatedAt: number;
  completedAt?: number | null;
}
export interface ServerAccessoryBill {
  accessoryEntitlementNo: string;
  objectNo: string;
  kind: string;
  billingMode: string;
  status: string;
  currencyCode: string;
  periodUnit: string;
  periodCount: number;
  currentPeriodStartAt: number | null;
  currentPeriodEndAt: number | null;
  autoRenewEnabled: boolean;
  pendingInvoiceNo: string;
  pendingInvoiceStatus: string;
  pendingAmountMinor: string | null;
  createdAt: number;
  updatedAt: number;
}
const path = (serverNo: string, suffix: string) =>
  pluginApiPath(
    "idc-resource",
    `account/native-servers/${encodeURIComponent(serverNo)}/${suffix}`,
  );
export function listServerCommercialRecords(
  serverNo: string,
  cursor = "",
  operationNo = "",
) {
  return requestClient.get<{
    items: ServerCommercialRecord[];
    nextCursor: string;
  }>(path(serverNo, "commercial-operations"), {
    params: { cursor, operationNo, limit: 20 },
    silentErrorMessage: true,
  });
}
export function listServerAccessoryBills(
  serverNo: string,
  cursor = "",
  kind = "",
) {
  return requestClient.get<{
    items: ServerAccessoryBill[];
    nextCursor: string;
  }>(path(serverNo, "accessory-billing"), {
    params: { cursor, kind, limit: 20 },
    silentErrorMessage: true,
  });
}
