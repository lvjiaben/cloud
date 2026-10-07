import { test, expect } from '@host-tests/fixtures/auth';
import { smokeSourcePluginLifecycle } from '@host-tests/support/source-plugin-lifecycle';

test.describe('TC006 组织插件生命周期 smoke', () => {
  test('TC-2a: 组织插件可安装、启用、挂载菜单并访问页面', async ({
    adminPage,
  }) => {
    await smokeSourcePluginLifecycle(adminPage, {
      id: 'org-core',
      mountedTitles: ['部门管理', '岗位管理'],
      route: '/system/dept',
      assertAvailable: async (page) => {
        await expect(page.locator('.vxe-table')).toBeVisible({ timeout: 10000 });
        await expect(page.getByRole('button', { name: /新\s*增/ }).first()).toBeVisible();
      },
    });
  });
});

