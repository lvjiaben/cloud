import { pluginApiPath, requestClient } from '#/api/request';

const notifyPluginID = 'idc-notify';
const corePluginID = 'idc-core';

export type CustomerType = 'enterprise' | 'individual';
export type DestinationKind = 'email' | 'sms';

export interface OnboardingBinding {
  browserNonce: string;
  customerType: 'individual';
  destination: string;
  destinationKind: DestinationKind;
  locale: 'en-US' | 'zh-CN';
  returnPath: string;
  username: string;
}

export interface HumanChallenge {
  accessiblePrompt?: string;
  challengeNo: string;
  expiresAt: number;
  imageDataUrl?: string;
  kind: string;
  providerAction?: string;
  providerCode: string;
  siteKey?: string;
}

export interface VerificationCode {
  deliveryState: string;
  expiresAt: number;
  maskedDestination: string;
  retryAfterAt: number;
  verificationNo: string;
}

export interface OnboardingIntent {
  expiresAt: number;
  intentNo: string;
  reused: boolean;
  token: string;
}

export function createOnboardingHumanChallenge(data: OnboardingBinding) {
  return requestClient.post<HumanChallenge>(
    pluginApiPath(
      notifyPluginID,
      'public/account/onboarding/human-challenges',
    ),
    { ...data, presentationKind: 'accessible' },
  );
}

export function completeOnboardingHumanChallenge(
  data: OnboardingBinding & { answer: string; challengeNo: string },
) {
  return requestClient.post<{ expiresAt: number; proofToken: string }>(
    pluginApiPath(
      notifyPluginID,
      'public/account/onboarding/human-challenge-completions',
    ),
    { ...data, providerToken: '' },
  );
}

export function sendOnboardingVerificationCode(
  data: OnboardingBinding & { idempotencyKey: string; proofToken: string },
) {
  return requestClient.post<VerificationCode>(
    pluginApiPath(
      notifyPluginID,
      'public/account/onboarding/verification-codes',
    ),
    data,
  );
}

export function createVerifiedOnboardingIntent(
  data: OnboardingBinding & {
    code: string;
    idempotencyKey: string;
    verificationNo: string;
  },
) {
  return requestClient.post<OnboardingIntent>(
    pluginApiPath(notifyPluginID, 'public/account/onboarding/intents'),
    data,
  );
}

export function getCustomerOnboardingStatus() {
  return requestClient.get<{ customerNo?: string; required: boolean }>(
    pluginApiPath(corePluginID, 'account/onboarding/status'),
  );
}

export function redeemCustomerOnboarding(data: {
  browserNonce: string;
  deviceId: string;
  hostRegistrationUserId: number;
  token: string;
}) {
  return requestClient.post<{
    customerNo: string;
    customerType: CustomerType;
    reused: boolean;
  }>(pluginApiPath(corePluginID, 'account/onboarding/redeem'), data);
}

const registrationDeviceKey = 'linapro.customer.registration.device';

export function getOrCreateRegistrationDeviceID() {
  const existing = localStorage.getItem(registrationDeviceKey)?.trim() || '';
  if (/^device-[0-9a-f]{36}$/u.test(existing)) return existing;
  const bytes = new Uint8Array(18);
  crypto.getRandomValues(bytes);
  const created = `device-${Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')}`;
  localStorage.setItem(registrationDeviceKey, created);
  return created;
}
