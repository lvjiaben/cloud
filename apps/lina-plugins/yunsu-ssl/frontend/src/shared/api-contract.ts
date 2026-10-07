import type { Audience, CertificateAction } from "./model.js";

// Owner-governed logical bridge paths are deliberately isolated here. The
// page never calls a supplier endpoint and never receives a host access token.
export function certificateAPI(audience: Audience) {
  const scope = audience === "admin" ? "admin" : "account";
  return {
    action: `${scope}/service-entitlements`,
    detail: `${scope}/service-entitlements`,
    download: `${scope}/service-entitlements`,
    list: `${scope}/service-entitlements`,
    task: `${scope}/service-action-tasks`,
  } as const;
}

export interface CertificateActionInput {
  action: CertificateAction;
  entitlementNo: string;
  validationMethod?: "dns" | "file";
}
