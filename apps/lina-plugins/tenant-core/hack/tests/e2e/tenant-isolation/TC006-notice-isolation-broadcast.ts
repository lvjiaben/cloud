import { test, expect } from '../../support/tenant-core';
import { scenarioTC0192 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 通知跨租户隔离', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: tenant notices and platform broadcast messages persist separately', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0192();
  });
});
