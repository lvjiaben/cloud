import { test, expect } from '../../support/tenant-core';
import { scenarioTC0195 } from '../../support/tenant-core-scenarios';

test.describe('TC-5 审计日志隔离与 impersonation 标记', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-5a: login and operation logs keep tenant and impersonation fields', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0195();
  });
});
