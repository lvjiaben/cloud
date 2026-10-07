import { requestPreservingHttpStatus } from "./compliance-request";
import type { RequestClientConfig } from "@vben/request";
import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-compliance";
const basePath =
  "admin/compliance/electronic-contract/auto-sign-authorizations";

export interface AutoSignTemplateRef {
  name: string;
  templateNo: string;
  versionNo: number;
}

export interface AutoSignAuthorization {
  allowManualOptional: boolean;
  allowRequiredOrder: boolean;
  authorizationNo: string;
  createdAt: number;
  createdByUserId: number;
  currencyCode: string;
  expiresAt: number;
  maxAmountMinor: string;
  revokedAt?: number;
  revokedByUserId?: number;
  statusCode: "active" | "expired" | "revoked";
  successCount: number;
  successLimit: number;
  templates: AutoSignTemplateRef[];
  version: number;
}

export interface AutoSignAuthorizationPage {
  candidates: AutoSignTemplateRef[];
  items: AutoSignAuthorization[];
  page: number;
  pageSize: number;
  total: number;
}

export interface CreateAutoSignAuthorizationInput {
  allowManualOptional: boolean;
  allowRequiredOrder: boolean;
  confirmed: true;
  currencyCode: string;
  expiresAt: number;
  idempotencyKey: string;
  maxAmountMinor: string;
  successLimit: number;
  templates: Array<Pick<AutoSignTemplateRef, "templateNo" | "versionNo">>;
}

export function autoSignAuthorizationList(page = 1, pageSize = 10) {
  return readPage<AutoSignAuthorizationPage>(
    pluginApiPath(pluginID, basePath),
    { params: { page, pageSize } },
  );
}

export function autoSignAuthorizationCreate(
  input: CreateAutoSignAuthorizationInput,
) {
  return requestClient.post<{ item: AutoSignAuthorization }>(
    pluginApiPath(pluginID, basePath),
    input,
  );
}

export function autoSignAuthorizationRevoke(
  authorizationNo: string,
  expectedVersion: number,
  idempotencyKey: string,
) {
  return requestClient.put<{ item: AutoSignAuthorization }>(
    pluginApiPath(pluginID, `${basePath}/${authorizationNo}/revocation`),
    { confirmed: true, expectedVersion, idempotencyKey },
  );
}
function readPage<T>(url: string, config: RequestClientConfig = {}) { return requestPreservingHttpStatus<T>(requestClient,url,{...config,method:"GET"}); }
