import { pluginApiPath, requestClient } from "#/api/request";

export type WebsiteField = {
  key: string;
  type: string;
  label: { enUS: string; zhCN: string };
  description?: { enUS: string; zhCN: string };
  required?: boolean;
  maximumLength?: number;
};
export type WebsitePage = { id: string; nameZh: string; nameEn: string; path: string };
export type WebsiteNavigationItem = { group?: boolean; id: string; parentId: string; nameZh: string; nameEn: string; pageId: string; url: string; sortOrder: number };
export type WebsiteNavigation = { header: WebsiteNavigationItem[]; footer: WebsiteNavigationItem[] };
export type WebsitePlugin = {
 pages?: WebsitePage[];
  pluginId: string;
  name: string;
  schema: { version: string; fields: WebsiteField[] };
};
export type WebsiteCommon = {
  headerLogoUrl: string;
  footerLogoUrl: string;
  copyright: string;
  businessQq: string;
  companyAddress: string;
  enterpriseWechatQrCodeUrl: string;
  wechatOfficialQrCodeUrl: string;
  telecomLicenseNumber: string;
  telecomLicenseUrl: string;

  icpNumber: string;
  icpUrl: string;
  policeNumber: string;
  policeUrl: string;

  siteName: string;
  siteTitle: string;
  companyName: string;
  contactEmail: string;
  contactPhone: string;
  logoUrl: string;
  description: string;
};
export type WebsiteConfiguration = {
  navigation?: WebsiteNavigation;
  plugins: WebsitePlugin[];
  selectedPlugin: string;
  common: WebsiteCommon;
  config: Record<string, string | number | boolean>;
  configurations?: Record<string, Record<string, string | number | boolean>>;
};
const base = () => pluginApiPath("idc-portal", "admin/site/configuration");
export const getWebsiteConfiguration = () =>
  requestClient.get<WebsiteConfiguration>(base());
export const saveWebsiteConfiguration = (
  input: Omit<WebsiteConfiguration, "plugins">,
) => requestClient.put(base(), input);
