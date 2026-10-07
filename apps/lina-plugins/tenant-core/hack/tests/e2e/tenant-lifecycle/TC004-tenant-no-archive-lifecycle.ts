import { test, expect } from '../../support/tenant-core';
import { scenarioTC0181 } from '../../support/tenant-core-scenarios';

test.describe('TC-4 租户不暴露归档生命周期', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-4a: archived status transitions are rejected', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0181();
  });
});
