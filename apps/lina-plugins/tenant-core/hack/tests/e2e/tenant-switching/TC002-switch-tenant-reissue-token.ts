import { test, expect } from '../../support/tenant-core';
import { scenarioTC0185 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 切换租户重签 token', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: switch-tenant reissues token and revokes the previous token', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0185();
  });
});
