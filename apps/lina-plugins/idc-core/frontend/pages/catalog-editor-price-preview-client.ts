import { pluginApiPath, requestClient } from "#/api/request";
import type { CatalogStructuredConfigurationValue } from "./catalog-configuration-state";

export interface CatalogEditorPricePreviewTerm {
  billingRuleNo: string; status: string; chargeType: string; periodUnit: string; periodCount: number; cycles: number;
  baseAmountMinor: string | null; configurationAmountMinor: string | null; setupAmountMinor: string | null;
  issues?: Array<{kind:"base"|"option";reason:"missing"|"invalid";fieldKey:string;nameZhCN:string;nameEnUS:string}>;
  taxAmountMinor: string | null; totalAmountMinor: string | null;
}
export interface CatalogEditorPricePreview {
  versionNo: string; currencyCode: string; priceIncludesTax: boolean; terms: CatalogEditorPricePreviewTerm[];
}
export function catalogEditorPricePreview(productNo: string, versionNo: string, currencyCode: string, values: CatalogStructuredConfigurationValue[]) {
  const content = JSON.stringify(values);
  if (new TextEncoder().encode(content).length > 32768 || values.length > 128) return Promise.reject(new Error("preview_value_bound"));
  return requestClient.get<{preview:CatalogEditorPricePreview}>(pluginApiPath("idc-core",`admin/catalog/products/${encodeURIComponent(productNo)}/price-preview`), {params:{versionNo,currencyCode,values:content},silentErrorMessage:true,timeout:30000});
}
