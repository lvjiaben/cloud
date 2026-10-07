import type { Page, Route } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

function envelope(data: unknown) {
  return { code: 0, data, message: 'OK' };
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({ json: envelope(data), status: 200 });
}

export interface CustomerOnboardingMockState {
  humanChallenges: Array<Record<string, unknown>>;
  humanCompletions: Array<Record<string, unknown>>;
  intentRequests: Array<Record<string, unknown>>;
  redeemRequests: Array<Record<string, unknown>>;
  registrations: Array<Record<string, unknown>>;
  verificationRequests: Array<Record<string, unknown>>;
}

export async function installCustomerOnboardingMock(page: Page) {
  await mockCustomerAPIs(page);
  const state: CustomerOnboardingMockState = {
    humanChallenges: [],
    humanCompletions: [],
    intentRequests: [],
    redeemRequests: [],
    registrations: [],
    verificationRequests: [],
  };
  const now = Date.now();

  await page.route('**/api/v1/auth/register', async (route) => {
    state.registrations.push(
      route.request().postDataJSON() as Record<string, unknown>,
    );
    await fulfill(route, { userId: 2001 });
  });
  await page.route('**/api/v1/user/info', async (route) => {
    await fulfill(route, {
      avatar: '',
      homePath: '/account',
      realName: '自助开户客户',
      roles: [],
      userId: 2001,
      username: 'selfbuyer',
    });
  });
  await page.route(
    '**/x/idc-notify/api/v1/public/account/onboarding/**',
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      const body = request.postDataJSON() as Record<string, unknown>;
      if (path.endsWith('/human-challenges')) {
        state.humanChallenges.push(body);
        await fulfill(route, {
          accessiblePrompt: '2 + 3 = ?',
          challengeNo: 'challenge-0001',
          expiresAt: now + 300_000,
          kind: 'accessible',
          providerCode: 'builtin',
        });
        return;
      }
      if (path.endsWith('/human-challenge-completions')) {
        state.humanCompletions.push(body);
        await fulfill(route, {
          expiresAt: now + 300_000,
          proofToken: 'proof-token-0001',
        });
        return;
      }
      if (path.endsWith('/verification-codes')) {
        state.verificationRequests.push(body);
        await fulfill(route, {
          deliveryState: 'sent',
          expiresAt: now + 600_000,
          maskedDestination: 's***@example.com',
          retryAfterAt: now + 60_000,
          verificationNo: 'verification-0001',
        });
        return;
      }
      if (path.endsWith('/intents')) {
        state.intentRequests.push(body);
        await fulfill(route, {
          expiresAt: now + 1_800_000,
          intentNo: 'intent-0001',
          reused: false,
          token: 'T'.repeat(43),
        });
        return;
      }
      await route.fulfill({ status: 404 });
    },
  );
  await page.route(
    '**/x/idc-core/api/v1/account/onboarding/**',
    async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith('/status')) {
        await fulfill(route, { required: true });
        return;
      }
      if (path.endsWith('/redeem')) {
        state.redeemRequests.push(
          route.request().postDataJSON() as Record<string, unknown>,
        );
        await fulfill(route, {
          customerNo: 'CUS_00000000000000000000000000002001',
          customerType: 'individual',
          reused: false,
        });
        return;
      }
      await route.fulfill({ status: 404 });
    },
  );
  return state;
}

export async function installCustomerActivationMock(page: Page) {
  await mockCustomerAPIs(page);
  const tokens: string[] = [];
  await page.route(
    '**/x/idc-core/api/v1/account/activation-invitations/redeem',
    async (route) => {
      const body = route.request().postDataJSON() as { token?: string };
      const token = body.token ?? '';
      tokens.push(token);
      if (token.startsWith('active-')) {
        await fulfill(route, {
          customerNo: 'CUS_00000000000000000000000000001001',
          customerType: 'individual',
          reused: false,
          status: 'active',
        });
        return;
      }
      await route.fulfill({
        json: { code: 409, data: null, message: 'invitation unavailable' },
        status: 409,
      });
    },
  );
  return { tokens };
}
