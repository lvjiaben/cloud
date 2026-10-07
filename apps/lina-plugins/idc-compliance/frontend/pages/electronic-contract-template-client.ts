import { requestPreservingHttpStatus } from "./compliance-request";
import type { RequestClientConfig } from "@vben/request";
import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-compliance";
const templatePath = "admin/compliance/electronic-contract/templates";

export type ContractTemplateOrigin = "local" | "tencent_synced";
export type ContractTemplateLanguage = "" | "en-US" | "zh-CN";
export type ContractTemplateScope =
  | ""
  | "all_customers"
  | "enterprise"
  | "personal";
export type ContractTemplateSigningOrder =
  | ""
  | "customer_first"
  | "merchant_first"
  | "parallel";
export type ContractTemplateStatus =
  | "disabled"
  | "draft"
  | "enabled"
  | "needs_configuration"
  | "provider_disabled"
  | "provider_offline"
  | "submission_unavailable"
  | "submitted";

export interface ElectronicContractTemplate {
  languageCode: ContractTemplateLanguage;
  lockVersion: number;
  name: string;
  originCode: ContractTemplateOrigin;
  providerState?: "active" | "disabled" | "offline";
  providerVersion?: string;
  scopeCode: ContractTemplateScope;
  signingOrderCode: ContractTemplateSigningOrder;
  statusCode: ContractTemplateStatus;
  templateNo: string;
  updatedAt: number;
  usedCount: number;
  versionNo: number;
}

export interface ContractTemplateListQuery {
 pageSize?: number;
  name?: string;
  page: number;
  scopeCode?: ContractTemplateScope;
  statusCode?: ContractTemplateStatus | "";
}

export interface ContractTemplateList {
  items: ElectronicContractTemplate[];
  page: number;
  pageSize: number;
  total: number;
}

export interface ContractTemplateSyncResult {
  markedOffline: number;
  newVersions: number;
  providerTotal: number;
  syncedAt: number;
}

export type ContractTemplateSourceMode = "online_editor" | "uploaded_file";
export type ContractTemplateFieldType =
  | "enterprise_seal"
  | "signature"
  | "signing_date"
  | "text_variable";
export type ContractTemplateParty = "customer" | "merchant";
export type ContractTemplateVariable =
  | "amount"
  | "contract_no"
  | "currency"
  | "customer_name"
  | "merchant_name"
  | "order_no"
  | "service_name"
  | "service_term"
  | "signing_date";
export type ContractTemplateProviderValue =
  | "contract_no"
  | "currency"
  | "customer_name"
  | "merchant_name"
  | "order_no"
  | "service_lines"
  | "signing_date"
  | "total_amount";

export interface ContractTemplateContentRevision {
  contentRevisionNo: string;
  conversionStatusCode: "ready" | "source_saved";
  createdAt: number;
  pdfByteSize?: number;
  pdfPageCount?: number;
  pdfSha256?: string;
  revisionNo: number;
  sourceByteSize: number;
  sourceMediaType: string;
  sourceModeCode: ContractTemplateSourceMode;
  sourceSha256: string;
}

export interface ContractTemplateField {
  fieldNo?: string;
  fieldTypeCode: ContractTemplateFieldType;
  heightPPM: number;
  pageNo: number;
  partyCode: ContractTemplateParty;
  positionXPPM: number;
  positionYPPM: number;
  sequenceNo?: number;
  variableCode?: ContractTemplateVariable | "";
  widthPPM: number;
}

export interface ContractTemplateProviderComponent {
  componentRef: string;
  name: string;
  recipientRef?: string;
  required: boolean;
  typeCode: string;
  valueCode?: ContractTemplateProviderValue | "";
}

export interface ContractTemplateProviderRecipient {
  isPromoter: boolean;
  partyCode?: ContractTemplateParty | "";
  recipientRef: string;
  requireSign: boolean;
  roleName: string;
  routingOrder?: number | null;
  signComponentCount: number;
  typeCode: "ENTERPRISE" | "INDIVIDUAL" | "PROMOTER";
}

