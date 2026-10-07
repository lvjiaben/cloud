import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

type SignerState = {
  authorizationVersion: number;
  createdAt: number;
  identityAttemptNo: string;
  identityStatusCode: string;
  identityVerificationVersion: number;
  identityVerifiedAt: number;
  maskedIdentityNo: string;
  maskedName: string;
  maskedPhone: string;
  phoneStatusCode: string;
  phoneVerificationVersion: number;
  phoneVerifiedAt: number;
  signerNo: string;
  statusCode: string;
  updatedAt: number;
  version: number;
};

function envelope(data: unknown) {
  return { code: 0, data, message: 'OK' };
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({ json: envelope(data), status: 200 });
}

function newSigner(): SignerState {
  return {
    authorizationVersion: 1,
    createdAt: 1_786_665_600_000,
    identityAttemptNo: '',
    identityStatusCode: 'unverified',
    identityVerificationVersion: 0,
    identityVerifiedAt: 0,
    maskedIdentityNo: '110***********1234',
    maskedName: '测***人',
    maskedPhone: '138****8000',
    phoneStatusCode: 'unverified',
    phoneVerificationVersion: 0,
    phoneVerifiedAt: 0,
    signerNo: 'csgn_e2e_0001',
    statusCode: 'active',
    updatedAt: 1_786_665_600_000,
    version: 1,
  };
}

export interface ContractSignerMockState {
  requests: Array<{ method: string; path: string; payload: unknown }>;
  signer?: SignerState;
}

export async function installContractSignerMock(
  page: Page,
): Promise<ContractSignerMockState> {
  await mockCustomerAPIs(page);
  const state: ContractSignerMockState = { requests: [] };
  const root =
    '/x/idc-compliance/api/v1/account/compliance/contract-signers';

  await page.route(
    '**/x/idc-core/api/v1/account/customer/profile',
    async (route) => {
      await fulfill(route, {
        item: {
          countryCode: 'CN',
          customerCode: 10000,
          displayName: '测试企业',
          identifierMasked: '91**************01',
          legalNameMasked: '测***业',
          preferredLocale: 'zh-CN',
          status: 'active',
          type: 'enterprise',
          verification: 'verified',
          version: 1,
        },
      });
    },
  );

  await page.route('**/x/idc-compliance/api/v1/account/compliance/contract-signers**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const payload = request.postData()
      ? (request.postDataJSON() as unknown)
      : undefined;
    state.requests.push({ method: request.method(), path: url.pathname, payload });

    if (url.pathname === root && request.method() === 'GET') {
      await fulfill(route, { items: state.signer ? [state.signer] : [] });
      return;
    }
    if (url.pathname === root && request.method() === 'POST') {
      state.signer = newSigner();
      await fulfill(route, { item: state.signer });
      return;
    }
    if (!state.signer) {
      await route.fulfill({ json: { code: 404, message: 'not found' }, status: 404 });
      return;
    }
    if (url.pathname.endsWith('/identity-verifications') && request.method() === 'POST') {
      state.signer.identityAttemptNo = 'csia_e2e_0001';
      state.signer.identityStatusCode = 'verifying';
      state.signer.version += 1;
      await fulfill(route, {
        item: {
          attemptNo: state.signer.identityAttemptNo,
          continuationUrl: 'about:blank#contract-signer-identity',
          expiresAt: 1_786_666_200_000,
          signerNo: state.signer.signerNo,
          statusCode: 'verifying',
        },
      });
      return;
    }
    if (url.pathname.endsWith('/authoritative-checks') && request.method() === 'POST') {
      state.signer.identityStatusCode = 'verified';
      state.signer.identityVerificationVersion = 1;
      state.signer.identityVerifiedAt = 1_786_665_660_000;
      state.signer.version += 1;
      await fulfill(route, { item: state.signer });
      return;
    }
    if (url.pathname.endsWith('/phone-human-challenges')) {
      await fulfill(route, {
        item: {
          accessiblePrompt: '3 + 4 = ?',
          challengeNo: 'challenge-e2e-1',
          expiresAt: 1_786_665_900_000,
          imageDataUrl: '',
          kind: 'accessible',
          providerAction: 'contract_signer_phone',
          providerCode: 'local',
          siteKey: '',
        },
      });
      return;
    }
    if (url.pathname.endsWith('/phone-human-challenge-completions')) {
      await fulfill(route, {
        item: { expiresAt: 1_786_665_900_000, proofToken: 'proof-e2e-1' },
      });
      return;
    }
    if (url.pathname.endsWith('/phone-verification-codes')) {
      state.signer.phoneStatusCode = 'verifying';
      state.signer.version += 1;
      await fulfill(route, {
        item: {
          deliveryState: 'accepted',
          expiresAt: 1_786_666_200_000,
          maskedDestination: state.signer.maskedPhone,
          retryAfterAt: 1_786_665_660_000,
          verificationNo: 'verification-e2e-1',
        },
      });
      return;
    }
    if (url.pathname.endsWith('/phone-verifications')) {
      state.signer.phoneStatusCode = 'verified';
      state.signer.phoneVerificationVersion = 1;
      state.signer.phoneVerifiedAt = 1_786_665_720_000;
      state.signer.version += 1;
      await fulfill(route, { item: state.signer });
      return;
    }
    if (url.pathname.endsWith('/disablements')) {
      state.signer.statusCode = 'disabled';
      state.signer.authorizationVersion += 1;
      state.signer.version += 1;
      await fulfill(route, { item: state.signer });
      return;
    }
    await route.fulfill({ json: { code: 404, message: 'not found' }, status: 404 });
  });

  return state;
}
