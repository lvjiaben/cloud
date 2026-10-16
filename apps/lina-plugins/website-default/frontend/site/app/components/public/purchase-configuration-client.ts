// This client exchanges only opaque field tokens and explicit typed values with
// the governed public product-fulfillment configuration boundary.
import {
  publicRequestHeaders,
  resolvePublicApiURL,
} from "./request-context";

export type PurchaseConfigurationFieldType =
  | "address"
  | "boolean"
  | "date"
  | "datetime"
  | "decimal"
  | "file"
  | "integer"
  | "multi_select"
  | "multi_line"
  | "single_select"
  | "single_line"
  | "tree_select";

export interface PurchaseConfigurationOption {
  children?: PurchaseConfigurationOption[];
  labelEnUS: string;
  labelZhCN: string;
  optionToken: string;
}

export interface PurchaseConfigurationUploadPolicy {
  allowedExtensions: string[];
  allowedMimeTypes: string[];
  maximumFileBytes: number;
  maximumFiles: number;
  maximumTotalBytes: number;
  safetyScanRequired: boolean;
}

export interface PurchaseConfigurationField {
  acceptedFileTypes?: string[];
  descriptionEnUS: string;
  descriptionZhCN: string;
  disabled: boolean;
  fieldToken: string;
  format?: "domain" | "email" | "phone" | "url";
  labelEnUS: string;
  labelZhCN: string;
  maximumDecimal?: string;
  maximumFiles?: number;
  maximumInteger?: string;
  maximumLength?: number;
  maximumSelections?: number;
  minimumDecimal?: string;
  minimumInteger?: string;
  minimumLength?: number;
  minimumSelections?: number;
  options?: PurchaseConfigurationOption[];
  required: boolean;
  type: PurchaseConfigurationFieldType;
  uploadPolicy?: PurchaseConfigurationUploadPolicy;
  visible: boolean;
}

export interface PurchaseConfigurationValue {
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
  type: PurchaseConfigurationFieldType;
}

export interface PurchaseConfigurationView {
  configurationRevision: string;
  fields: PurchaseConfigurationField[];
  values: PurchaseConfigurationValue[];
}

export interface PurchaseConfigurationIssue {
  fieldToken?: string;
  issueCode: string;
}

export interface PurchaseConfigurationEvaluation extends PurchaseConfigurationView {
  issues: PurchaseConfigurationIssue[];
  valid: boolean;
}

export interface PurchaseConfigurationQuoteTerm {
  billingTermToken: string;
  labelEnUS: string;
  labelZhCN: string;
  minimumPurchaseCycles: number;
  setupAmount: string;
  status: "final" | "requires_customer_context";
  subtotalAmount: string;
  taxAmount: string;
  totalAmount: string;
}

export interface PurchaseConfigurationQuote {
  configurationRevision: string;
  currencyCode: string;
  terms: PurchaseConfigurationQuoteTerm[];
}

export interface GuestCartReference {
  reference: string;
}

interface ResponseEnvelope<T> {
  code?: number | string;
  data?: T;
}

const catalogBase = "/x/idc-core/api/v1/public/catalog/products";

export async function getPurchaseConfiguration(publicSlug: string) {
  return requestPurchaseConfiguration<PurchaseConfigurationView>(
    `${catalogBase}/${encodeURIComponent(publicSlug)}/purchase-configuration`,
  );
}

export async function evaluatePurchaseConfiguration(
  publicSlug: string,
  configurationRevision: string,
  values: PurchaseConfigurationValue[],
) {
  return requestPurchaseConfiguration<PurchaseConfigurationEvaluation>(
    `${catalogBase}/${encodeURIComponent(publicSlug)}/purchase-configuration/evaluate`,
    {
      body: JSON.stringify({ configurationRevision, values }),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    },
  );
}

export async function quotePurchaseConfiguration(
  publicSlug: string,
  configurationRevision: string,
  currencyCode: string,
  values: PurchaseConfigurationValue[],
) {
  return requestPurchaseConfiguration<PurchaseConfigurationQuote>(
    `${catalogBase}/${encodeURIComponent(publicSlug)}/purchase-configuration/quote`,
    {
      body: JSON.stringify({ configurationRevision, currencyCode, values }),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    },
  );
}

export async function openGuestCart(currencyCode: string) {
  return requestPurchaseConfiguration<{ cart: GuestCartReference }>(
    "/x/idc-core/api/v1/public/commerce/carts",
    {
      body: JSON.stringify({ currencyCode }),
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      method: "POST",
    },
  );
}

export async function addGuestCartItem(input: {
  billingTermToken: string;
  configurationRevision: string;
  currencyCode: string;
  publicSlug: string;
  purchaseCycles: number;
  quantity: number;
  values: PurchaseConfigurationValue[];
}) {
  return requestPurchaseConfiguration<{ cart: GuestCartReference }>(
    "/x/idc-core/api/v1/public/commerce/cart-items",
    {
      body: JSON.stringify(input),
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      method: "POST",
    },
  );
}

async function requestPurchaseConfiguration<T>(
  url: string,
  init: RequestInit = {},
): Promise<T> {
  const controller = new AbortController();
  const timeout = globalThis.setTimeout(() => controller.abort(), 8_000);
  try {
    const response = await fetch(resolvePublicApiURL(url), {
      ...init,
      credentials: init.credentials ?? "same-origin",
      headers: {
        ...publicRequestHeaders(),
        Accept: "application/json",
        ...init.headers,
      },
      signal: controller.signal,
    });
    const payload = (await response.json()) as ResponseEnvelope<T> | T;
    if (!response.ok) throw new Error("purchase_configuration_request_failed");
    if (typeof payload === "object" && payload !== null && "data" in payload) {
      const envelope = payload as ResponseEnvelope<T>;
      if (
        envelope.data === undefined ||
        (envelope.code !== undefined && `${envelope.code}` !== "0")
      ) {
        throw new Error("purchase_configuration_request_failed");
      }
      return envelope.data;
    }
    return payload as T;
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new Error("purchase_configuration_request_timeout");
    }
    throw error;
  } finally {
    globalThis.clearTimeout(timeout);
  }
}
