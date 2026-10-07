import { test, expect } from '../../support/tenant-core';
import { scenarioTC0216 } from '../../support/tenant-core-scenarios';

test.describe('TC-4 强制操作审计', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-4a: force lifecycle request is protected and observable through stable state', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0216();
  });
});
