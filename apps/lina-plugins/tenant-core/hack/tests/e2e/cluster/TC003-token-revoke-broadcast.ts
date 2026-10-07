import { test, expect } from '../../support/tenant-core';
import { scenarioTC0220 } from '../../support/tenant-core-scenarios';

test.describe('TC-3 token revoke 广播', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-3a: old token is rejected after switch through shared revoke state', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0220();
  });
});
