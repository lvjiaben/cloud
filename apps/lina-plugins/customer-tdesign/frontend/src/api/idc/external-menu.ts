import { pluginApiPath, requestClient } from "#/api/request";

export interface CustomerExternalMenu {
  enabled: boolean;
  icon: string;
  id: number;
  nameEn: string;
  nameZh: string;
  sortOrder: number;
  url: string;
  version: number;
}

export function listCustomerExternalMenus() {
  return requestClient.get<{ items: CustomerExternalMenu[] }>(
    pluginApiPath("idc-portal", "customer/external-menus"),
  );
}
