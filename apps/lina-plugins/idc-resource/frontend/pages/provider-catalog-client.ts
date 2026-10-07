import { pluginApiPath, requestClient } from "#/api/request";

import type { ProviderCatalogDetail, ProviderCatalogPage, ProviderCatalogQuery } from "./provider-catalog-state";
import type { CatalogImportInput, CatalogImportResult, ImportCategory } from "./provider-catalog-import";

const interfacePath = (interfaceNo: string) => pluginApiPath(
  "idc-resource",
  `admin/resource/provider-configurations/interfaces/${encodeURIComponent(interfaceNo)}`,
);

/** Read one supplier page; rows never trigger individual detail requests. */
export function getProviderCatalogProducts(interfaceNo: string, query: ProviderCatalogQuery) {
  return requestClient.get<ProviderCatalogPage>(`${interfacePath(interfaceNo)}/catalog-products`, { params: query });
}

/** Read prices only after an employee explicitly selects one product. */
export function getProviderCatalogProduct(interfaceNo: string, reference: string) {
  return requestClient.get<{ item: ProviderCatalogDetail }>(`${interfacePath(interfaceNo)}/catalog-product`, { params: { reference } });
}

export const providerCatalogImportClient = {
  categories: () => requestClient.get<{ categories: ImportCategory[] }>(pluginApiPath("idc-core", "admin/catalog/categories")),
  importProduct: (input: CatalogImportInput) => requestClient.post<CatalogImportResult>(pluginApiPath("idc-core", "admin/catalog/product-imports"), input),
};
