import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-compliance";
const providerPath = "admin/compliance/providers/aliyun-face";

export type ProviderConfigurationStatus =
  | "configured"
  | "disabled"
  | "draft"
  | "updating";

export type ProviderValidationStatus =
  | "reachable"
  | "rejected"
  | "unavailable"
  | "untested";

export interface ProviderConfiguration {
  credentialConfigured: boolean;
  credentialGeneration: number;
  credentialMaskedTail: string;
  providerCode: "aliyun_face";
  sceneId: string;
  status: ProviderConfigurationStatus;
  updatedAt: null | number;
  validatedAt: null | number;
  validationStatus: ProviderValidationStatus;
  version: number;
}

export interface ProviderConfigurationUpdateInput {
  accessKeyId: string;
  accessKeySecret: string;
  expectedVersion: number;
  idempotencyKey: string;
  sceneId: string;
}

export function providerConfigurationGet() {
  return requestClient.get<{ item: ProviderConfiguration }>(
    pluginApiPath(pluginID, providerPath),
  );
}

export function providerConfigurationUpdate(
  input: ProviderConfigurationUpdateInput,
) {
  return requestClient.put<{ item: ProviderConfiguration }>(
    pluginApiPath(pluginID, providerPath),
    input,
  );
}

export function providerConfigurationRecover(input: {
  accessKeyId: string;
  accessKeySecret: string;
}) {
  return requestClient.post<{
    item: ProviderConfiguration;
    reusedExistingMutation: boolean;
  }>(pluginApiPath(pluginID, `${providerPath}/recoveries`), input);
}

export function providerConfigurationValidate() {
  return requestClient.post<{ item: ProviderConfiguration }>(
    pluginApiPath(pluginID, `${providerPath}/validations`),
  );
}