export interface ElectronicContractTemplateEditor {
  content?: ContractTemplateContentRevision;
  enableReady: boolean;
  enableReasonCode?: string;
  fields: ContractTemplateField[];
  onlineHtml?: string;
  previewConfirmed: boolean;
  providerComponents: ContractTemplateProviderComponent[];
  providerMappingSaved: boolean;
  providerRecipients: ContractTemplateProviderRecipient[];
  submissionAvailable: boolean;
  submissionReasonCode?: string;
  template: ElectronicContractTemplate;
}

function versionPath(templateNo: string, versionNo: number) {
  return `${templatePath}/${encodeURIComponent(templateNo)}/versions/${versionNo}`;
}

export function electronicContractTemplateList(
  query: ContractTemplateListQuery,
) {
  return readPage<ContractTemplateList>(
    pluginApiPath(pluginID, templatePath),
    { params: query },
  );
}

export function electronicContractTemplateGet(
  templateNo: string,
  versionNo: number,
) {
  return requestClient.get<{ item: ElectronicContractTemplate }>(
    pluginApiPath(pluginID, versionPath(templateNo, versionNo)),
  );
}

export function electronicContractTemplateCreate(input: {
  idempotencyKey: string;
  languageCode: Exclude<ContractTemplateLanguage, "">;
  name: string;
  scopeCode: Exclude<ContractTemplateScope, "">;
  signingOrderCode: Exclude<ContractTemplateSigningOrder, "">;
}) {
  return requestClient.post<{ item: ElectronicContractTemplate }>(
    pluginApiPath(pluginID, templatePath),
    input,
  );
}

export function electronicContractTemplateCopy(
  templateNo: string,
  versionNo: number,
  idempotencyKey: string,
) {
  return requestClient.post<{ item: ElectronicContractTemplate }>(
    pluginApiPath(pluginID, `${versionPath(templateNo, versionNo)}/copies`),
    { idempotencyKey },
  );
}

export function electronicContractTemplateUpdate(input: {
  expectedVersion: number;
  languageCode: Exclude<ContractTemplateLanguage, "">;
  name: string;
  scopeCode: Exclude<ContractTemplateScope, "">;
  signingOrderCode: Exclude<ContractTemplateSigningOrder, "">;
  templateNo: string;
  versionNo: number;
}) {
  return requestClient.put<{ item: ElectronicContractTemplate }>(
    pluginApiPath(
      pluginID,
      versionPath(input.templateNo, input.versionNo),
    ),
    input,
  );
}

export function electronicContractTemplateEnable(
  templateNo: string,
  versionNo: number,
  expectedVersion: number,
) {
  return requestClient.post<{ item: ElectronicContractTemplate }>(
    pluginApiPath(
      pluginID,
      `${versionPath(templateNo, versionNo)}/enablements`,
    ),
    { expectedVersion },
  );
}

export function electronicContractTemplateDisable(
  templateNo: string,
  versionNo: number,
  expectedVersion: number,
) {
  return requestClient.post<{ item: ElectronicContractTemplate }>(
    pluginApiPath(
      pluginID,
      `${versionPath(templateNo, versionNo)}/disablements`,
    ),
    { expectedVersion },
  );
}

export function electronicContractTemplateTencentSync() {
  return requestClient.post<{ item: ContractTemplateSyncResult }>(
    pluginApiPath(pluginID, `${templatePath}/tencent-syncs`),
  );
}

export function electronicContractTemplateEditorGet(
  templateNo: string,
  versionNo: number,
) {
  return requestClient.get<{ item: ElectronicContractTemplateEditor }>(
    pluginApiPath(
      pluginID,
      `${versionPath(templateNo, versionNo)}/editor`,
    ),
  );
}

