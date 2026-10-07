import { test, expect } from '../../support/tenant-core';
import { scenarioTC0239 } from '../../support/tenant-core-scenarios';

test.describe('TC-1 租户角色平台权限阻断', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-1a: dirty platform grants are hidden and rejected in tenant context', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0239();
  });
});
