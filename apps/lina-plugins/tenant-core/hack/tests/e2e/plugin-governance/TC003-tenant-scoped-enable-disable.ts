import { test, expect } from '../../support/tenant-core';
import { scenarioTC0208 } from '../../support/tenant-core-scenarios';

test.describe('TC-3 tenant-scoped 插件启停', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-3a: tenant plugin API toggles tenant-scoped enablement rows', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0208();
  });
});
