import { test, expect } from '../../support/tenant-core';
import { scenarioTC0217 } from '../../support/tenant-core-scenarios';

test.describe('TC-1 平台与租户视图差异', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-1a: platform and tenant API menu views differ', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0217();
  });
});
