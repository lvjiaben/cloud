import { test, expect } from '../../support/tenant-core';
import { scenarioTC0179 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 平台管理员创建租户', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: tenant CRUD validates code uniqueness and tombstones', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0179();
  });
});
