import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

const now = 1_786_480_800_000;
const providerURL =
  'https://picker.antcloudauth.aliyuncs.com/gateway.do?token=opaque';

type VerificationStatus =
  | 'rejected'
  | 'review_required'
  | 'unverified'
  | 'verified'
  | 'verifying';

function envelope(data: unknown) {
  return { code: 0, data, message: 'OK' };
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({ json: envelope(data), status: 200 });
}

function verification(status: VerificationStatus, unsafeContinuation = false) {
  const started = status !== 'unverified';
  return {
    attemptNo: started ? 'pva_20260812_0001' : '',
    caseNo: started ? 'case_20260812_0001' : '',
    continuationUrl:
      status === 'verifying'
        ? unsafeContinuation
          ? 'javascript:alert(1)'
          : providerURL
        : '',
    expiresAt: started ? now + 1_800_000 : 0,
    maskedIdentityNo: status === 'verified' ? '110***********002X' : '',
    maskedName: status === 'verified' ? '测**' : '',
    reasonCode: status === 'verified' ? 'provider_passed' : '',
    statusCode: status,
    submittedAt: started ? now : 0,
    verifiedAt: status === 'verified' ? now + 120_000 : 0,
  };
}

export interface PersonalVerificationMockState {
  consentRequests: Array<Record<string, unknown>>;
  paymentResumeRequests: string[];
  refreshRequests: Array<{ attemptNo: string; body: null | string }>;
  startRequests: Array<Record<string, unknown>>;
}

export async function installPersonalVerificationMock(
  page: Page,
  options: {
    authoritativeStatus?: VerificationStatus;
    initialStatus?: VerificationStatus;
    locale?: 'en-US' | 'zh-CN';
    paymentReady?: boolean;
    personalOnly?: boolean;
    unsafeContinuation?: boolean;
  } = {},
) {
  await mockCustomerAPIs(page, { locale: options.locale });
  const state: PersonalVerificationMockState = {
    consentRequests: [],
    paymentResumeRequests: [],
    refreshRequests: [],
    startRequests: [],
  };
  let current = verification(options.initialStatus || 'unverified');
  const root = '/x/idc-compliance/api/v1/account/compliance';

  await page.route(
    '**/x/idc-compliance/api/v1/account/compliance/**',
    async (route) => {
      if (options.personalOnly) {
        await route.fulfill({
          json: {
            code: 'PERSONAL_ONLY',
            data: null,
            message: 'not accessible',
          },
          status: 403,
        });
        return;
      }
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const method = request.method();
      if (path === `${root}/personal-verification` && method === 'GET') {
        await fulfill(route, {
          item: current,
          policyVersion: 'personal-spi-v1',
        });
        return;
      }
      if (
        path === `${root}/personal-verification-consents` &&
        method === 'POST'
      ) {
        state.consentRequests.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        await fulfill(route, {
          consentNo: 'consent_20260812_0001',
          occurredAt: now,
          policyVersion: 'personal-spi-v1',
          reused: false,
        });
        return;
      }
      if (path === `${root}/personal-verifications` && method === 'POST') {
        state.startRequests.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        current = verification('verifying', options.unsafeContinuation);
        await fulfill(route, { item: current });
        return;
      }
      if (path.endsWith('/authoritative-state') && method === 'PUT') {
        state.refreshRequests.push({
          attemptNo: decodeURIComponent(path.split('/').at(-2) || ''),
          body: request.postData(),
        });
        current = verification(options.authoritativeStatus || 'verified');
        await fulfill(route, { item: current });
        return;
      }
      await route.fulfill({
        json: { code: 404, data: null, message: 'not found' },
        status: 404,
      });
    },
  );

  await page.route(
    '**/x/idc-core/api/v1/account/commerce/payment-preparations/**',
    async (route) => {
      const path = new URL(route.request().url()).pathname;
      const contextNo = decodeURIComponent(path.split('/').at(-1) || '');
      state.paymentResumeRequests.push(contextNo);
      await fulfill(route, {
        preparation: {
          amountDueMinor: '19900',
          contextNo,
          currencyCode: 'CNY',
          customerTypeCode: 'personal',
          identityStatusCode: 'verified',
          invoiceNo: 'INV_20260812_0001',
          lastEvaluatedAt: now + 120_000,
          orderNo: 'ORD_20260811_0001',
          paymentDueAt: now + 3_600_000,
          ready: options.paymentReady !== false,
          reasonCode: '',
          reused: true,
          statusCode: 'ready',
        },
      });
    },
  );

  await page.route(
    'https://picker.antcloudauth.aliyuncs.com/**',
    async (route) => {
      await route.fulfill({
        body: '<!doctype html><title>Aliyun verification fixture</title>',
        contentType: 'text/html',
        status: 200,
      });
    },
  );
  return state;
}

export { providerURL as identityProviderURL };
