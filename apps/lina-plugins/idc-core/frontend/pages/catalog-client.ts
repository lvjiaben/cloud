import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./commerce-request";

import type { ProductFulfillmentBinding } from "./catalog-automation-client";
import type {
  CatalogConfigurationCandidate,
  CatalogConfigurationDeclaration,
  CatalogConfigurationEvaluation,
  CatalogStructuredConfigurationValue,
} from "./catalog-configuration-state";
import type { CatalogFulfillmentStrategy } from "./catalog-fulfillment-policy-state";

const pluginID = "idc-core";
// Configuration reads and validation may resolve real supplier choices. Keep
// their bounded wait above the shared client's ten-second default; writes are
// never retried merely because a response is slow.
const configurationRequestTimeout = 30_000;

export type CatalogProductStatus = "active" | "draft" | "retired" | "suspended";
export type CatalogVersionSource = "copy" | "manual" | "migration";
export type CatalogVersionStatus =
  | "draft"
  | "published"
  | "retired"
  | "superseded";

export interface CatalogProductBasePricePreview {
  currencyCode: string;
  amountMinor: string;
  setupFeeMinor: string;
  periodUnit: "once" | "hour" | "day" | "month" | "year";
  periodCount: number;
  chargeType: "one_time" | "recurring" | "trial";
}
export interface CatalogProductSummary {
  availableQuantity: number | null;
  provisionedQuantity: number;
  totalQuantity: number;
  sortOrder: number;
  basePrice?: CatalogProductBasePricePreview | null;
  billingRuleCount: number;
  categoryNameEnUs: string;
  categoryNameZhCn: string;
  categoryNo: string;
  createdAt: number;
  currencyCount: number;
  nameEnUs: string;
  nameZhCn: string;
  productNo: string;
  publicSlug: string;
  publishedAt: null | number;
  source: CatalogVersionSource;
  status: CatalogProductStatus;
  summaryEnUs: string;
  summaryZhCn: string;
  updatedAt: number;
  versionNo: string;
  versionNumber: number;
  versionStatus: CatalogVersionStatus;
}

export interface CatalogProductListParams {
  page?: number;
  categoryNo?: string;
  cursor?: string;
  limit?: number;
  namePrefix?: string;
  source?: "" | CatalogVersionSource;
  status?: "" | CatalogProductStatus;
}

export interface CatalogProductListResult {
  total: number;
  nextCursor: string;
  products: CatalogProductSummary[];
}

export interface CatalogProductDetailProduct {
  currentVersionNo?: string;
  descriptionVersion?: number;
  directoryNo?: string;
  categoryNameEnUs: string;
  categoryNameZhCn: string;
  categoryNo: string;
  createdAt: number;
  nameEnUs: string;
  nameZhCn: string;
  productNo: string;
  publicSlug: string;
  sortOrder: number;
  status: CatalogProductStatus;
  summaryEnUs: string;
  summaryZhCn: string;
  updatedAt: number;
}

export interface CatalogProductDetailVersion {
  changeSummary: string;
  createdAt: number;
  publishedAt: null | number;
  retiredAt: null | number;
  source: CatalogVersionSource;
  sourceVersion: string;
  status: CatalogVersionStatus;
  versionNo: string;
  versionNumber: number;
}

export interface CatalogBillingRule {
  billingRuleNo: string;
  chargeType: string;
  maximumRenewalCycles: null | number;
  meterUnit: null | string;
  minimumPurchaseCycles: number;
  periodCount: number;
  periodUnit: string;
  pricePhase: string;
  quantityMode: string;
  ruleCode: string;
  status: string;
  tierMode: null | string;
  trialQuantity: number;
  usageCollectionCount: null | number;
  usageCollectionUnit: null | string;
  usageMaximumQuantity: null | number;
  usageMinimumQuantity: null | number;
  usageMissingPolicy: null | string;
  usageRoundingMode: null | string;
  usageRoundingQuantity: null | number;
}

export interface CatalogPrice {
  amountMinor: string;
  billingRuleNo: string;
  currencyCode: string;
  maximumQuantity: null | number;
  minimumQuantity: number;
  priceNo: string;
  renewalAmountMinor: null | string;
  setupFeeMinor: string;
  status: string;
  configurationPriceDimensionNo: string;
  configurationPricingMode?:
    | "fixed"
    | "per_unit"
    | "band_total"
    | "graduated"
    | "";
  configurationFieldLabelZhCn: string;
  configurationFieldLabelEnUs: string;
  configurationCandidateLabelZhCn: string;
  configurationCandidateLabelEnUs: string;
}

