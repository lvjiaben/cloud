import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-compliance";

export type SupplierProviderCode = "nuonuo_invoice" | "tencent_essbasic";

export interface SupplierConfiguration {
  adapterEnabled: boolean;
  capabilityEvidenceCode:
    | "external_confirmation_required"
    | "official_public_contract";
  credentialConfigured: boolean;
  credentialGeneration: number;
  credentialMaskedTail: string;
  credentialStatus: string;
  lastOperationAt: null | number;
  lastOperationKind: string;
  lastOperationNo: string;
  lastOperationResult: string;
  providerCode: SupplierProviderCode;
  subjectBindingNo: string;
  subjectStatus: string;
  updatedAt: null | number;
  version: number;
}

function supplierPath(providerCode: SupplierProviderCode) {
  return `admin/compliance/suppliers/${providerCode}`;
}

export function supplierConfigurationGet(providerCode: SupplierProviderCode) {
  return requestClient.get<{ item: SupplierConfiguration }>(
    pluginApiPath(pluginID, supplierPath(providerCode)),
  );
}

export function supplierCredentialUpdate(
  providerCode: SupplierProviderCode,
  input: {
    credentialEnvelope: string;
    expectedVersion: number;
    idempotencyKey: string;
  },
) {
  return requestClient.put<{ item: SupplierConfiguration }>(
    pluginApiPath(pluginID, `${supplierPath(providerCode)}/credential`),
    input,
  );
}
