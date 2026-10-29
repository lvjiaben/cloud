import { pluginApiPath, requestClient } from "#/api/request";

export interface AccessoryRenewalWorkItem {
  workItemNo: string;
  invoiceNo: string;
  customerNo: string;
  subjectNo: string;
  reasonCode: "balance_payment_failed" | "deletion_invoice_pending";
  createdAt: number;
}

const path = (resource: string) => pluginApiPath("idc-core", `admin/${resource}`);

export function syncAccessoryRenewalNotifications(locale: "en-US" | "zh-CN") {
  return requestClient.post<{ total: number; eventNo: string }>(
    path("server-accessory-renewal-notification-syncs"),
    { locale },
    { silentErrorMessage: true },
  );
}

export function accessoryRenewalWorkItems(page: number, pageSize: number) {
  return requestClient.get<{ items: AccessoryRenewalWorkItem[]; total: number }>(
    path("server-accessory-renewal-work-items"),
    { params: { page, pageSize }, silentErrorMessage: true },
  );
}
