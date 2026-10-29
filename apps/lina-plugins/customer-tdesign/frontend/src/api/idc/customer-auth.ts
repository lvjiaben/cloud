// Customer authentication uses the host password owner and purpose-bound contact proofs.
import { pluginApiPath, requestClient, ApiError } from "#/api/request";
import type { HumanChallenge, VerificationCode } from "./customer-onboarding";
type LoginKind = "username" | "email" | "phone";
export interface RecoveryBinding {
  kind: "email" | "phone";
  account: string;
  browserNonce: string;
  locale: "zh-CN" | "en-US";
}
const path = (resource: string) =>
  pluginApiPath("idc-notify", `public/account/${resource}`);
// Each submission chooses one existing authentication route; never retry passwords
// against another identity type after an authentication failure.
const emailAccount = /^[^\s@]+@[^\s@]+$/;
const phoneAccount = /^(?:1[0-9]{10}|\+[1-9][0-9]{6,14})$/;
export function usernameAllowed(value: string | undefined) {
  const account = value?.trim() || "";
  return (
    !/^\p{Decimal_Number}+$/u.test(account) &&
    !emailAccount.test(account) &&
    !phoneAccount.test(account)
  );
}
export function passwordLogin(
  account: string,
  password: string,
  kind?: LoginKind,
) {
  account = account.trim();
  kind ??= emailAccount.test(account)
    ? "email"
    : phoneAccount.test(account)
      ? "phone"
      : "username";
  return requestClient.post<{ accessToken: string; refreshToken?: string }>(
    path("password-logins"),
    { kind, account, password },
  );
}
export function createRecoveryChallenge(data: RecoveryBinding) {
  return requestClient.post<HumanChallenge>(
    path("password-recovery/human-challenges"),
    { ...data, presentationKind: "accessible" },
  );
}
export function completeRecoveryChallenge(
  data: RecoveryBinding & { challengeNo: string; answer: string },
) {
  return requestClient.post<{ proofToken: string }>(
    path("password-recovery/human-challenge-completions"),
    { ...data, providerToken: "" },
  );
}
export function sendRecoveryCode(
  data: RecoveryBinding & { proofToken: string; idempotencyKey: string },
) {
  return requestClient
    .post<VerificationCode>(path("password-recovery/verification-codes"), data)
    .catch((error: unknown) => {
      if (
        error instanceof ApiError &&
        (error.response.data as { errorCode?: string })?.errorCode ===
          "IDC_NOTIFY_CHANNEL_UNAVAILABLE"
      )
        throw new Error("验证码服务暂未开通，请联系网站客服");
      throw error;
    });
}
export function resetPassword(
  data: RecoveryBinding & {
    verificationNo: string;
    code: string;
    password: string;
  },
) {
  return requestClient.post<{
    reset: boolean;
    cleanupPending: boolean;
    cleanupToken: string;
  }>(path("password-recovery/password-resets"), data);
}
export function retryRecoveryCleanup(
  data: RecoveryBinding & { verificationNo: string; cleanupToken: string },
) {
  return requestClient.post<{ pending: boolean }>(
    path("password-recovery/session-cleanups"),
    data,
  );
}
