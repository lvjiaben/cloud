import { test, expect } from '../../support/tenant-core';
import { scenarioTC0214 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 平台管理员跨租户读', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: platform tenant list can read tenants across scopes', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0214();
  });
});
