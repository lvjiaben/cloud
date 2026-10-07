import { test, expect } from '../../support/tenant-core';
import { scenarioTC0203 } from '../../support/tenant-core-scenarios';

test.describe('TC-2 固定 prompt 歧义策略', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-2a: resolver policy stays code-owned and rejects reject mode', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0203();
  });
});
