import type { Page } from '@playwright/test';

import { mockCustomerAPIs } from './customer-api-mock';

type Body = Record<string, unknown>;

const now = 1_787_356_800_000;

function key(status: 'active' | 'revoked' = 'active') {
  return {
    algorithm: 'ssh-ed25519',
    createdAt: now,
    fingerprint: 'SHA256:customerSafeFingerprint0000000000000000000',
    keyNo: 'SSH_00000000000000000000000000000001',
    name: 'Production access',
    revokedAt: status === 'revoked' ? now + 1_000 : null,
    status,
    version: status === 'revoked' ? 2 : 1,
  };
}

export async function installCustomerSSHKeyMock(
  page: Page,
  options: { locale?: 'en-US' | 'zh-CN' } = {},
) {
  await mockCustomerAPIs(page, options);
  const state = {
    createBodies: [] as Body[],
    items: [] as ReturnType<typeof key>[],
    revokeBodies: [] as Body[],
  };

  await page.route(
    '**/x/idc-core/api/v1/account/ssh-public-keys**',
    async (route) => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      if (request.method() === 'GET') {
        await route.fulfill({
          json: { code: 0, data: { items: state.items }, message: 'OK' },
        });
        return;
      }
      if (
        request.method() === 'POST' &&
        path.endsWith('/account/ssh-public-keys')
      ) {
        state.createBodies.push(request.postDataJSON() as Body);
        state.items = [key()];
        await route.fulfill({
          json: { code: 0, data: { item: state.items[0] }, message: 'OK' },
        });
        return;
      }
      if (request.method() === 'DELETE') {
        state.revokeBodies.push(request.postDataJSON() as Body);
        state.items = [key('revoked')];
        await route.fulfill({
          json: { code: 0, data: { item: state.items[0] }, message: 'OK' },
        });
        return;
      }
      await route.fulfill({
        json: { code: 4_004, message: 'unexpected SSH key request' },
        status: 404,
      });
    },
  );

  return state;
}
