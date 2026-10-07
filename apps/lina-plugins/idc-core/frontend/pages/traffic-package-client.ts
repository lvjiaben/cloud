// Traffic package settings use the accessory owner; amounts stay in minor units.
import { pluginApiPath, requestClient } from "#/api/request";
const path = (suffix = "") => pluginApiPath("idc-core", `admin/resource/traffic-packages${suffix}`);

export interface TrafficPackage {
  packageNo: string;
  version: number;
  nameZhCN: string;
  nameEnUS: string;
  currencyCode: string;
  trafficGb: number;
  amountMinor: number;
  enabled: boolean;
  stockQuantity: number | null;
  occupiedQuantity: number;
  remainingQuantity: number | null;
  productNos: string[];
  publishedAt: number;
}
export type TrafficPackageFields = Pick<TrafficPackage, "nameZhCN" | "nameEnUS" | "currencyCode" | "trafficGb" | "amountMinor" | "enabled" | "stockQuantity" | "productNos">;
export interface TrafficProductChoice { productNo: string; nameZhCN: string; nameEnUS: string }

export function listTrafficPackages(params: { page: number; pageSize: number; keyword?: string }) {
  return requestClient.get<{ items: TrafficPackage[]; total: number }>(path(), { params });
}
export function saveTrafficPackage(input: TrafficPackageFields, previous?: TrafficPackage) {
  return previous
    ? requestClient.put<{ package: TrafficPackage }>(path(`/${encodeURIComponent(previous.packageNo)}`), { ...input, expectedVersion: previous.version })
    : requestClient.post<{ package: TrafficPackage }>(path(), input);
}
export function searchTrafficProducts(keyword: string, productNos?: string[]) {
  return requestClient.get<{ items: TrafficProductChoice[] }>(pluginApiPath("idc-core", "admin/resource/traffic-package-products"), { params: { keyword, productNos } });
}
