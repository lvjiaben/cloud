import { test, expect } from '../../support/tenant-core';
import { scenarioTC0204 } from '../../support/tenant-core-scenarios';

test.describe('TC-3 override 解析器', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-3a: platform override creates impersonation and ordinary users cannot override tenant context', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0204();
  });
});