export interface CatalogConfigurationPriceCandidate {
  fieldToken?: string;
  quantity: boolean;
  pricingMode: "fixed" | "per_unit" | "band_total" | "graduated" | "";
  candidateToken: string;
  fieldLabelZhCN: string;
  fieldLabelEnUS: string;
  candidateLabelZhCN: string;
  candidateLabelEnUS: string;
  dimensionNo: string;
}

export function catalogConfigurationPriceCandidates(
  versionNo: string,
  options?: { silentErrorMessage?: boolean },
) {
  return requestClient.get<{
    candidates: CatalogConfigurationPriceCandidate[];
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration-price-candidates`,
    ),
    { timeout: configurationRequestTimeout, ...options },
  );
}

export function catalogConfigurationPriceDimensionCreate(
  versionNo: string,
  candidateToken: string,
  pricingMode: string = "fixed",
) {
  return requestClient.post<{
    dimensionNo: string;
    fieldLabelZhCN: string;
    fieldLabelEnUS: string;
    candidateLabelZhCN: string;
    candidateLabelEnUS: string;
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration-price-dimensions`,
    ),
    { candidateToken, pricingMode },
  );
}

export interface CatalogPriceScheduleResult {
  tiers: Array<{ priceNo: string }>;
}

export function catalogConfigurationPriceScheduleCreate(
  versionNo: string,
  billingRuleNo: string,
  input: {
    configurationPriceDimensionNo?: string;
    currencyCode: string;
    status: "active" | "inactive";
    tiers: Array<{
      amountMinor: string;
      maximumQuantity: number | null;
      minimumQuantity: number;
      renewalAmountMinor?: string;
      setupFeeMinor: string;
    }>;
  },
) {
  return requestClient.post<CatalogPriceScheduleResult>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/billing-rules/${encodeURIComponent(billingRuleNo)}/price-schedules`,
    ),
    input,
  );
}

export interface CatalogTaxPolicy {
  currencyCode: string;
  priceIncludesTax: boolean;
  roundingMode: "down" | "half_up" | "up";
  roundingQuantumMinor: string;
  roundingScope: "line" | "total";
  taxCategoryCode: string;
  taxPolicyNo: string;
  taxRateBasisPoints: null | number;
  taxTreatment: "exempt" | "taxable" | "zero_rated";
}

export function catalogPriceScheduleUpdate(
  versionNo: string,
  billingRuleNo: string,
  priceNo: string,
  input: Parameters<typeof catalogConfigurationPriceScheduleCreate>[2],
) {
  return requestClient.put<CatalogPriceScheduleResult>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/billing-rules/${encodeURIComponent(billingRuleNo)}/price-schedules/${encodeURIComponent(priceNo)}`,
    ),
    input,
  );
}

export function catalogPriceScheduleDelete(versionNo: string, priceNo: string) {
  return requestClient.delete(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/price-schedules/${encodeURIComponent(priceNo)}`,
    ),
  );
}

export function catalogTaxPolicyCreate(
  versionNo: string,
  input: Omit<CatalogTaxPolicy, "taxPolicyNo">,
) {
  return requestClient.post<CatalogTaxPolicy>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/tax-policies`,
    ),
    input,
  );
}

export interface CatalogProductDetail {
  billingRules: CatalogBillingRule[];
  fulfillmentBinding: null | ProductFulfillmentBinding;
  fulfillmentPolicy: CatalogFulfillmentPolicySummary | null;
  prices: CatalogPrice[];
  product: CatalogProductDetailProduct;
  taxPolicies: CatalogTaxPolicy[];
  version: CatalogProductDetailVersion;
  inventory: CatalogProductVersionInventory;
}

export interface CatalogProductVersionInventory {
  trackingEnabled: boolean;
  sellableQuantity: number;
  version: number;
}

export interface CatalogProductVersionCost {
  costNo: string;
  currencyCode: string;
  effectiveFrom: number;
  effectiveUntil: null | number;
  periodCount: number;
  periodUnit: "day" | "hour" | "month" | "once" | "year";
  unitCostMinor: string;
  version: number;
}

