import { test, expect } from '../../support/tenant-core';
import { scenarioTC0241 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 租户任务分组隔离', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: tenant job groups are listed, created, updated, and migrated in tenant scope', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0241();
  });
});
