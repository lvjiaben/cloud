import { test, expect } from '../../support/tenant-core';
import { scenarioTC0182 } from '../../support/tenant-core-scenarios';

test.describe('TC-1 租户删除生命周期前置条件', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-1a: active tenant delete passes lifecycle precondition before soft delete', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0182();
  });
});
