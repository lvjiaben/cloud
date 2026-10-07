import { test } from '@host-tests/fixtures/auth';
import { MultiTenantPage } from '../../pages/MultiTenantPage';

test.describe('TC-3 多租户上下文刷新默认页', () => {
  test('TC-3a: tenant context changes refresh permissions and enter default pages', async ({
    page,
  }) => {
    const multiTenantPage = new MultiTenantPage(page);

    await multiTenantPage.exerciseDirectImpersonationDefaultRoute();
    await multiTenantPage.exerciseTenantSwitch();
    await multiTenantPage.exerciseTenantUserSwitch();
  });
});
