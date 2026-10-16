import { pluginApiPath, requestClient } from '#/api/request';

const compliancePluginID = 'idc-compliance';
const compliancePath = 'account/compliance';

export type PersonalVerificationStatus =
  | 'expired'
  | 'rejected'
  | 'review_required'
  | 'unverified'
  | 'verified'
  | 'verifying';

export interface PersonalVerificationItem {
  attemptNo: string;
  caseNo: string;
  continuationUrl: string;
  expiresAt: number;
  maskedIdentityNo: string;
  maskedName: string;
  reasonCode: string;
  statusCode: PersonalVerificationStatus;
  submittedAt: number;
  verifiedAt: number;
}

export interface PersonalVerificationState {
  item?: null | PersonalVerificationItem;
  policyVersion: string;
}

function path(suffix: string) {
  return pluginApiPath(compliancePluginID, `${compliancePath}/${suffix}`);
}

export function getPersonalVerification() {
  return requestClient.get<PersonalVerificationState>(
    path('personal-verification'),
  );
}

export function createPersonalVerificationConsent(input: {
  idempotencyKey: string;
  locale: 'en-US' | 'zh-CN';
  policyVersion: string;
}) {
  return requestClient.post<{
    consentNo: string;
    occurredAt: number;
    policyVersion: string;
    reused: boolean;
  }>(path('personal-verification-consents'), {
    accepted: true,
    ...input,
  });
}

export function startPersonalVerification(input: {
  certName: string;
  certNo: string;
  consentNo: string;
}) {
  return requestClient.post<{ item: PersonalVerificationItem }>(
    path('personal-verifications'),
    { certType: 'IDENTITY_CARD', ...input },
  );
}

export function refreshPersonalVerification(attemptNo: string) {
  return requestClient.put<{ item: PersonalVerificationItem }>(
    path(
      `personal-verifications/${encodeURIComponent(attemptNo)}/authoritative-state`,
    ),
  );
}