export interface CatalogProductVersionCostInput {
  currencyCode: string;
  effectiveFrom?: number;
  effectiveUntil?: number;
  periodCount: number;
  periodUnit: CatalogProductVersionCost["periodUnit"];
  unitCostMinor: string;
}

export function catalogProductVersionCostList(versionNo: string) {
  return requestClient.get<{ costs: CatalogProductVersionCost[] }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/costs`,
    ),
  );
}

export function catalogProductVersionCostCreate(
  versionNo: string,
  input: CatalogProductVersionCostInput,
) {
  return requestClient.post<{ cost: CatalogProductVersionCost }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/costs`,
    ),
    input,
  );
}

export function catalogProductVersionCostUpdate(
  versionNo: string,
  costNo: string,
  input: CatalogProductVersionCostInput & { expectedVersion: number },
) {
  return requestClient.put<{ cost: CatalogProductVersionCost }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/costs/${encodeURIComponent(costNo)}`,
    ),
    input,
  );
}

export interface CatalogFulfillmentPolicySummary {
  trialSupported?: boolean;
  capabilityEnUs: string;
  capabilityZhCn: string;
  quantityHandling: "separately" | "together";
  strategyCode: CatalogFulfillmentStrategy;
}

export function catalogFulfillmentPolicyUpdate(
  versionNo: string,
  strategyCode: CatalogFulfillmentStrategy,
  interfaceInstanceNo?: string,
  upstreamProduct?: CatalogUpstreamProductSelection,
) {
  return requestClient.put<{ policy: CatalogFulfillmentPolicySummary }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/fulfillment-policy`,
    ),
    {
      strategyCode,
      ...(upstreamProduct ? {upstreamProduct} : {}),
      ...(interfaceInstanceNo === undefined ? {} : { interfaceInstanceNo }),
    },
  );
}

export interface CatalogUpstreamProductSelection {
  requestKey:string; reference:string; categoryNo:string; pricingMode:"markup" | "manual"; markupPercent:string;
  prices:Array<{currencyCode:string;period:string;periodCount:number;amount:string;setupFee:string}>;
}

export interface CatalogCustomerFieldSelection {
  customerVisible?: boolean;
  defaultValue?: unknown;
  upgradeEnabled?: boolean;
  upgradeMinimum?: string;
  upgradeMaximum?: string;
  upgradeCandidateValues?: string[];
  productChoiceBindings?: Record<string, string>;
  informationOnly?: boolean;
  productFieldKey?: string;
  numericPricing?: "quantity" | "per_choice";
  groupZhCN?: string;
  groupEnUS?: string;
  unit?: string;
  descriptionZhCN?: string;
  descriptionEnUS?: string;
  advanced?: boolean;
  numericLabels?: Record<string, { zhCN: string; enUS: string }>;
  localField?: CatalogConfigurationDeclaration["fields"][number];
  displayControl?: "auto" | "input" | "slider" | "select" | "radio";
  quantityStep?: string;
  numericChoices?: string[];
  displayNameZhCN?: string;
  displayNameEnUS?: string;
  sortOrder?: number;
  fieldKey: string;
  minimum?: string;
  maximum?: string;
  candidateValues?: string[];
  candidateLabels?: Record<string, { zhCN: string; enUS: string }>;
}

export interface CatalogProductVersionConfiguration {
  pendingAutomationFields?: string[];
  // Read-only facts; the writer independently validates current mappings.
  directoryRelationships?: CatalogDirectoryRelationship[];
  customerChoiceRules?: Record<
    string,
    import("./catalog-configuration-state").CatalogConfigurationRule[]
  >;
  merchantPriceTargets?: CatalogMerchantPriceTarget[];
  productChoiceFields?: string[];
  customerProductFields?: Record<string, string>;
  priceFieldTokens?: Record<string, string>;
  upstreamOptionsStatus?: "ready" | "pending" | "failed" | "retained";
  customerConfigurableFields?: CatalogConfigurationDeclaration["fields"];
  customerFieldSelections?: CatalogCustomerFieldSelection[];
  importedConfiguration?: boolean;
  importedProductReference?:string;
  importedProductName?:string;
  importedPricingMode?:"markup"|"manual"|"mixed";
  importedMarkupPercent?:string;
  importedManualPrices?:CatalogUpstreamProductSelection["prices"];
  customerDeclaration?: CatalogConfigurationDeclaration;
  customerResolvedCandidates: Record<string, CatalogConfigurationCandidate[]>;
  merchantValues: CatalogStructuredConfigurationValue[];
  productDeclaration?: CatalogConfigurationDeclaration;
  productResolvedCandidates: Record<string, CatalogConfigurationCandidate[]>;
  version: number;
}

