import { test, expect } from '../../support/tenant-core';
import { scenarioTC0193 } from '../../support/tenant-core-scenarios';

test.describe('TC-3 任务跨租户隔离', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-3a: tenant jobs and platform built-in jobs persist separately', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0193();
  });
});
