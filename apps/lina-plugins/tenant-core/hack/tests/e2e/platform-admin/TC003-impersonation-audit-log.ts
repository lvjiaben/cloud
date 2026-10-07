import { test, expect } from '../../support/tenant-core';
import { scenarioTC0215 } from '../../support/tenant-core-scenarios';

test.describe('TC-3 impersonation 审计日志', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-3a: impersonation writes dual-track audit fields', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0215();
  });
});
