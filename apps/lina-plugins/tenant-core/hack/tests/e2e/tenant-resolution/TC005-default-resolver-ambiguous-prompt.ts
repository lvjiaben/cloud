import { test, expect } from '../../support/tenant-core';
import { scenarioTC0202 } from '../../support/tenant-core-scenarios';

test.describe('TC-1 default 解析器 ambiguous prompt', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-1a: ambiguous login returns preToken and tenant candidates', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0202();
  });
});
