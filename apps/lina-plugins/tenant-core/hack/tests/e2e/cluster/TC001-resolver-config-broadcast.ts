import { test, expect } from '../../support/tenant-core';
import { scenarioTC0218 } from '../../support/tenant-core-scenarios';

test.describe('TC-1 解析策略无广播', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-1a: removed resolver policy API does not create shared revision', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0218();
  });
});
