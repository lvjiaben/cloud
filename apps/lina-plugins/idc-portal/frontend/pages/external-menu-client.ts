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

export interface CustomerExternalMenuInput {
  icon: string;
  id?: number;
  nameEn: string;
  nameZh: string;
  sortOrder: number;
  url: string;
  version?: number;
}

// Resolve the plugin URL when an action is invoked. Keeping this out of module
// initialization avoids a circular startup dependency through the auth store.
const base = () =>
  pluginApiPath("idc-portal", "admin/customer-external-menus");

export function listCustomerExternalMenus() {
  return requestClient.get<{ items: CustomerExternalMenu[] }>(base());
}

export function saveCustomerExternalMenu(input: CustomerExternalMenuInput) {
  return requestClient.post<{ item: CustomerExternalMenu }>(base(), input);
}

export function setCustomerExternalMenuEnabled(
  item: CustomerExternalMenu,
  enabled: boolean,
) {
  return requestClient.put<{ item: CustomerExternalMenu }>(
    `${base()}/${item.id}/status`,
    { enabled, version: item.version },
  );
}
