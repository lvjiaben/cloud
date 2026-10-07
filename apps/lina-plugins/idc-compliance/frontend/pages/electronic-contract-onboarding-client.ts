import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-compliance";
const onboardingPath = "admin/compliance/electronic-contract/onboarding";

export type OnboardingState =
  | "active"
  | "enterprise_pending"
  | "not_configured"
  | "operator_pending"
  | "ready"
  | "temporarily_unavailable";

export interface OnboardingCapability {
  checkedAt: number;
  code: "auto_sign.enterprise" | "flow.download";
  reasonCode: string;
  state: "available" | "disabled" | "unavailable";
}

export interface ElectronicContractOnboarding {
  bindingNo: string;
  capabilities: OnboardingCapability[];
  consoleUrl?: string;
  merchant: {
    countryCode: string;
    legalName: string;
    merchantNo: string;
  };
  operatorReady: boolean;
  organizationReady: boolean;
  reasonCode: string;
  state: OnboardingState;
  subjectStatus: string;
}

export function electronicContractOnboardingGet() {
  return requestClient.get<{ item: ElectronicContractOnboarding }>(
    pluginApiPath(pluginID, onboardingPath),
  );
}

export function electronicContractOnboardingLinkCreate(idempotencyKey: string) {
  return requestClient.post<{ item: ElectronicContractOnboarding }>(
    pluginApiPath(pluginID, onboardingPath + "/links"),
    { idempotencyKey },
  );
}

export function electronicContractOnboardingRecheck(idempotencyKey: string) {
  return requestClient.post<{ item: ElectronicContractOnboarding }>(
    pluginApiPath(pluginID, onboardingPath + "/rechecks"),
    { idempotencyKey },
  );
}
