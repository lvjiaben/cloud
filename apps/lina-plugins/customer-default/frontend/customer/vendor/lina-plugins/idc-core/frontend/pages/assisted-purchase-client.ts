import { pluginApiPath, requestClient } from "#/api/request";
import type {
  CatalogConfigurationCandidate, CatalogConfigurationDeclaration,
  CatalogConfigurationFieldType, CatalogConfigurationValues,
  CatalogStructuredConfigurationValue,
} from "./catalog-configuration-state";
import { catalogConfigurationRowsToValues, catalogConfigurationValuesToRows } from "./catalog-configuration-state";

export interface AssistedProduct {
  publicSlug: string; nameZhCN: string; nameEnUS: string;
  currencies: string[]; availability: string;
}
export interface PurchaseOption {
  optionToken: string; labelZhCN: string; labelEnUS: string; children?: PurchaseOption[];
}
export interface PurchaseField {
  fieldToken: string; type: CatalogConfigurationFieldType;
  labelZhCN: string; labelEnUS: string; descriptionZhCN: string; descriptionEnUS: string;
  visible: boolean; required: boolean; disabled: boolean;
  format?: "domain" | "email" | "phone" | "url";
  minimumDecimal?: string; maximumDecimal?: string; minimumInteger?: string; maximumInteger?: string;
  minimumLength?: number; maximumLength?: number; options?: PurchaseOption[];
  uploadPolicy?: NonNullable<CatalogConfigurationDeclaration["uploadPolicies"]>[number];
}
export interface PurchaseValue extends Omit<CatalogStructuredConfigurationValue, "fieldKey"> {
  fieldToken: string; type: CatalogConfigurationFieldType;
}
export interface PurchaseConfiguration {
  configurationRevision: string; fields: PurchaseField[]; values: PurchaseValue[];
}
export interface PurchaseEvaluation extends PurchaseConfiguration {
  valid: boolean; issues: { fieldToken?: string; issueCode: string }[];
}
export interface PurchaseTerm {
  billingTermToken: string; labelZhCN: string; labelEnUS: string;
  minimumPurchaseCycles: number; status: string;
  setupAmount: string; subtotalAmount: string; taxAmount: string; totalAmount: string;
}
const path = (slug: string) => pluginApiPath("idc-core", `public/catalog/products/${encodeURIComponent(slug)}/purchase-configuration`);
export function assistedProducts(params: { namePrefix?: string; currency?: string; cursor?: string }) {
  return requestClient.get<{ products: AssistedProduct[]; availableCurrencies: string[]; nextCursor: string }>(
    pluginApiPath("idc-core", "public/catalog/products"), { params: { ...params, limit: 24 } },
  );
}
export function purchaseConfiguration(slug: string) { return requestClient.get<PurchaseConfiguration>(path(slug)); }
export function evaluatePurchase(slug: string, configurationRevision: string, values: PurchaseValue[]) {
  return requestClient.post<PurchaseEvaluation>(`${path(slug)}/evaluate`, { configurationRevision, values });
}
export function quotePurchase(slug: string, configurationRevision: string, currencyCode: string, values: PurchaseValue[]) {
  return requestClient.post<{ terms: PurchaseTerm[] }>(`${path(slug)}/quote`, { configurationRevision, currencyCode, values });
}
function candidates(options: PurchaseOption[] = [], parentValue?: string): CatalogConfigurationCandidate[] {
  return options.flatMap(option => [{ value: option.optionToken, parentValue, label: { zhCN: option.labelZhCN, enUS: option.labelEnUS } }, ...candidates(option.children, option.optionToken)]);
}
// Adapt public opaque tokens to the plugin's existing governed field renderer.
// Visibility, validation state and available choices always come from the server.
export function purchaseDeclaration(view: PurchaseConfiguration): CatalogConfigurationDeclaration {
  return {
    version: view.configurationRevision,
    fields: view.fields.map(field => ({
      key: field.fieldToken, type: field.type,
      label: { zhCN: field.labelZhCN, enUS: field.labelEnUS },
      description: { zhCN: field.descriptionZhCN, enUS: field.descriptionEnUS },
      required: field.required && field.visible && !field.disabled,
      textFormat: field.format, minimum: field.minimumInteger || field.minimumDecimal,
      maximum: field.maximumInteger || field.maximumDecimal,
      minimumLength: field.minimumLength || undefined, maximumLength: field.maximumLength || undefined,
      staticCandidates: candidates(field.options), uploadPolicy: field.uploadPolicy ? field.fieldToken : undefined,
    })),
    uploadPolicies: view.fields.filter(field => field.uploadPolicy).map(field => ({ ...field.uploadPolicy!, id: field.fieldToken })),
  };
}
export function purchaseDraft(view: PurchaseConfiguration) {
  return catalogConfigurationRowsToValues(view.values.map(({ fieldToken, type: _type, ...value }) => ({ ...value, fieldKey: fieldToken })), purchaseDeclaration(view));
}
export function purchaseValues(view: PurchaseConfiguration, draft: CatalogConfigurationValues): PurchaseValue[] {
  const types = new Map(view.fields.map(field => [field.fieldToken, field.type]));
  const activeDraft = Object.fromEntries(view.fields.filter(field => field.visible && !field.disabled).map(field => [field.fieldToken, draft[field.fieldToken]]));
  return catalogConfigurationValuesToRows(purchaseDeclaration(view), "customer", activeDraft)
    .map(({ fieldKey, ...value }) => ({ ...value, fieldToken: fieldKey, type: types.get(fieldKey)! }));
}
