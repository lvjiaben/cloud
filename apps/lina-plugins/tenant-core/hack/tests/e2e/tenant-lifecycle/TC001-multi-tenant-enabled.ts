import { test, expect } from '../../support/tenant-core';
import { scenarioTC0178 } from '../../support/tenant-core-scenarios';

test.describe('TC-1 多租户启用', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-1a: tenant-core plugin is installed and exposes real tenant APIs', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0178();
  });
});
