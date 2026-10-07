import { test, expect } from '../../support/tenant-core';
import { scenarioTC0190 } from '../../support/tenant-core-scenarios';

test.describe('TC-4 配置跨租户隔离', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-4a: tenant config override and platform fallback rows stay isolated', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0190();
  });
});