export interface CatalogDirectoryRelationship {
  parentFieldKey: string;
  childFieldKey: string;
  links: Array<{ parentValue: string; childValue: string }>;
}

export interface CatalogMerchantPriceTarget {
  fieldKey: string;
  candidateValue: string;
  quantity: boolean;
  dimensionNo: string;
  pricingMode: "fixed" | "per_unit" | "band_total" | "graduated" | "";
}

export interface CatalogMerchantPriceRow {
  fieldKey: string;
  candidateValue: string;
  quantity: boolean;
  pricingMode: "fixed" | "per_unit" | "band_total" | "graduated";
  active: boolean;
  previousPriceNo: string;
  billingRuleNo: string;
  currencyCode: string;
  tiers: Array<{
    minimumQuantity: number;
    maximumQuantity: number | null;
    amountMinor: string;
    setupFeeMinor: string;
    renewalAmountMinor?: string;
  }>;
}

export async function catalogProductVersionConfigurationEvaluate(
  versionNo: string,
  input: {
    expectedVersion: number;
    merchantValues: CatalogStructuredConfigurationValue[];
  },
) {
  const evaluate = () =>
    requestClient.post<{
      evaluation: CatalogConfigurationEvaluation;
      version: number;
    }>(
      pluginApiPath(
        pluginID,
        `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration/evaluation`,
      ),
      input,
      { silentErrorMessage: true, timeout: configurationRequestTimeout },
    );
  try {
    return await evaluate();
  } catch (error) {
    const response = error as {
      code?: number;
      response?: { data?: { code?: number } };
    };
    // Validation is read-only. Retry one transient service failure without
    // emitting a stale global toast; conflicts and authorization never retry.
    if ((response?.code ?? response?.response?.data?.code) === 50) {
      await new Promise((resolve) => globalThis.setTimeout(resolve, 250));
      return await evaluate();
    }
    throw error;
  }
}

// Empty declarations omit collections on the wire. An unbound commercial
// offer is still a real editable schema and must not crash form consumers.
function normalizeProductConfiguration(
  configuration: CatalogProductVersionConfiguration,
) {
  return {
    ...configuration,
    productDeclaration: configuration.productDeclaration
      ? {
          ...configuration.productDeclaration,
          fields: configuration.productDeclaration.fields ?? [],
        }
      : undefined,
    customerDeclaration: configuration.customerDeclaration
      ? {
          ...configuration.customerDeclaration,
          fields: configuration.customerDeclaration.fields ?? [],
        }
      : undefined,
    merchantValues: configuration.merchantValues ?? [],
    customerConfigurableFields: configuration.customerConfigurableFields ?? [],
    customerFieldSelections: configuration.customerFieldSelections ?? [],
    productResolvedCandidates: configuration.productResolvedCandidates ?? {},
    customerResolvedCandidates: configuration.customerResolvedCandidates ?? {},
  };
}

export async function catalogProductVersionConfigurationGet(versionNo: string) {
  const result = await requestClient.get<{
    configuration: CatalogProductVersionConfiguration | null;
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration`,
    ),
    { timeout: configurationRequestTimeout },
  );
  return {
    ...result,
    configuration: result.configuration
      ? normalizeProductConfiguration(result.configuration)
      : null,
  };
}

export async function catalogProductVersionConfigurationUpdate(
  versionNo: string,
  input: {
    priceRows?: CatalogMerchantPriceRow[];
    customerFieldSelections?: CatalogCustomerFieldSelection[];
    expectedVersion: number;
    merchantValues: CatalogStructuredConfigurationValue[];
  },
) {
  const result = await requestClient.put<{
    configuration: CatalogProductVersionConfiguration;
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration`,
    ),
    input,
    { timeout: configurationRequestTimeout },
  );
  return {
    ...result,
    configuration: normalizeProductConfiguration(result.configuration),
  };
}

