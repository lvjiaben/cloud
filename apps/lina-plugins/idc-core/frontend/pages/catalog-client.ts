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

export type CatalogProductStatus = "active" | "draft" | "retired" | "suspended";
export type CatalogVersionSource =
  | "copy"
  | "manual"
  | "migration";
export type CatalogVersionStatus =
  | "draft"
  | "published"
  | "retired"
  | "superseded";

export interface CatalogProductSummary {
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
  contractPolicyCode: "inherit" | "none" | "required";
  contractPolicyVersion: number;
  contractTemplateNo: string;
  createdAt: number;
  publishedAt: null | number;
  requiresContract: boolean;
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
  configurationFieldLabelZhCn: string;
  configurationFieldLabelEnUs: string;
  configurationCandidateLabelZhCn: string;
  configurationCandidateLabelEnUs: string;
}

export interface CatalogConfigurationPriceCandidate {
  candidateToken: string;
  fieldLabelZhCN: string;
  fieldLabelEnUS: string;
  candidateLabelZhCN: string;
  candidateLabelEnUS: string;
  dimensionNo: string;
}

export function catalogConfigurationPriceCandidates(versionNo: string) {
  return requestClient.get<{ candidates: CatalogConfigurationPriceCandidate[] }>(
    pluginApiPath(pluginID, `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration-price-candidates`),
  );
}

export function catalogConfigurationPriceDimensionCreate(versionNo: string, candidateToken: string) {
  return requestClient.post<{
    dimensionNo: string;
    fieldLabelZhCN: string;
    fieldLabelEnUS: string;
    candidateLabelZhCN: string;
    candidateLabelEnUS: string;
  }>(pluginApiPath(pluginID, `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration-price-dimensions`), { candidateToken });
}

export function catalogConfigurationPriceScheduleCreate(
  versionNo: string,
  billingRuleNo: string,
  input: {
    configurationPriceDimensionNo: string;
    currencyCode: string;
    status: "active";
    tiers: Array<{
      amountMinor: string;
      maximumQuantity: null;
      minimumQuantity: number;
      renewalAmountMinor?: string;
      setupFeeMinor: string;
    }>;
  },
) {
  return requestClient.post(
    pluginApiPath(pluginID, `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/billing-rules/${encodeURIComponent(billingRuleNo)}/price-schedules`),
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
    pluginApiPath(pluginID, `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/costs`),
  );
}

export function catalogProductVersionCostCreate(versionNo: string, input: CatalogProductVersionCostInput) {
  return requestClient.post<{ cost: CatalogProductVersionCost }>(
    pluginApiPath(pluginID, `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/costs`), input,
  );
}

export function catalogProductVersionCostUpdate(versionNo: string, costNo: string, input: CatalogProductVersionCostInput & { expectedVersion: number }) {
  return requestClient.put<{ cost: CatalogProductVersionCost }>(
    pluginApiPath(pluginID, `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/costs/${encodeURIComponent(costNo)}`), input,
  );
}

export interface CatalogFulfillmentPolicySummary {
  capabilityEnUs: string;
  capabilityZhCn: string;
  quantityHandling: "separately" | "together";
  strategyCode: CatalogFulfillmentStrategy;
}

export function catalogFulfillmentPolicyUpdate(
  versionNo: string,
  strategyCode: CatalogFulfillmentStrategy,
) {
  return requestClient.put<{ policy: CatalogFulfillmentPolicySummary }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/fulfillment-policy`,
    ),
    { strategyCode },
  );
}

export interface CatalogProductVersionConfiguration {
  customerDeclaration?: CatalogConfigurationDeclaration;
  customerResolvedCandidates: Record<string, CatalogConfigurationCandidate[]>;
  merchantValues: CatalogStructuredConfigurationValue[];
  productDeclaration?: CatalogConfigurationDeclaration;
  productResolvedCandidates: Record<string, CatalogConfigurationCandidate[]>;
  version: number;
}

export function catalogProductVersionConfigurationEvaluate(
  versionNo: string,
  input: {
    expectedVersion: number;
    merchantValues: CatalogStructuredConfigurationValue[];
  },
) {
  return requestClient.post<{
    evaluation: CatalogConfigurationEvaluation;
    version: number;
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration/evaluation`,
    ),
    input,
  );
}

export function catalogProductVersionConfigurationGet(versionNo: string) {
  return requestClient.get<{
    configuration: CatalogProductVersionConfiguration | null;
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration`,
    ),
  );
}

export function catalogProductVersionConfigurationUpdate(
  versionNo: string,
  input: {
    expectedVersion: number;
    merchantValues: CatalogStructuredConfigurationValue[];
  },
) {
  return requestClient.put<{
    configuration: CatalogProductVersionConfiguration;
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/configuration`,
    ),
    input,
  );
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
  checks: Array<{ code: CatalogPublicationCheckCode; passed: boolean }>;
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
  categoryNo: string;
  contractPolicyCode: "inherit" | "none" | "required";
  contractTemplateNo: string;
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
  return requestPreservingHttpStatus<CatalogProductListResult>(requestClient,
    pluginApiPath(pluginID, "admin/catalog/products"),
    { method: "GET", params },
  );
}

export function catalogProductDetail(productNo: string) {
  return requestClient.get<CatalogProductDetail>(
    pluginApiPath(
      pluginID,
      `admin/catalog/products/${encodeURIComponent(productNo)}`,
    ),
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

export function catalogCategoryRestore(categoryNo: string, version: number) {
  return requestClient.put<{ category: CatalogCategory }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/categories/${encodeURIComponent(categoryNo)}/restore`,
    ),
    { version },
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
	input: { expectedVersion: number; sellableQuantity: number; trackingEnabled: boolean },
) {
	return requestClient.put<{ inventory: CatalogProductVersionInventory }>(
		pluginApiPath(pluginID, `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/inventory`),
		input,
	);
}
