/** Governed customer purchase-configuration and private file-upload client. */
import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-core';
const path = (value: string) => pluginApiPath(pluginID, value);

export type PurchaseFieldType =
  | 'address' | 'boolean' | 'date' | 'datetime' | 'decimal' | 'file'
  | 'integer' | 'multi_line' | 'multi_select' | 'single_line'
  | 'single_select' | 'tree_select';

export interface PurchaseOption {
  children?: PurchaseOption[];
  labelEnUS: string;
  labelZhCN: string;
  optionToken: string;
}

export interface PurchaseUploadPolicy {
  allowedExtensions: string[];
  allowedMimeTypes: string[];
  maximumFileBytes: number;
  maximumFiles: number;
  maximumTotalBytes: number;
  safetyScanRequired: boolean;
}

export interface PurchaseField {
  descriptionEnUS: string;
  descriptionZhCN: string;
  disabled: boolean;
  fieldToken: string;
  labelEnUS: string;
  labelZhCN: string;
  maximumLength?: number;
  options?: PurchaseOption[];
  required: boolean;
  type: PurchaseFieldType;
  uploadPolicy?: PurchaseUploadPolicy;
  visible: boolean;
}

export interface PurchaseValue {
  addressOptionTokens?: string[];
  booleanValue?: boolean;
  dateTimeValue?: string;
  dateValue?: string;
  decimalValue?: string;
  fieldToken: string;
  fileTokens?: string[];
  integerValue?: string;
  optionTokens?: string[];
  textValue?: string;
  treeOptionTokens?: string[];
  type: PurchaseFieldType;
}

export interface PurchaseConfigurationView {
  configurationRevision: string;
  fields: PurchaseField[];
  values: PurchaseValue[];
}

export interface PurchaseEvaluation extends PurchaseConfigurationView {
  issues: Array<{ fieldToken?: string; issueCode: string }>;
  valid: boolean;
}

export interface PurchaseQuoteTerm {
  billingTermToken: string;
  labelEnUS: string;
  labelZhCN: string;
  minimumPurchaseCycles: number;
  setupAmount: string;
  status: 'final' | 'requires_customer_context';
  subtotalAmount: string;
  taxAmount: string;
  totalAmount: string;
}

export interface PurchaseFileItem {
  expiresAt: number;
  fileName: string;
  fileToken: string;
  mediaType: string;
  sizeBytes: number;
  status: 'ready' | 'reserved' | 'revoked';
}

export interface PublicCatalogProduct {
  availability: 'available' | 'manual_confirmation' | 'unavailable';
  categoryNameEnUS: string;
  categoryNameZhCN: string;
  currencies: string[];
  nameEnUS: string;
  nameZhCN: string;
  publicSlug: string;
  summaryEnUS: string;
  summaryZhCN: string;
}

export function listPublicCatalogProducts(cursor = '') {
  return requestClient.get<{ nextCursor: string; products: PublicCatalogProduct[] }>(
    path('public/catalog/products'),
    { params: { cursor: cursor || undefined, limit: 24 } },
  );
}

export function getPurchaseConfiguration(publicSlug: string) {
  return requestClient.get<PurchaseConfigurationView>(
    path(`public/catalog/products/${encodeURIComponent(publicSlug)}/purchase-configuration`),
  );
}

export function evaluatePurchaseConfiguration(publicSlug: string, configurationRevision: string, values: PurchaseValue[]) {
  return requestClient.post<PurchaseEvaluation>(
    path(`account/catalog/products/${encodeURIComponent(publicSlug)}/purchase-configuration/evaluate`),
    { configurationRevision, values },
  );
}

export function quotePurchaseConfiguration(publicSlug: string, configurationRevision: string, currencyCode: string, values: PurchaseValue[]) {
  return requestClient.post<{ configurationRevision: string; currencyCode: string; terms: PurchaseQuoteTerm[] }>(
    path(`account/catalog/products/${encodeURIComponent(publicSlug)}/purchase-configuration/quote`),
    { configurationRevision, currencyCode, values },
  );
}

export function freezePurchaseConfiguration(quoteItemNo: string, publicSlug: string, configurationRevision: string, values: PurchaseValue[]) {
  return requestClient.post<{ snapshotNo: string }>(
    path(`account/commerce/quote-items/${encodeURIComponent(quoteItemNo)}/purchase-configuration`),
    { publicSlug, configurationRevision, values },
  );
}

export async function uploadPurchaseConfigurationFile(publicSlug: string, configurationRevision: string, fieldToken: string, file: File) {
  const reservation = await requestClient.post<{ expiresAt: number; uploadToken: string }>(
    path(`account/catalog/products/${encodeURIComponent(publicSlug)}/purchase-configuration/file-uploads`),
    { configurationRevision, fieldToken, fileName: file.name, sizeBytes: file.size },
  );
  return requestClient.put<PurchaseFileItem>(
    path(`account/catalog/purchase-configuration/file-uploads/${encodeURIComponent(reservation.uploadToken)}/content`),
    file,
    { headers: { 'Content-Type': file.type || 'application/octet-stream' } },
  );
}

export function getPurchaseConfigurationFile(fileToken: string) {
  return requestClient.get<PurchaseFileItem>(
    path(`account/catalog/purchase-configuration/file-uploads/${encodeURIComponent(fileToken)}`),
  );
}

export function revokePurchaseConfigurationFile(fileToken: string) {
  return requestClient.delete<PurchaseFileItem>(
    path(`account/catalog/purchase-configuration/file-uploads/${encodeURIComponent(fileToken)}`),
  );
}
