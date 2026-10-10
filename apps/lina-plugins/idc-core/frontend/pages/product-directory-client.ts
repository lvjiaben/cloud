// Merchant-owned customer directory API; product identifiers never imply customer access.
import { pluginApiPath, requestClient } from "#/api/request";
export interface ProductDirectory {
  directoryNo: string;
  nameZhCn: string;
  nameEnUs: string;
  sortOrder: number;
  statusCode: "enabled" | "disabled";
  version: number;
  productNos: string[];
  productCount: number;
  products: Array<{ productNo: string; nameZhCn: string; nameEnUs: string }>;
}
export interface DirectoryInput {
  nameZhCn: string;
  nameEnUs: string;
  sortOrder: number;
  productNos: string[];
}
export function listProductDirectories(page = 1, pageSize = 20) {
  return requestClient.get<{ items: ProductDirectory[]; total: number }>(
    pluginApiPath("idc-core", "admin/product-directories"),
    { params: { page, pageSize } },
  );
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
  return requestClient.put(
    pluginApiPath(
      "idc-core",
      `admin/product-directories/${encodeURIComponent(directoryNo)}`,
    ),
    input,
  );
}
