// Merchant-owned customer directory API; product identifiers never imply customer access.
import { pluginApiPath, requestClient } from "#/api/request";
export interface ProductDirectory {
 pagePluginIds?: string[];
 parentDirectoryNo?: string;
 navigationType: "product" | "system" | "custom" | "plugin";
 targetUrl: string;
 icon: string;
  directoryNo: string;
  nameZhCn: string;
  nameEnUs: string;
  sortOrder: number;
  statusCode: "enabled" | "disabled";
  version: number;
  productCount: number;
}
export interface DirectoryInput {
 pagePluginIds?: string[];
 parentDirectoryNo?: string;
 navigationType?: ProductDirectory["navigationType"];
 targetUrl?: string;
 icon?: string;
  nameZhCn: string;
  nameEnUs: string;
  sortOrder: number;
}
export function listProductDirectories(page = 1, pageSize = 20, tree = false) {
  return requestClient.get<{ items: ProductDirectory[]; parents?: ProductDirectory[]; total: number }>(
    pluginApiPath("idc-core", "admin/product-directories"),
    { params: { page, pageSize, tree } },
  );
}
// Product forms need the full candidate set rather than only the first grid page.
export async function listProductDirectoryOptions() {
  const first = await listProductDirectories(1, 100);
  const items = [...first.items];
  for (let page = 2; items.length < first.total; page++) {
    const next = await listProductDirectories(page, 100);
    if (next.items.length === 0) break;
    items.push(...next.items);
  }
  return items;
}

export function createProductDirectory(input: DirectoryInput) {
  return requestClient.post(
    pluginApiPath("idc-core", "admin/product-directories"),
    input,
  );
}
export function updateProductDirectory(
  directoryNo: string,
  input: DirectoryInput & {
    expectedVersion: number;
    statusCode: ProductDirectory["statusCode"];
  },
) {
  return requestClient.put<{directory:ProductDirectory}>(
    pluginApiPath(
      "idc-core",
      `admin/product-directories/${encodeURIComponent(directoryNo)}`,
    ),
    input,
  );
}
