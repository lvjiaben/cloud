import { pluginApiPath, requestClient } from "#/api/request";

export type CustomerCenterField = {
  key: string;
  type: string;
  label: { enUS: string; zhCN: string };
  description?: { enUS: string; zhCN: string };
  required?: boolean;
  maximumLength?: number;
};
export type CustomerCenterPage = { id: string; nameZh: string; nameEn: string; path: string };
export type CustomerCenterPlugin = {
 pages?: CustomerCenterPage[];
  pluginId: string;
  name: string;
  schema: { version: string; fields: CustomerCenterField[] };
};
export type CustomerCenterCommon = {
  siteName: string;
  siteTitle: string;
  companyName: string;
  contactEmail: string;
  contactPhone: string;
  logoUrl: string;
  description: string;
};
export type CustomerCenterConfiguration = {
  plugins: CustomerCenterPlugin[];
  selectedPlugin: string;
  common: CustomerCenterCommon;
  config: Record<string, string | number | boolean>;
  configurations?: Record<string, Record<string, string | number | boolean>>;
};
const base = () => pluginApiPath("idc-portal", "admin/customer-center/configuration");
export const getCustomerCenterConfiguration = () =>
  requestClient.get<CustomerCenterConfiguration>(base());
export const saveCustomerCenterConfiguration = (
  input: Omit<CustomerCenterConfiguration, "plugins">,
) => requestClient.put(base(), input);
