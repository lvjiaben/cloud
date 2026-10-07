import type { APIRequestContext, APIResponse } from '@host-tests/support/playwright';

import { expect, test } from '@host-tests/fixtures/auth';
import { pluginApiPath } from '@host-tests/fixtures/config';
import {
  createAdminApiContext,
  createApiContext,
  expectSuccess,
} from '@host-tests/support/api/job';

const customerEmail = 'idc-ai-customer-e2e@example.test';
const customerPassword = 'Customer-E2E-Password-2026!';

async function expectRejected(response: APIResponse) {
  const payload = await response.json().catch(() => null);
  expect(response.ok() && payload?.code === 0).toBe(false);
}

test.describe('TC002 customer assistant', () => {
  let adminApi: APIRequestContext;
  let customerApi: APIRequestContext;

  test.beforeAll(async () => {
    adminApi = await createAdminApiContext();
    await expectSuccess(
      await adminApi.post(
        pluginApiPath('idc-core', 'admin/customers/direct-accounts'),
        {
          data: {
            countryCode: 'CN',
            customerType: 'individual',
            displayName: 'IDC AI E2E Customer',
            email: customerEmail,
            idempotencyKey: 'idc-ai-customer-e2e-account-20260831',
            initialPassword: customerPassword,
            preferredLocale: 'zh-CN',
            reason: 'Create an isolated customer for IDC AI access verification.',
          },
        },
      ),
    );
    customerApi = await createApiContext(customerEmail, customerPassword);
  });

  test.afterAll(async () => {
    await Promise.all([adminApi.dispose(), customerApi.dispose()]);
  });

  test('TC-2a reads only the current customer context', async () => {
    await expectRejected(
      await adminApi.post('/x/idc-ai/api/v1/customer/ai/chat', {
        data: { prompt: '我的服务器和账单有什么异常？' },
      }),
    );
    const result = await expectSuccess<{ navigation: string }>(
      await customerApi.post('/x/idc-ai/api/v1/customer/ai/chat', {
        data: { prompt: '我的服务器和账单有什么异常？' },
      }),
    );
    expect(result.navigation).toMatch(/^\/account/u);
  });
  test('TC-2b customer never receives model configuration', async () => {
    const response = await customerApi.get('/x/idc-ai/api/v1/customer/ai/status');
    await expectSuccess(response);
    const text = JSON.stringify(await response.json());
    expect(text).not.toMatch(/baseUrl|model|apiKey|budget/i);
  });
  test('TC-2c anomaly points to normal page or ticket', async () => {
    const result = await expectSuccess<{ navigation: string }>(
      await customerApi.post('/x/idc-ai/api/v1/customer/ai/chat', {
        data: { prompt: '支付失败怎么办？' },
      }),
    );
    expect(result.navigation).toMatch(/^\/account/u);
  });
});
