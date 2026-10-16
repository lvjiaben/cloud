// This client reads only the anonymous bounded catalog contract.
import { publicRequestHeaders, resolvePublicApiURL } from "./request-context";

export interface CatalogCategory {
  categoryNo: string;
  nameEnUS: string;
  nameZhCN: string;
}

export interface CatalogProduct {
  availability: "available" | "manual_confirmation" | "unavailable";
  categoryNameEnUS: string;
  categoryNameZhCN: string;
  categoryNo: string;
  currencies: string[];
  nameEnUS: string;
  nameZhCN: string;
  pricingPresentation: "base_price";
  publicSlug: string;
  summaryEnUS: string;
  summaryZhCN: string;
}

export interface CatalogPage {
  availableCurrencies: string[];
  categories: CatalogCategory[];
  nextCursor: string;
  products: CatalogProduct[];
}

export interface CatalogFilters {
  categoryNo?: string;
  currency?: string;
  cursor?: string;
  namePrefix?: string;
}

export interface PublicCatalogCampaign {
  campaignNo: string;
  campaignType: string;
  descriptionEnUS: string;
  descriptionZhCN: string;
  endsAt: number;
  nameEnUS: string;
  nameZhCN: string;
  startsAt: number;
}

interface ResponseEnvelope<T> {
  code?: number | string;
  data?: T;
  message?: string;
}

const catalogBase = "/x/idc-core/api/v1/public/catalog/products";

function normalizeCatalogProduct(product: CatalogProduct): CatalogProduct {
  return {
    ...product,
    currencies: Array.isArray(product.currencies) ? product.currencies : [],
  };
}

export async function listCatalogProducts(
  filters: CatalogFilters,
): Promise<CatalogPage> {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value) query.set(key, value);
  }
  query.set("limit", "12");
  const response = await requestCatalog<CatalogPage>(
    `${catalogBase}?${query.toString()}`,
  );
  return {
    availableCurrencies: Array.isArray(response?.availableCurrencies)
      ? response.availableCurrencies
      : [],
    categories: Array.isArray(response?.categories) ? response.categories : [],
    nextCursor:
      typeof response?.nextCursor === "string" ? response.nextCursor : "",
    products: Array.isArray(response?.products)
      ? response.products.map(normalizeCatalogProduct)
      : [],
  };
}

export async function getCatalogProduct(slug: string): Promise<CatalogProduct> {
  const response = await requestCatalog<{ product: CatalogProduct }>(
    `${catalogBase}/${encodeURIComponent(slug)}`,
  );
  if (!response?.product || typeof response.product !== "object") {
    throw new Error("catalog_request_failed");
  }
  return normalizeCatalogProduct(response.product);
}

export async function listPublicCatalogCampaigns(
  slug: string,
  currencyCode: string,
): Promise<PublicCatalogCampaign[]> {
  const query = new URLSearchParams({ currencyCode });
  const response = await requestCatalog<{ campaigns: PublicCatalogCampaign[] }>(
    `${catalogBase}/${encodeURIComponent(slug)}/campaigns?${query.toString()}`,
  );
  return Array.isArray(response?.campaigns) ? response.campaigns : [];
}

export async function recordPublicCatalogCampaignExposure(
  slug: string,
  campaignNo: string,
  currencyCode: string,
  deviceId: string,
): Promise<{ recorded: boolean }> {
  return requestCatalog<{ recorded: boolean }>(
    `${catalogBase}/${encodeURIComponent(slug)}/campaign-exposures`,
    {
      body: JSON.stringify({ campaignNo, currencyCode, deviceId }),
      headers: { "Content-Type": "application/json" },
      method: "POST",
    },
  );
}

async function requestCatalog<T>(
  url: string,
  init: RequestInit = {},
): Promise<T> {
  const controller = new AbortController();
  const timeout = globalThis.setTimeout(() => controller.abort(), 8_000);
  try {
    const response = await fetch(resolvePublicApiURL(url), {
      ...init,
      headers: {
        ...publicRequestHeaders(),
        Accept: "application/json",
        ...init.headers,
      },
      signal: controller.signal,
    });
    const payload = (await response.json()) as ResponseEnvelope<T> | T;
    if (!response.ok) throw new Error("catalog_request_failed");
    if (typeof payload === "object" && payload !== null && "data" in payload) {
      const envelope = payload as ResponseEnvelope<T>;
      if (
        envelope.data === undefined ||
        (envelope.code !== undefined && `${envelope.code}` !== "0")
      ) {
        throw new Error("catalog_request_failed");
      }
      return envelope.data;
    }
    return payload as T;
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new Error("catalog_request_timeout");
    }
    throw error;
  } finally {
    globalThis.clearTimeout(timeout);
  }
}
