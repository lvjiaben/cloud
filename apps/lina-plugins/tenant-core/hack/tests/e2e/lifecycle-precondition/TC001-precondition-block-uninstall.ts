import { test, expect } from '../../support/tenant-core';
import { scenarioTC0210 } from '../../support/tenant-core-scenarios';

test.describe('TC-1 Lifecycle Precondition 否决卸载', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-1a: lifecycle precondition blocks tenant-core uninstall', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0210();
  });
});
