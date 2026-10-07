import { test, expect } from '../../support/tenant-core';
import { scenarioTC0198 } from '../../support/tenant-core-scenarios';

test.describe('TC-1 header 解析器', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-1a: header hint is configured while formal JWT remains authoritative', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0198();
  });
});
