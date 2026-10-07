import type {
  SiteApiEnvelope,
  SiteCatalogFilters,
  SiteCatalogPage,
  SiteCatalogProduct,
  SiteGuestCartItemInput,
  SiteGuestCartReference,
} from '@lina/site-contracts';

export type SiteFetch = typeof globalThis.fetch;

export interface SiteClientOptions {
  apiBase?: string;
  fetch?: SiteFetch;
  timeoutMs?: number;
}

export class SiteApiError extends Error {
  constructor(
    readonly code: 'invalid_response' | 'request_failed' | 'request_timeout',
    readonly status?: number,
  ) {
    super(code);
    this.name = 'SiteApiError';
  }
}

export function createSiteClient(options: SiteClientOptions = {}) {
  const apiBase = normalizeBase(
    options.apiBase ?? '/x/idc-core/api/v1',
  );
  const fetcher = options.fetch ?? globalThis.fetch;
  const timeoutMs = options.timeoutMs ?? 8_000;

  return {
    async addGuestCartItem(
      input: SiteGuestCartItemInput,
      signal?: AbortSignal,
    ): Promise<SiteGuestCartReference> {
      const response = await request<{ cart: SiteGuestCartReference }>(
        `${apiBase}/public/commerce/cart-items`,
        fetcher,
        timeoutMs,
        signal,
        jsonRequest(input),
      );
      return response.cart;
    },

    async getCatalogProduct(
      publicSlug: string,
      signal?: AbortSignal,
    ): Promise<SiteCatalogProduct> {
      const response = await request<{ product: SiteCatalogProduct }>(
        `${apiBase}/public/catalog/products/${encodeURIComponent(publicSlug)}`,
        fetcher,
        timeoutMs,
        signal,
      );
      return response.product;
    },

    listCatalogProducts(
      filters: SiteCatalogFilters = {},
      signal?: AbortSignal,
    ): Promise<SiteCatalogPage> {
      const query = new URLSearchParams();
      for (const [key, value] of Object.entries(filters)) {
        if (value !== undefined && value !== '') {
          query.set(key, String(value));
        }
      }
      if (!query.has('limit')) query.set('limit', '12');
      return request<SiteCatalogPage>(
        `${apiBase}/public/catalog/products?${query.toString()}`,
        fetcher,
        timeoutMs,
        signal,
      );
    },

    async openGuestCart(
      currencyCode: string,
      signal?: AbortSignal,
    ): Promise<SiteGuestCartReference> {
      const response = await request<{ cart: SiteGuestCartReference }>(
        `${apiBase}/public/commerce/carts`,
        fetcher,
        timeoutMs,
        signal,
        jsonRequest({ currencyCode }),
      );
      return response.cart;
    },
  };
}

function normalizeBase(value: string) {
  return value.replace(/\/+$/, '');
}

async function request<T>(
  url: string,
  fetcher: SiteFetch,
  timeoutMs: number,
  outerSignal?: AbortSignal,
  init: RequestInit = {},
): Promise<T> {
  const controller = new AbortController();
  const timeout = globalThis.setTimeout(() => controller.abort(), timeoutMs);
  const cancel = () => controller.abort();
  outerSignal?.addEventListener('abort', cancel, { once: true });

  try {
    const response = await fetcher(url, {
      ...init,
      credentials: init.credentials ?? 'same-origin',
      headers: { Accept: 'application/json', ...init.headers },
      signal: controller.signal,
    });
    if (!response.ok) {
      throw new SiteApiError('request_failed', response.status);
    }
    const payload = (await response.json()) as SiteApiEnvelope<T> | T;
    if (isEnvelope(payload)) {
      if (
        payload.data === undefined ||
        (payload.code !== undefined && String(payload.code) !== '0')
      ) {
        throw new SiteApiError('invalid_response', response.status);
      }
      return payload.data;
    }
    return payload;
  } catch (error) {
    if (error instanceof SiteApiError) throw error;
    if (controller.signal.aborted) {
      throw new SiteApiError('request_timeout');
    }
    throw new SiteApiError('request_failed');
  } finally {
    globalThis.clearTimeout(timeout);
    outerSignal?.removeEventListener('abort', cancel);
  }
}

function jsonRequest(value: unknown): RequestInit {
  return {
    body: JSON.stringify(value),
    headers: { 'Content-Type': 'application/json' },
    method: 'POST',
  };
}

function isEnvelope<T>(value: SiteApiEnvelope<T> | T): value is SiteApiEnvelope<T> {
  return typeof value === 'object' && value !== null && 'data' in value;
}
