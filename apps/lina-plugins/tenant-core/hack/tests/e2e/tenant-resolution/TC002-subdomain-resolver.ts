import { test, expect } from '../../support/tenant-core';
import { scenarioTC0199 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 subdomain 解析器', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: subdomain root is fixed empty and reserved labels are enforced', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0199();
  });
});
