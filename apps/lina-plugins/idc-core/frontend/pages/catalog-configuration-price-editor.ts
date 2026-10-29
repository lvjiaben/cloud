// Input shared by the inline price editor and its legacy dialog adapter.
import type { CatalogBillingRule, CatalogPrice } from "./catalog-client";
export interface CatalogConfigurationPriceEditorData {
  versionNo: string;
  billingRules: CatalogBillingRule[];
  currencyCodes?: string[];
  prices?: CatalogPrice[];
  availablePrices?: CatalogPrice[];
  fieldToken?:string;
  fieldLabel?: { zhCN: string; enUS: string };
  quantityOnly?: boolean;
}
