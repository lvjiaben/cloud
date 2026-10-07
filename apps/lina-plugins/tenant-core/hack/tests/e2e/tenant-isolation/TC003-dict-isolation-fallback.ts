import { test, expect } from '../../support/tenant-core';
import { scenarioTC0189 } from '../../support/tenant-core-scenarios';

test.describe('TC-3 字典跨租户隔离', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-3a: tenant dictionary override and platform fallback rows stay isolated', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0189();
  });
});
