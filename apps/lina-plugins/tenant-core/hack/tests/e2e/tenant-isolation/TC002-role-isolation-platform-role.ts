import { test, expect } from '../../support/tenant-core';
import { scenarioTC0188 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 角色跨租户隔离', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: tenant roles and platform roles persist in disjoint tenant buckets', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0188();
  });
});
