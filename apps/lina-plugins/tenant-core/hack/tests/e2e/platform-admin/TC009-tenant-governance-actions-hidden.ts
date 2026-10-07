import { test, expect } from '../../support/tenant-core';
import { scenarioTC0240 } from '../../support/tenant-core-scenarios';

test.describe('TC-5 租户态平台治理动作隐藏', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-5a: tenant context cannot use platform menu or plugin governance actions', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0240();
  });
});
