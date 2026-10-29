import { pluginApiPath, requestClient } from "#/api/request";

import type {
  CatalogConfigurationCandidate,
  CatalogConfigurationDeclaration,
  CatalogConfigurationEvaluation,
  CatalogStructuredConfigurationValue,
} from "./catalog-configuration-state";

const pluginID = "idc-core";

export type AutomationInterfaceNewBusinessState = "accepting" | "stopped";

export interface AutomationProviderCandidate {
  displayName: string;
  interfaceConfiguration?: CatalogConfigurationDeclaration;
  selectionToken: string;
}

export interface AutomationInterface {
  catalogAvailable?: boolean;
  canDelete: boolean;
  changedAt: number;
  createdAt: number;
  displayName: string;
  interfaceInstanceNo: string;
  newBusinessState: AutomationInterfaceNewBusinessState;
  version: number;
}

export interface ProductFulfillmentBinding {
  catalogAvailable?: boolean;
  boundAt: number;
  evidenceVerified: boolean;
  interfaceDisplayName: string;
  interfaceInstanceNo: string;
  interfaceNewBusinessState: AutomationInterfaceNewBusinessState;
}

export interface AutomationInterfaceListParams {
  forProductBinding?: boolean;
  namePrefix?: string;
  newBusinessState?: "" | AutomationInterfaceNewBusinessState;
  page?: number;
  pageSize?: number;
  providerSelectionToken?: string;
}

export function automationProviderList(page = 1, pageSize = 100) {
  return requestClient.get<{
    items: AutomationProviderCandidate[];
    total: number;
  }>(pluginApiPath(pluginID, "admin/catalog/automation/providers"), {
    params: { page, pageSize },
  });
}

export function automationInterfaceList(
  params?: AutomationInterfaceListParams,
) {
  return requestClient.get<{ items: AutomationInterface[]; total: number }>(
    pluginApiPath(pluginID, "admin/catalog/automation/interfaces"),
    { params },
  );
}

export function automationInterfaceCreate(input: {
  configuration: CatalogStructuredConfigurationValue[];
  displayName: string;
  selectionToken: string;
}) {
  return requestClient.post<{ interface: AutomationInterface }>(
    pluginApiPath(pluginID, "admin/catalog/automation/interfaces"),
    input,
  );
}

export interface AutomationInterfaceConfiguration {
  declaration?: CatalogConfigurationDeclaration;
  resolvedCandidates: Record<string, CatalogConfigurationCandidate[]>;
  revision: number;
  secrets: Array<{
    configured: boolean;
    fieldKey: string;
    maskedTail: string;
    updatedAt: number;
  }>;
  values: CatalogStructuredConfigurationValue[];
}

export function automationInterfaceConfigurationEvaluate(
  interfaceInstanceNo: string,
  input: {
    expectedRevision: number;
    values: CatalogStructuredConfigurationValue[];
  },
) {
  return requestClient.post<{
    evaluation: CatalogConfigurationEvaluation;
    revision: number;
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/automation/interfaces/${encodeURIComponent(interfaceInstanceNo)}/configuration/evaluation`,
    ),
    input,
  );
}

export function automationInterfaceConfigurationGet(
  interfaceInstanceNo: string,
) {
  return requestClient.get<{
    configuration: AutomationInterfaceConfiguration;
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/automation/interfaces/${encodeURIComponent(interfaceInstanceNo)}/configuration`,
    ),
  );
}

export function automationInterfaceConfigurationUpdate(
  interfaceInstanceNo: string,
  input: {
    expectedRevision: number;
    values: CatalogStructuredConfigurationValue[];
  },
) {
  return requestClient.put<{
    configuration: AutomationInterfaceConfiguration;
  }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/automation/interfaces/${encodeURIComponent(interfaceInstanceNo)}/configuration`,
    ),
    input,
  );
}

export interface ManagementConfigurationFileItem {
  expiresAt: number;
  fileName: string;
  fileToken: string;
  mediaType: string;
  sizeBytes: number;
  status: string;
}
export async function automationInterfaceConfigurationFileUpload(
  interfaceInstanceNo: string,
  targetRevision: number,
  fieldKey: string,
  file: File,
) {
  const base = pluginApiPath(
    pluginID,
    `admin/catalog/automation/interfaces/${encodeURIComponent(interfaceInstanceNo)}/configuration/file-uploads`,
  );
  const reservation = await requestClient.post<ManagementConfigurationFileItem>(
    base,
    { fieldKey, fileName: file.name, sizeBytes: file.size, targetRevision },
  );
  return requestClient.put<ManagementConfigurationFileItem>(
    `${base}/${encodeURIComponent(reservation.fileToken)}/content`,
    file,
    {
      headers: { "Content-Type": file.type || "application/octet-stream" },
      params: { targetRevision },
    },
  );
}
export function automationInterfaceConfigurationFileRevoke(
  interfaceInstanceNo: string,
  targetRevision: number,
  fileToken: string,
) {
  return requestClient.delete<ManagementConfigurationFileItem>(
    pluginApiPath(
      pluginID,
      `admin/catalog/automation/interfaces/${encodeURIComponent(interfaceInstanceNo)}/configuration/file-uploads/${encodeURIComponent(fileToken)}`,
    ),
    { params: { targetRevision } },
  );
}

export function automationInterfaceRename(
  interfaceInstanceNo: string,
  input: { displayName: string; expectedVersion: number },
) {
  return requestClient.put<{ interface: AutomationInterface }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/automation/interfaces/${encodeURIComponent(interfaceInstanceNo)}`,
    ),
    input,
  );
}

export function automationInterfaceSetNewBusinessState(
  interfaceInstanceNo: string,
  input: {
    expectedVersion: number;
    newBusinessState: AutomationInterfaceNewBusinessState;
  },
) {
  return requestClient.put<{ interface: AutomationInterface }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/automation/interfaces/${encodeURIComponent(interfaceInstanceNo)}/new-business-state`,
    ),
    input,
  );
}

export function automationInterfaceDelete(
  interfaceInstanceNo: string,
  expectedVersion: number,
) {
  return requestClient.delete(
    pluginApiPath(
      pluginID,
      `admin/catalog/automation/interfaces/${encodeURIComponent(interfaceInstanceNo)}`,
    ),
    { params: { expectedVersion } },
  );
}

export function productFulfillmentBindingPut(
  versionNo: string,
  interfaceInstanceNo: string,
) {
  return requestClient.put<{ binding: ProductFulfillmentBinding }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/fulfillment-binding`,
    ),
    { interfaceInstanceNo },
  );
}
