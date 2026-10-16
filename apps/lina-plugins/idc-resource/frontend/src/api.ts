import type { InterfaceConfigurationSchema } from "../pages/provider-interface-configuration";
import type { PluginHostMountContext } from "./host-context";
import type { ProviderCatalogDetail, ProviderCatalogPage, ProviderCatalogQuery } from "../pages/provider-catalog-state";
import type { CatalogImportInput, CatalogImportResult, ImportCategory } from "../pages/provider-catalog-import";

import { createRequest } from "./request";

export type SecretField = {
  key: string;
  ready: boolean;
  required: boolean;
  updatedAt?: string;
};
export type Channel = {
  interfaceInstanceNo?: string;
  catalogAvailable?: boolean;
  channelNo: string;
  connectionStatus?: string;
  connectionStatusCode?: string;
  connectionVerifiedAt?: number;
  displayName: string;
  endpoint: string;
  tlsCertificateSha256?: string;
  interfaceConfiguration?: InterfaceConfigurationSchema;
  nonSecretSettings?: Record<string, unknown>;
  providerPluginId: string;
  providerReleaseVersion?: string;
  secretFields: SecretField[];
  status: string;
  version: number;
  usage?: { activeInstances: number; totalInstances: number; configuredProducts: number };
};
export type ProviderCandidate = {
  displayName: string;
  interfaceConfiguration?: InterfaceConfigurationSchema;
  pluginId: string;
  releaseVersion: string;
};
export type Pool = {
  channelNo: string;
  channelName?: string;
  displayName: string;
  poolNo: string;
  resourceKind: string;
  routingPriority: number;
  routingWeight: number;
  status: string;
  version: number;
};

const base = "/x/idc-resource/api/v1/admin/resource/provider-configurations";
export function createAPI(context: () => PluginHostMountContext) {
  const request = createRequest(context);
  return {
    categories: () => request<{ categories: ImportCategory[] }>("/x/idc-core/api/v1/admin/catalog/categories"),
    importProduct: (body: CatalogImportInput) => request<CatalogImportResult>("/x/idc-core/api/v1/admin/catalog/product-imports", { method: "POST", body }),
    catalogProducts: (interfaceNo: string, query: ProviderCatalogQuery) => request<ProviderCatalogPage>(
      `${base}/interfaces/${encodeURIComponent(interfaceNo)}/catalog-products?${new URLSearchParams({ page: String(query.page), pageSize: String(query.pageSize), keyword: query.keyword })}`,
    ),
    catalogProduct: (interfaceNo: string, reference: string) => request<{ item: ProviderCatalogDetail }>(
      `${base}/interfaces/${encodeURIComponent(interfaceNo)}/catalog-product?${new URLSearchParams({ reference })}`,
    ),
    list: (query: { channelPage?: number; channelPageSize?: number; poolPage?: number; poolPageSize?: number; channelKeyword?: string; availableChannelsOnly?: boolean } = {}) =>
      request<{
        channelTotal: number;
        poolTotal: number;
        channels: Channel[];
        pools: Pool[];
        providerCandidates: ProviderCandidate[];
      }>(`${base}?${new URLSearchParams(Object.entries(query).map(([key, value]) => [key, String(value)]))}`),
    createChannel: (body: Record<string, unknown>) =>
      request<{ item: Channel }>(`${base}/channels`, { method: "POST", body }),
    updateChannel: (item: Channel, body: Record<string, unknown>) =>
      request<{ item: Channel }>(
        `${base}/channels/${encodeURIComponent(item.channelNo)}`,
        { method: "PUT", body: { ...body, version: item.version } },
      ),
    verifyChannel: (channelNo: string) =>
      request<{ observedAt: number; statusCode: string; verified: boolean }>(
        `${base}/channels/${encodeURIComponent(channelNo)}/connection-verifications`,
        { method: "POST" },
      ),
    createPool: (body: Record<string, unknown>) =>
      request<{ item: Pool }>(`${base}/pools`, { method: "POST", body }),
    updatePool: (item: Pool, body: Record<string, unknown>) =>
      request<{ item: Pool }>(
        `${base}/pools/${encodeURIComponent(item.poolNo)}`,
        { method: "PUT", body: { ...body, version: item.version } },
      ),
    secretAccess: (channelNo: string, body: Record<string, unknown>) =>
      request<{ item: { token: string } }>(
        `/x/idc-resource/api/v1/admin/resource/channels/${encodeURIComponent(channelNo)}/secrets/access-contexts`,
        { method: "POST", body },
      ),
    createSecret: (channelNo: string, body: Record<string, unknown>) =>
      request<unknown>(
        `/x/idc-resource/api/v1/admin/resource/channels/${encodeURIComponent(channelNo)}/secrets`,
        { method: "POST", body },
      ),
    replaceSecret: (
      channelNo: string,
      bindingName: string,
      body: Record<string, unknown>,
    ) =>
      request<unknown>(
        `/x/idc-resource/api/v1/admin/resource/channels/${encodeURIComponent(channelNo)}/secrets/${encodeURIComponent(bindingName)}`,
        { method: "PUT", body },
      ),
  };
}
