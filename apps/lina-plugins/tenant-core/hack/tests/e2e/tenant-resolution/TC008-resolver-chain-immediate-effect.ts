import { test, expect } from '../../support/tenant-core';
import { scenarioTC0205 } from '../../support/tenant-core-scenarios';

test.describe('TC-4 解析链固定策略', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-4a: removed resolver policy API leaves code-owned policy unchanged', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0205();
  });
});
