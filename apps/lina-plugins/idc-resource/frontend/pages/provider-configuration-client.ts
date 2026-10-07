import { requestPreservingHttpStatus } from "./provider-request";


import { pluginApiPath, requestClient } from "#/api/request";

import type { InterfaceConfigurationSchema } from "./provider-interface-configuration";

export interface ProviderSecretFieldStatus {
  key: string;
  ready: boolean;
  required: boolean;
  updatedAt?: string;
}

export interface ProviderChannelUsage {
  activeInstances: number;
  totalInstances: number;
  configuredProducts: number;
}

export interface ProviderConfigurationQuery {
  channelPage?: number;
  channelPageSize?: number;
  poolPage?: number;
  poolPageSize?: number;
  channelKeyword?: string;
  availableChannelsOnly?: boolean;
}

export interface ProviderChannel {
  interfaceInstanceNo?: string;
  catalogAvailable?: boolean;
  usage?: ProviderChannelUsage;
  channelNo: string;
  connectionStatus?: string;
  connectionStatusCode?: string;
  connectionVerifiedAt?: number;
  displayName: string;
  endpoint: string;
  interfaceConfiguration?: InterfaceConfigurationSchema;
  nonSecretSettings?: Record<string, unknown>;
  providerPluginId: string;
  providerReleaseVersion?: string;
  secretFields: ProviderSecretFieldStatus[];
  status: string;
  version: number;
}

export interface ProviderChannelVerification {
  observedAt: number;
  statusCode: string;
  verified: boolean;
}

export interface ProviderCandidate {
  displayName: string;
  pluginId: string;
  providerId: string;
  releaseVersion: string;
  interfaceConfiguration?: InterfaceConfigurationSchema;
}

export interface ProviderPool {
  channelName?: string;
  channelNo: string;
  displayName: string;
  poolNo: string;
  resourceKind: string;
  routingPriority: number;
  routingWeight: number;
  status: string;
  version: number;
}

const base = () =>
  pluginApiPath("idc-resource", "admin/resource/provider-configurations");

export function getProviderConfigurations(params: ProviderConfigurationQuery = {}) {
  return requestPreservingHttpStatus<{
    channelTotal: number;
    poolTotal: number;
    channels: ProviderChannel[];
    pools: ProviderPool[];
    providerCandidates: ProviderCandidate[];
  }>(requestClient, base(), { method: "GET", params });
}

export function createProviderChannel(body: Record<string, unknown>) {
  return requestClient.post<{ item: ProviderChannel }>(
    `${base()}/channels`,
    body,
  );
}

export function updateProviderChannel(
  item: ProviderChannel,
  body: Record<string, unknown>,
) {
  return requestClient.put<{ item: ProviderChannel }>(
    `${base()}/channels/${encodeURIComponent(item.channelNo)}`,
    { ...body, version: item.version },
  );
}

export function verifyProviderChannel(channelNo: string) {
  return requestClient.post<ProviderChannelVerification>(
    `${base()}/channels/${encodeURIComponent(channelNo)}/connection-verifications`,
  );
}

export function createProviderPool(body: Record<string, unknown>) {
  return requestClient.post<{ item: ProviderPool }>(`${base()}/pools`, body);
}

export function updateProviderPool(
  item: ProviderPool,
  body: Record<string, unknown>,
) {
  return requestClient.put<{ item: ProviderPool }>(
    `${base()}/pools/${encodeURIComponent(item.poolNo)}`,
    { ...body, version: item.version },
  );
}

export function createProviderSecretAccess(
  channelNo: string,
  body: Record<string, unknown>,
) {
  return requestClient.post<{ item: { token: string } }>(
    pluginApiPath(
      "idc-resource",
      `admin/resource/channels/${encodeURIComponent(channelNo)}/secrets/access-contexts`,
    ),
    body,
  );
}

export function createProviderSecret(
  channelNo: string,
  body: Record<string, unknown>,
) {
  return requestClient.post(
    pluginApiPath(
      "idc-resource",
      `admin/resource/channels/${encodeURIComponent(channelNo)}/secrets`,
    ),
    body,
  );
}

export function replaceProviderSecret(
  channelNo: string,
  bindingName: string,
  body: Record<string, unknown>,
) {
  return requestClient.put(
    pluginApiPath(
      "idc-resource",
      `admin/resource/channels/${encodeURIComponent(channelNo)}/secrets/${encodeURIComponent(bindingName)}`,
    ),
    body,
  );
}
