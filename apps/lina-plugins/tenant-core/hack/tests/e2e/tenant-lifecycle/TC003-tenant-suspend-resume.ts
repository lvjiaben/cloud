import { test, expect } from '../../support/tenant-core';
import { scenarioTC0180 } from '../../support/tenant-core-scenarios';

test.describe('TC-3 租户暂停恢复', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-3a: suspended tenant blocks login and resumed tenant allows it again', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0180();
  });
});
