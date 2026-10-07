import { test } from '@host-tests/fixtures/auth';
import { MultiTenantPage } from '../../pages/MultiTenantPage';

test.describe('TC-2 多租户管理工作台页面路由', () => {
  test('TC-2a: platform tenant management stays visible', async ({
    page,
  }) => {
    test.setTimeout(180_000);
    const multiTenantPage = new MultiTenantPage(page);

    await multiTenantPage.gotoPlatformTenants();
    await multiTenantPage.expectPlatformTenantWorkbench();
  });

  test('TC-2b: platform user management exposes tenant controls', async ({
    page,
  }) => {
    const multiTenantPage = new MultiTenantPage(page);

    await multiTenantPage.gotoSystemUsers();
    await multiTenantPage.expectSystemUserTenantWorkbench();
  });

  test('TC-2c: tenant member management uses the user page', async ({
    page,
  }) => {
    const multiTenantPage = new MultiTenantPage(page);
    await multiTenantPage.expectTenantMemberManagementUsesUserPage();
  });

  test('TC-2d: tenant switch enters the tenant workbench', async ({
    page,
  }) => {
    const multiTenantPage = new MultiTenantPage(page);
    await multiTenantPage.exerciseTenantSwitch();
  });

  test('TC-2e: platform impersonation can enter and exit a tenant', async ({
    page,
  }) => {
    const multiTenantPage = new MultiTenantPage(page);
    await multiTenantPage.exerciseImpersonation();
  });

  test('TC-2f: obsolete tenant management routes fall back', async ({
    page,
  }) => {
    const multiTenantPage = new MultiTenantPage(page);
    await multiTenantPage.expectRemovedManagementRoutesFallback();
  });
});
