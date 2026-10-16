import { pluginApiPath, requestClient } from '#/api/request';

const compliancePluginID = 'idc-compliance';
const signerPath = 'account/compliance/contract-signers';

export type ContractSignerIdentityStatus =
  | 'expired'
  | 'review_required'
  | 'unverified'
  | 'verified'
  | 'verifying';
export type ContractSignerPhoneStatus =
  | 'unverified'
  | 'verified'
  | 'verifying';
export type ContractSignerStatus = 'active' | 'disabled';

export interface ContractSignerItem {
  authorizationVersion: number;
  createdAt: number;
  identityAttemptNo: string;
  identityStatusCode: ContractSignerIdentityStatus;
  identityVerificationVersion: number;
  identityVerifiedAt: number;
  maskedIdentityNo: string;
  maskedName: string;
  maskedPhone: string;
  phoneStatusCode: ContractSignerPhoneStatus;
  phoneVerificationVersion: number;
  phoneVerifiedAt: number;
  signerNo: string;
  statusCode: ContractSignerStatus;
  updatedAt: number;
  version: number;
}

export interface ContractSignerIdentityStartItem {
  attemptNo: string;
  continuationUrl: string;
  expiresAt: number;
  signerNo: string;
  statusCode: 'verifying';
}

export interface ContractSignerHumanChallengeItem {
  accessiblePrompt: string;
  challengeNo: string;
  expiresAt: number;
  imageDataUrl: string;
  kind: string;
  providerAction: string;
  providerCode: string;
  siteKey: string;
}

export interface ContractSignerPhoneCodeItem {
  deliveryState: string;
  expiresAt: number;
  maskedDestination: string;
  retryAfterAt: number;
  verificationNo: string;
}

function path(suffix = '') {
  return pluginApiPath(
    compliancePluginID,
    suffix ? `${signerPath}/${suffix}` : signerPath,
  );
}

export function listContractSigners() {
  return requestClient.get<{ items: ContractSignerItem[] }>(path());
}

export function createContractSigner(input: {
  certName: string;
  certNo: string;
  certType: 'IDENTITY_CARD';
  consentAccepted: true;
  consentPolicyVersion: '1';
  idempotencyKey: string;
  phone: string;
}) {
  return requestClient.post<{ item: ContractSignerItem }>(path(), input);
}

export function startContractSignerIdentity(signerNo: string) {
  return requestClient.post<{ item: ContractSignerIdentityStartItem }>(
    path(`${encodeURIComponent(signerNo)}/identity-verifications`),
  );
}

export function checkContractSignerIdentity(
  signerNo: string,
  attemptNo: string,
) {
  return requestClient.post<{ item: ContractSignerItem }>(
    path(
      `${encodeURIComponent(signerNo)}/identity-verifications/${encodeURIComponent(attemptNo)}/authoritative-checks`,
    ),
  );
}

export function createContractSignerPhoneChallenge(
  signerNo: string,
  input: {
    browserNonce: string;
    locale: 'en-US' | 'zh-CN';
    presentationKind: 'accessible' | 'image';
  },
) {
  return requestClient.post<{ item: ContractSignerHumanChallengeItem }>(
    path(`${encodeURIComponent(signerNo)}/phone-human-challenges`),
    input,
  );
}

export function completeContractSignerPhoneChallenge(
  signerNo: string,
  input: {
    answer?: string;
    browserNonce: string;
    challengeNo: string;
    locale: 'en-US' | 'zh-CN';
    providerToken?: string;
  },
) {
  return requestClient.post<{ item: { expiresAt: number; proofToken: string } }>(
    path(
      `${encodeURIComponent(signerNo)}/phone-human-challenge-completions`,
    ),
    input,
  );
}

export function sendContractSignerPhoneCode(
  signerNo: string,
  input: {
    browserNonce: string;
    idempotencyKey: string;
    locale: 'en-US' | 'zh-CN';
    proofToken: string;
  },
) {
  return requestClient.post<{ item: ContractSignerPhoneCodeItem }>(
    path(`${encodeURIComponent(signerNo)}/phone-verification-codes`),
    input,
    { silentErrorMessage: true },
  );
}

export function verifyContractSignerPhone(
  signerNo: string,
  input: {
    browserNonce: string;
    code: string;
    locale: 'en-US' | 'zh-CN';
    verificationNo: string;
  },
) {
  return requestClient.post<{ item: ContractSignerItem }>(
    path(`${encodeURIComponent(signerNo)}/phone-verifications`),
    input,
  );
}

export function disableContractSigner(
  signerNo: string,
  input: { expectedVersion: number; idempotencyKey: string },
) {
  return requestClient.post<{ item: ContractSignerItem }>(
    path(`${encodeURIComponent(signerNo)}/disablements`),
    input,
  );
}
