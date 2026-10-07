import { test, expect } from '../../support/tenant-core';
import { scenarioTC0194 } from '../../support/tenant-core-scenarios';

test.describe('TC-4 在线会话跨租户隔离', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-4a: online session revocation targets tenant-token pairs', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0194();
  });
});
