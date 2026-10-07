import { test, expect } from '../../support/tenant-core';
import { scenarioTC0201 } from '../../support/tenant-core-scenarios';

test.describe('TC-4 session 解析器', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-4a: switch flow persists tenant session and revokes the old one', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0201();
  });
});
