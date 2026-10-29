import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./provider-request";

const pluginID = "idc-resource";
const basePath = "admin/resource/server-renewals";

export interface ServerRenewalCandidate {
  productNameEnUS: string;
  productNameZhCN: string;
  serverNo: string;
}

export interface ServerRenewalPreviewItem extends ServerRenewalCandidate {
  currencyCode: string;
  eligible: boolean;
  failureCode: string;
  originalExpiresAt?: number;
  projectedExpiresAt?: number;
  quoteExpiresAt?: number;
  totalAmountMinor: string;
}

export interface ServerRenewalPreview {
  confirmationDigest: string;
  items: ServerRenewalPreviewItem[];
  renewalCycles: number;
  requestKey: string;
}

export interface ServerRenewalResultItem {
  currencyCode: string;
  failureCode: string;
  productNameEnUS: string;
  productNameZhCN: string;
  serverNo: string;
  status: string;
  succeeded: boolean;
  totalAmountMinor: string;
}

export function listServerRenewalCandidates(input: {
	 entitlementNo?: string;
	page?: number;
  cursor?: string;
  limit?: number;
}) {
  return requestPreservingHttpStatus<{
	total: number;
    items: ServerRenewalCandidate[];
    nextCursor: string;
  }>(requestClient, pluginApiPath(pluginID, basePath), { method: "GET", params: input });
}

export function previewServerRenewals(input: {
  entitlementNo?: string;
  renewalCycles: number;
  requestKey: string;
  serverNos: string[];
}) {
  return requestClient.post<ServerRenewalPreview>(
    pluginApiPath(pluginID, `${basePath}/previews`),
    input,
  );
}

export function confirmServerRenewals(input: {
  entitlementNo?: string;
  confirmationDigest: string;
  renewalCycles: number;
  requestKey: string;
  serverNos: string[];
}) {
  return requestClient.post<{
    items: ServerRenewalResultItem[];
    renewalCycles: number;
    requestKey: string;
  }>(pluginApiPath(pluginID, `${basePath}/confirmations`), input);
}
