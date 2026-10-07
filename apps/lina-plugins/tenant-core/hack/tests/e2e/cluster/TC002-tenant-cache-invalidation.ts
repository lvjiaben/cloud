import { test, expect } from '../../support/tenant-core';
import { scenarioTC0219 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 租户缓存失效隔离', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: cache revision rows are isolated by tenant scope', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0219();
  });
});