export interface CatalogManagementConfigurationFileItem {
  expiresAt: number;
  fileName: string;
  fileToken: string;
  mediaType: string;
  sizeBytes: number;
  status: string;
}
export async function catalogProductVersionConfigurationFileUpload(
  versionNo: string,
  targetRevision: number,
  fieldKey: string,
  file: File,
) {
  const base = pluginApiPath(
    pluginID,
    `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration/file-uploads`,
  );
  const reservation =
    await requestClient.post<CatalogManagementConfigurationFileItem>(base, {
      fieldKey,
      fileName: file.name,
      sizeBytes: file.size,
      targetRevision,
    });
  return requestClient.put<CatalogManagementConfigurationFileItem>(
    `${base}/${encodeURIComponent(reservation.fileToken)}/content`,
    file,
    {
      headers: { "Content-Type": file.type || "application/octet-stream" },
      params: { targetRevision },
    },
  );
}
export function catalogProductVersionConfigurationFileRevoke(
  versionNo: string,
  targetRevision: number,
  fileToken: string,
) {
  return requestClient.delete<CatalogManagementConfigurationFileItem>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration/file-uploads/${encodeURIComponent(fileToken)}`,
    ),
    { params: { targetRevision } },
  );
}

export interface CatalogProductPublicationResult {
  productNo: string;
  publishedAt: number;
  status: "published";
  supersededVersionNo: string;
  versionNo: string;
}

export type CatalogPublicationCheckCode =
  | "automation"
  | "category"
  | "lifecycle"
  | "pricing"
  | "sales_copy"
  | "tax";

export interface CatalogProductPublicationPreview {
  checks: Array<{
    code: CatalogPublicationCheckCode;
    passed: boolean;
    reasonCode?: string;
  }>;
  productNo: string;
  ready: boolean;
  versionNo: string;
}

export interface CatalogCategory {
  categoryNo: string;
  deleted: boolean;
  nameEnUs: string;
  nameZhCn: string;
  parentCategoryNo: string;
  sortOrder: number;
  status: "active";
  version: number;
}

export interface CatalogCategoryCreateInput {
  nameEnUs: string;
  nameZhCn: string;
  parentCategoryNo?: string;
  sortOrder: number;
}

export interface CatalogCategoryUpdateInput extends CatalogCategoryCreateInput {
  version: number;
}

export interface CatalogProductCreateInput {
  directoryNo?: string;
  categoryNo: string;
  nameEnUs: string;
  nameZhCn: string;
  publicSlug: string;
  sortOrder: number;
  summaryEnUs: string;
  summaryZhCn: string;
  inventoryTrackingEnabled: boolean;
  sellableQuantity: number;
}

export interface CatalogProductCreateResult {
  categoryNo: string;
  productNo: string;
  source: "manual";
  status: "draft";
  versionNo: string;
  versionNumber: number;
  versionStatus: "draft";
}

export function catalogProductList(params?: CatalogProductListParams) {
  return requestPreservingHttpStatus<CatalogProductListResult>(
    requestClient,
    pluginApiPath(pluginID, "admin/catalog/products"),
    { method: "GET", params },
  );
}

export function catalogProductDetail(
  productNo: string,
  options?: { silentErrorMessage?: boolean },
) {
  return requestClient.get<CatalogProductDetail>(
    pluginApiPath(
      pluginID,
      `admin/catalog/products/${encodeURIComponent(productNo)}`,
    ),
    options,
  );
}

export function catalogProductDescriptionUpdate(
  productNo: string,
  input: {
    sortOrder?: number;
    categoryNo?: string;
    directoryNo?: string;
    version: number;
    nameZhCn: string;
    nameEnUs: string;
    summaryZhCn: string;
    summaryEnUs: string;
    versionNo?: string;
    inventory?: {trackingEnabled:boolean; sellableQuantity:number; expectedVersion:number};
    addonProductNos?: string[];
    loginRules?: {expectedVersion:number; values:CatalogStructuredConfigurationValue[]};
  },
) {
  return requestClient.put<{ version: number }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/products/${encodeURIComponent(productNo)}/description`,
    ),
    input,
  );
}

