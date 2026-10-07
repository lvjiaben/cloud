import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./provider-request";

const pluginID = "idc-resource";

export interface ResourceOverviewPool {
  channelName: string;
  channelNo: string;
  displayName: string;
  poolNo: string;
  resourceKind: string;
  routingPriority: number;
  routingWeight: number;
  status: string;
  updatedAt: number;
}

export interface ResourceFinanceFact {
  amount: string;
  channelName: string;
  channelNo: string;
  currencyCode: string;
  expiresAt: null | number;
  externalResourceRef: string;
  kind: "balance" | "cost" | "expiry";
  observedAt: number;
  snapshotNo: string;
}

export interface ResourceOverviewSummary {
	poolTotal: number;
	financeTotal: number;
  activeMappings: number;
  enabledChannels: number;
  enabledPools: number;
  finance: ResourceFinanceFact[];
  latestProviderCapacityAt: null | number;
  pendingTasks: number;
  pools: ResourceOverviewPool[];
  unknownTasks: number;
}

export function getResourceOverview(params: {poolPage: number; poolPageSize: number; financePage: number; financePageSize: number}) {
  return requestPreservingHttpStatus<{ summary: ResourceOverviewSummary }>(requestClient,
    pluginApiPath(pluginID, "admin/resource/overview"),
	{ method: "GET", params },
  );
}
