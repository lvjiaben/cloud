// This mock records the exact customer-account API contract. It models only
// provider-authoritative transitions and never accepts a customer identifier,
// tenant identifier, return URL, or browser-supplied verification result.
import type { Page } from "@host-tests/support/playwright";

const accountBase =
  "/x/idc-compliance/api/v1/account/compliance";

type VerificationStatus = "failed" | "unverified" | "verified" | "verifying";

function item(status: VerificationStatus) {
  const started = status !== "unverified";
  return {
    attemptNo: started ? "pva_20260812_0001" : "",
    caseNo: started ? "case_20260812_0001" : "",
    continuationUrl:
      status === "verifying"
        ? "https://picker.antcloudauth.aliyuncs.com/gateway.do?token=opaque"
        : "",
    expiresAt: started ? 1_786_482_600_000 : 0,
    maskedIdentityNo: status === "verified" ? "110***********002X" : "",
    maskedName: status === "verified" ? "测**" : "",
    reasonCode:
      status === "verified"
        ? "provider_passed"
        : status === "failed"
          ? "unsupported_identity"
          : "",
    statusCode: status,
    submittedAt: started ? 1_786_480_800_000 : 0,
    verifiedAt: status === "verified" ? 1_786_480_920_000 : 0,
  };
}

export interface PersonalVerificationAccountMockState {
  consentRequests: Array<Record<string, unknown>>;
  refreshRequests: Array<{ attemptNo: string; body: string | null }>;
  startRequests: Array<Record<string, unknown>>;
  unsupportedRequests: Array<Record<string, unknown>>;
}

export async function installPersonalVerificationAccountMock(
  page: Page,
  options: { authoritativeStatus?: VerificationStatus; unauthorized?: boolean } = {},
) {
  const state: PersonalVerificationAccountMockState = {
    consentRequests: [],
    refreshRequests: [],
    startRequests: [],
    unsupportedRequests: [],
  };
  let current = item("unverified");

  await page.route(`**${accountBase}/**`, async (route) => {
    if (options.unauthorized) {
      await route.fulfill({
        contentType: "application/json",
        json: { code: "AUTH_REQUIRED", message: "login required" },
        status: 401,
      });
      return;
    }
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    let data: unknown;
    if (path.endsWith("/personal-verification") && method === "GET") {
      data = { item: current, policyVersion: "personal-spi-v1" };
    } else if (
      path.endsWith("/personal-verification-consents") &&
      method === "POST"
    ) {
      const body = request.postDataJSON() as Record<string, unknown>;
      state.consentRequests.push(body);
      data = {
        consentNo: "consent_20260812_0001",
        occurredAt: 1_786_480_800_000,
        policyVersion: "personal-spi-v1",
        reused: false,
      };
    } else if (path.endsWith("/personal-verifications") && method === "POST") {
      const body = request.postDataJSON() as Record<string, unknown>;
      state.startRequests.push(body);
      if (
        body.certType !== "IDENTITY_CARD" ||
        body.certNo === "110105201801010010"
      ) {
        state.unsupportedRequests.push(body);
        await route.fulfill({
          contentType: "application/json",
          json: {
            code: "PERSONAL_VERIFICATION_UNSUPPORTED",
            message: "unsupported age or identity document",
          },
          status: 400,
        });
        return;
      }
      current = item("verifying");
      data = { item: current };
    } else if (
      path.endsWith("/authoritative-state") &&
      method === "PUT"
    ) {
      const attemptNo = decodeURIComponent(
        path.split("/").at(-2) ?? "",
      );
      state.refreshRequests.push({ attemptNo, body: request.postData() });
      current = item(options.authoritativeStatus ?? "verified");
      data = { item: current };
    } else {
      await route.fallback();
      return;
    }
    await route.fulfill({
      contentType: "application/json",
      json: { code: 0, data, message: "OK" },
      status: 200,
    });
  });
  return state;
}

export { accountBase as personalVerificationAccountBase };
