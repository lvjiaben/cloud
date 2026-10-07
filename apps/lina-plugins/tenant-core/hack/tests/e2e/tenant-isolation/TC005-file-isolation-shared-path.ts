import { test, expect } from '../../support/tenant-core';
import { scenarioTC0191 } from '../../support/tenant-core-scenarios';

test.describe('TC-1 文件跨租户隔离', () => {
  test.use({ multiTenantMode: 'tenant-core-enabled' });

  test('TC-1a: file storage paths include tenant buckets and platform shared paths', async ({ multiTenantMode }) => {
    expect(multiTenantMode).toBe('tenant-core-enabled');
    await scenarioTC0191();
  });
});