export function catalogProductPublish(versionNo: string) {
  return requestClient.post<CatalogProductPublicationResult>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/publish`,
    ),
  );
}

export function catalogProductPublicationPreview(versionNo: string) {
  return requestClient.get<CatalogProductPublicationPreview>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/publication-preview`,
    ),
  );
}

export function catalogCategoryList(
  deletionState: "active" | "all" | "deleted" = "active",
) {
  return requestClient.get<{ categories: CatalogCategory[] }>(
    pluginApiPath(pluginID, "admin/catalog/categories"),
    { params: { deletionState } },
  );
}

export function catalogCategoryCreate(input: CatalogCategoryCreateInput) {
  return requestClient.post<{ category: CatalogCategory }>(
    pluginApiPath(pluginID, "admin/catalog/categories"),
    input,
  );
}

export function catalogCategoryUpdate(
  categoryNo: string,
  input: CatalogCategoryUpdateInput,
) {
  return requestClient.put<{ category: CatalogCategory }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/categories/${encodeURIComponent(categoryNo)}`,
    ),
    input,
  );
}

export function catalogCategoryDelete(categoryNo: string, version: number) {
  return requestClient.delete(
    pluginApiPath(
      pluginID,
      `admin/catalog/categories/${encodeURIComponent(categoryNo)}`,
    ),
    { params: { version } },
  );
}

export function catalogProductCreate(input: CatalogProductCreateInput) {
  return requestClient.post<CatalogProductCreateResult>(
    pluginApiPath(pluginID, "admin/catalog/products"),
    input,
  );
}

export function catalogProductVersionInventoryUpdate(
  versionNo: string,
  input: {
    expectedVersion: number;
    sellableQuantity: number;
    trackingEnabled: boolean;
  },
) {
  return requestClient.put<{ inventory: CatalogProductVersionInventory }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/inventory`,
    ),
    input,
  );
}

// Copies settings into a new draft; historical purchases and source stock stay owned by the source.
export function catalogProductCopy(
  productNo: string,
  input: {
    sourceVersionNo: string;
    nameZhCn: string;
    nameEnUs: string;
    publicSlug: string;
  },
) {
  return requestClient.post<CatalogProductCreateResult>(
    pluginApiPath(
      pluginID,
      `admin/catalog/products/${encodeURIComponent(productNo)}/copies`,
    ),
    input,
  );
}

export function catalogProductRevisionCreate(
  productNo: string,
  sourceVersionNo: string,
) {
  return requestClient.post<CatalogProductCreateResult>(
    pluginApiPath(
      pluginID,
      `admin/catalog/products/${encodeURIComponent(productNo)}/draft-versions`,
    ),
    { sourceVersionNo },
  );
}

export function catalogProductSaleStateUpdate(
  productNo: string,
  input: { versionNo: string; expectedVersion: number; active: boolean },
) {
  return requestClient.put<{ version: number }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/products/${encodeURIComponent(productNo)}/sale-state`,
    ),
    input,
  );
}

export function catalogConfigurationPriceBatchSave(
  versionNo: string,
  rows: Array<{
    active?: boolean;
    targetKind?: "base" | "configuration";
    candidateToken?: string;
    previousPriceNo: string;
    billingRuleNo: string;
    currencyCode: string;
    tiers: Array<{
      minimumQuantity: number;
      maximumQuantity: null;
      amountMinor: string;
      setupFeeMinor: string;
      renewalAmountMinor?: string;
    }>;
  }>,
) {
  return requestClient.post<{ saved: number }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration-price-batches`,
    ),
    { rows },
  );
}

export function catalogPricedBillingRuleCreate(
  versionNo: string,
  input: {
    ruleCode: string;
    periodUnit: "once" | "hour" | "day" | "month" | "year";
    periodCount: number;
    minimumPurchaseCycles: number;
    maximumRenewalCycles?: number;
    currencyCode: string;
    amountMinor: string;
    setupFeeMinor: string;
    renewalAmountMinor?: string;
  },
) {
  return requestClient.post<{ billingRuleNo: string }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/priced-billing-rules`,
    ),
    input,
  );
}

export function catalogProductDelete(productNo: string, version: number) {
  return requestClient.delete<void>(pluginApiPath(pluginID, `admin/catalog/products/${encodeURIComponent(productNo)}`), { params: {version} });
}
