// Customer traffic packages are scoped by the original owned server.
import { pluginApiPath, requestClient } from '#/api/request';

export interface TrafficPackage {
  packageNo: string;
  nameZhCN: string;
  nameEnUS: string;
  trafficGb: number;
  currencyCode: string;
  amountMinor: number;
  available: boolean;
}

export function listTrafficPackages(serviceNo: string, params: { page: number; pageSize: number; keyword?: string }) {
  return requestClient.get<{ items: TrafficPackage[]; total: number }>(
    pluginApiPath('idc-core', `account/servers/${encodeURIComponent(serviceNo)}/traffic-packages`), { params },
  );
}