export async function electronicContractTemplateUploadSource(input: {
  expectedVersion: number;
  file: File;
  idempotencyKey: string;
  mediaType: string;
  templateNo: string;
  versionNo: number;
}) {
  const body = await input.file.arrayBuffer();
  return requestClient.put<{ item: ElectronicContractTemplateEditor }>(
    pluginApiPath(
      pluginID,
      `${versionPath(input.templateNo, input.versionNo)}/source-file`,
    ),
    body,
    {
      headers: { "Content-Type": input.mediaType },
      params: {
        expectedVersion: input.expectedVersion,
        idempotencyKey: input.idempotencyKey,
      },
    },
  );
}

export function electronicContractTemplateOnlineSourceSave(input: {
  expectedVersion: number;
  html: string;
  idempotencyKey: string;
  templateNo: string;
  versionNo: number;
}) {
  return requestClient.put<{ item: ElectronicContractTemplateEditor }>(
    pluginApiPath(
      pluginID,
      `${versionPath(input.templateNo, input.versionNo)}/online-source`,
    ),
    input,
  );
}

export function electronicContractTemplatePreviewGenerate(input: {
  contentRevisionNo: string;
  expectedVersion: number;
  idempotencyKey: string;
  templateNo: string;
  versionNo: number;
}) {
  return requestClient.post<{ item: ElectronicContractTemplateEditor }>(
    pluginApiPath(
      pluginID,
      `${versionPath(input.templateNo, input.versionNo)}/pdf-previews`,
    ),
    input,
  );
}

export function electronicContractTemplatePreviewConfirm(input: {
  contentRevisionNo: string;
  expectedVersion: number;
  templateNo: string;
  versionNo: number;
}) {
  return requestClient.post<{ item: ElectronicContractTemplateEditor }>(
    pluginApiPath(
      pluginID,
      `${versionPath(input.templateNo, input.versionNo)}/preview-confirmations`,
    ),
    input,
  );
}

export function electronicContractTemplateFieldSetSave(input: {
  contentRevisionNo: string;
  expectedVersion: number;
  fields: ContractTemplateField[];
  idempotencyKey: string;
  templateNo: string;
  versionNo: number;
}) {
  return requestClient.post<{ item: ElectronicContractTemplateEditor }>(
    pluginApiPath(
      pluginID,
      `${versionPath(input.templateNo, input.versionNo)}/field-sets`,
    ),
    input,
  );
}

export function electronicContractTemplateProviderMappingSave(input: {
  expectedVersion: number;
  fieldMappings: Array<{
    componentRef: string;
    valueCode: ContractTemplateProviderValue;
  }>;
  idempotencyKey: string;
  recipientMappings: Array<{
    partyCode: ContractTemplateParty;
    recipientRef: string;
  }>;
  templateNo: string;
  versionNo: number;
}) {
  return requestClient.post<{ item: ElectronicContractTemplateEditor }>(
    pluginApiPath(
      pluginID,
      `${versionPath(input.templateNo, input.versionNo)}/provider-mappings`,
    ),
    input,
  );
}

export function electronicContractTemplatePreviewDownload(
  templateNo: string,
  versionNo: number,
  contentRevisionNo: string,
) {
  return requestClient.download<Blob>(
    pluginApiPath(
      pluginID,
      `${versionPath(templateNo, versionNo)}/content-revisions/${encodeURIComponent(contentRevisionNo)}/preview`,
    ),
  );
}

export function electronicContractTemplateProviderPreview(
  templateNo: string,
  versionNo: number,
) {
  return requestClient.get<{ previewUrl: string }>(
    pluginApiPath(
      pluginID,
      `${versionPath(templateNo, versionNo)}/provider-preview`,
    ),
  );
}
function readPage<T>(url: string, config: RequestClientConfig = {}) { return requestPreservingHttpStatus<T>(requestClient,url,{...config,method:"GET"}); }
