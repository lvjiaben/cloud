import type { Page, Route } from '@playwright/test';

import { test, expect } from '../../../fixtures/auth';
import { PluginPage } from '../../../pages/PluginPage';

const pluginID = 'plugin-new-business-lifecycle-e2e';

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: 'success' };
}

function managedPluginRow() {
  return {
    abnormalReason: '',
    authorizationRequired: 0,
    authorizationStatus: 'not_required',
    autoEnableForNewTenants: false,
    autoEnableManaged: 0,
    authorizedHostServices: [],
    declaredRoutes: [],
    dependencyCheck: null,
    description: 'Verifies the separate new-business lifecycle.',
    discoveredVersion: 'v0.1.0',
    distribution: 'managed',
    effectiveVersion: 'v0.1.0',
    enabled: 1,
    hasMockData: 0,
    id: pluginID,
    installMode: 'global',
    installed: 1,
    installedAt: '',
    name: 'Plugin New Business Lifecycle E2E',
    newBusinessState: 'accepting',
    newBusinessStoppedAt: 0,
    requestedHostServices: [],
    runtimeState: 'normal',
    scopeNature: 'global',
    statusKey: 'enabled',
    supportsMultiTenant: false,
    type: 'dynamic',
    updatedAt: '',
    upgradeAvailable: false,
    version: 'v0.1.0',
  };
}

async function mockLifecycleApis(page: Page) {
  const row = managedPluginRow();

  await page.route('**/api/v1/plugins**', async (route: Route) => {
    const request = route.request();
    const requestPath = new URL(request.url()).pathname;

    if (request.method() === 'GET' && /\/api\/v1\/plugins$/u.test(requestPath)) {
      await route.fulfill({
        json: apiEnvelope({ list: [row], total: 1 }),
      });
      return;
    }
    if (request.method() === 'GET' && requestPath.endsWith('/plugins/dynamic')) {
      await route.fulfill({ json: apiEnvelope({ list: [] }) });
      return;
    }
    if (
      request.method() === 'PUT' &&
      requestPath.endsWith(`/plugins/${pluginID}/new-business/stop`)
    ) {
      row.newBusinessState = 'stopped';
      row.newBusinessStoppedAt = Date.now();
      await route.fulfill({ json: apiEnvelope(null) });
      return;
    }
    if (
      request.method() === 'PUT' &&
      requestPath.endsWith(`/plugins/${pluginID}/new-business/resume`)
    ) {
      row.newBusinessState = 'accepting';
      await route.fulfill({ json: apiEnvelope(null) });
      return;
    }

    await route.continue();
  });
}

test.describe('TC-20 插件新业务生命周期', () => {
  test('TC-20a~d: 停售确认、状态持久、既有管理入口保留并可恢复', async ({
    adminPage,
  }) => {
    await mockLifecycleApis(adminPage);
    const pluginPage = new PluginPage(adminPage);
    await pluginPage.gotoManage();
    await pluginPage.searchByPluginId(pluginID);
    await pluginPage.showPluginLifecycleColumns();

    const newBusinessSwitch = pluginPage.pluginNewBusinessSwitch(pluginID);
    const enabledSwitch = pluginPage.pluginEnabledSwitch(pluginID);
    await expect(newBusinessSwitch).toHaveAttribute('aria-checked', 'true');
    await expect(enabledSwitch).toHaveAttribute('aria-checked', 'true');

    await newBusinessSwitch.click();
    const stopDialog = pluginPage.newBusinessStopDialog();
    await expect(stopDialog).toContainText('已有服务不会被隐藏或停用');
    await pluginPage.newBusinessStopConfirmButton().click();

    await expect(newBusinessSwitch).toHaveAttribute('aria-checked', 'false');
    await expect(
      pluginPage.messageNotice('已停止新销售，已有服务继续可用'),
    ).toBeVisible();
    await expect(pluginPage.pluginRow(pluginID)).toBeVisible();
    await expect(enabledSwitch).toHaveAttribute('aria-checked', 'true');

    await adminPage.reload({ waitUntil: 'domcontentloaded' });
    await pluginPage.searchByPluginId(pluginID);
    await pluginPage.showPluginLifecycleColumns();
    await expect(pluginPage.pluginNewBusinessSwitch(pluginID)).toHaveAttribute(
      'aria-checked',
      'false',
    );

    await pluginPage.pluginNewBusinessSwitch(pluginID).click();
    await expect(pluginPage.pluginNewBusinessSwitch(pluginID)).toHaveAttribute(
      'aria-checked',
      'true',
    );
    await expect(pluginPage.messageNotice('已恢复接受新业务')).toBeVisible();
  });

  test('TC-20f: 英文界面使用完整业务文案而不是内部枚举', async ({
    adminPage,
    mainLayout,
  }) => {
    await mockLifecycleApis(adminPage);
    await mainLayout.switchLanguage('English');

    const pluginPage = new PluginPage(adminPage);
    await pluginPage.gotoManage();
    await pluginPage.searchByPluginId(pluginID);
    await pluginPage.showPluginLifecycleColumns();
    await expect(pluginPage.tableColumn('New Business')).toBeVisible();
    await expect(pluginPage.pluginNewBusinessSwitch(pluginID)).toContainText(
      'Selling',
    );

    await pluginPage.pluginNewBusinessSwitch(pluginID).click();
    await expect(pluginPage.newBusinessStopDialog()).toContainText(
      'Existing services remain visible and available.',
    );
    await pluginPage
      .newBusinessStopDialog()
      .getByRole('button', { name: 'Cancel' })
      .click();
  });

});
