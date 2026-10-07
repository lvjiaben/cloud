import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./provider-request";

const pluginID = "idc-resource";

export type ProviderHealthCode =
  | "degraded"
  | "healthy"
  | "unavailable"
  | "unknown";

export interface ProviderHealthObservation {
  healthCode: ProviderHealthCode;
  latencyMs: number;
  observedAt: null | number;
  summaryCode: string;
}

export interface ProviderHealthPolicy {
  consecutiveFailureLimit: number;
  failureLimit: number;
  minimumSamplesToProtect: number;
  protectionDurationSeconds: number;
  unknownResultLimit: number;
  updatedAt: null | number;
  version: number;
  windowSampleLimit: number;
}

export interface ProviderChannelHealth {
  authentication: ProviderHealthObservation;
  channelName: string;
  channelNo: string;
  channelStatus: string;
  circuitOpen: boolean;
  consecutiveFailures: number;
  failureCount: number;
  healthCode: ProviderHealthCode;
  providerCapacity: ProviderHealthObservation;
  lastObservedAt: null | number;
  lastSummaryCode: string;
  policy: ProviderHealthPolicy;
  providerId: string;
  query: ProviderHealthObservation;
  sampleCount: number;
  unknownCount: number;
  write: ProviderHealthObservation;
  writeProtectedUntil: null | number;
}

export interface ProviderHealthListResult {
	total: number;
  items: ProviderChannelHealth[];
  nextCursor: string;
}

export type ProviderHealthPolicyUpdate = Omit<
  ProviderHealthPolicy,
  "updatedAt" | "version"
> & { expectedVersion: number };

export function providerHealthList(params?: {
	page?: number;
  channelNo?: string;
  cursor?: string;
  limit?: number;
}) {
  return requestPreservingHttpStatus<ProviderHealthListResult>(requestClient,
    pluginApiPath(pluginID, "admin/resource/channel-health"),
    { method: "GET", params },
  );
}

export function providerHealthPolicyUpdate(
  channelNo: string,
  input: ProviderHealthPolicyUpdate,
) {
  return requestClient.put<{ item: ProviderChannelHealth }>(
    pluginApiPath(
      pluginID,
      `admin/resource/channels/${encodeURIComponent(channelNo)}/health-policy`,
    ),
    input,
  );
}
