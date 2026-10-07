import { test, expect } from '../../support/tenant-core';
import { scenarioTC0183 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 多租户禁用退化', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: tenant-core lifecycle precondition keeps the platform-only plugin installed', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0183();
  });
});
