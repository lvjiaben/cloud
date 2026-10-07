import { readFileSync } from 'node:fs';
import { expect, test } from '../../../../../../hack/tests/fixtures/auth';
import { IsolatedMountPage } from '../../../../vben-adapter/hack/tests/pages/IsolatedMountPage';

const catalogs = Object.fromEntries(['zh-CN', 'en-US'].map(locale => [locale,
  JSON.parse(readFileSync(new URL(`../../../manifest/i18n/${locale}/plugin.json`, import.meta.url), 'utf8')) as Record<string, string>,
]));

test.describe('TC-10 独立资源配置使用官方 Vben 表格', () => {
  test('TC-10a: 接口与资源池表保留业务状态、编辑重开和白色面板', async ({ adminPage }) => {
    const channel = { channelNo: 'CHANNEL1', displayName: '演示接口', endpoint: 'https://provider.example.invalid', providerPluginId: 'demo', status: 'enabled', connectionStatus: 'healthy', secretFields: [], version: 3 };
    const requests: unknown[] = [];
    let rejectNext = false;
    await adminPage.route('**/x/idc-resource/api/v1/admin/resource/provider-configurations**', async route => {
      if (route.request().method() !== 'GET') {
        requests.push(route.request().postDataJSON());
        if (rejectNext) {
          await route.fulfill({ status: 403, json: { code: 403, message: '本次测试禁止修改' } });
          return;
        }
        channel.status = route.request().postDataJSON().status;
      }
      await route.fulfill({ json: { code: 0, data: { channelTotal: 1, poolTotal: 1, channels: [{ ...channel, usage: { activeInstances: 3, totalInstances: 5, configuredProducts: 2 } }], pools: [{ poolNo: 'POOL1', displayName: '演示资源池', channelNo: 'CHANNEL1', channelName: '演示接口', resourceKind: 'virtual_server', routingPriority: 7, routingWeight: 13, status: 'enabled', version: 2 }], providerCandidates: [] } } });
    });
    const surface = new IsolatedMountPage(adminPage);
    try {
      await surface.mount('/x-assets/idc-resource/v0.1.25/mount.js', '/idc/resource/provider-configuration', 'zh-CN', catalogs['zh-CN']);
      await expect(surface.rows).toHaveCount(1);
      await expect(surface.headers).toContainText('接口地址');
      await expect(surface.rows).toContainText('演示接口');
      await expect(surface.rows).toContainText('3 / 5');
      await expect(surface.root.locator('.vxe-pager--total')).toContainText('1');
      expect((await surface.whiteSurfaces()).every(color => color === 'rgb(255, 255, 255)')).toBe(true);
      await surface.screenshot('resource-vben-channel');
      await surface.click('详情');
      const detail = adminPage.getByRole('dialog', { name: '服务器接口详情' });
      await expect(detail).toBeVisible();
      await expect(detail.getByTestId('provider-interface-details')).toContainText('3 / 5');
      await expect(detail).not.toContainText('plugin.idc-resource');
      await surface.screenshot('resource-vben-detail');
      await adminPage.keyboard.press('Escape');
      await expect(detail).toBeHidden();
      for (let i = 0; i < 2; i++) {
        await surface.click('编辑');
        const editor = adminPage.getByRole('dialog', { name: '编辑接口' });
        await expect(editor).toBeVisible();
        await expect(editor.getByRole('textbox', { name: '接口名称' })).toHaveValue('演示接口');
        await surface.screenshot(`resource-vben-editor-${i}`);
        await editor.getByRole('button', { name: '取消', exact: true }).click();
        await expect(editor).toBeHidden();
      }
      await surface.click('停用');
      await expect.poll(() => requests.length).toBe(1);
      expect(requests[0]).toMatchObject({ status: 'disabled', version: 3 });
      await expect(surface.rows).toContainText('停用');
      rejectNext = true;
      await surface.click('删除');
      await expect(surface.root.locator('.notice.error')).toContainText('本次测试禁止修改');
      await expect(surface.rows).toHaveCount(1);
      await surface.screenshot('resource-vben-denied');
      await surface.click('资源池绑定');
      await expect(surface.rows).toContainText('演示资源池');
      await expect(surface.rows).toContainText('7 / 13');
      await surface.setLocale('en-US', catalogs['en-US']);
      await expect(surface.headers).toContainText('Priority / weight');
      await expect(surface.rows).toContainText('Cloud server');
      expect((await surface.whiteSurfaces()).every(color => color === 'rgb(255, 255, 255)')).toBe(true);
      await surface.screenshot('resource-vben-pool-en');
    } finally { await surface.dispose(); }
  });

  test('TC-10b: 官方抽屉校验、保存失败重试、资源池新增与凭据关闭清空', async ({ adminPage }) => {
    const configuration = { version: '1', fields: [
      { key: 'username', type: 'single_line', required: true, label: { zhCN: '账号', enUS: 'Username' } },
      { key: 'password', type: 'secret', required: true, label: { zhCN: '密码', enUS: 'Password' } },
    ] };
    const channel = { channelNo: 'CHANNEL1', displayName: '演示接口', endpoint: 'https://provider.example.invalid', providerPluginId: 'demo', status: 'disabled', connectionStatus: 'healthy', interfaceConfiguration: configuration, nonSecretSettings: { username: 'fixture-user' }, secretFields: [{ key: 'password', required: true, ready: true }], version: 3 };
    const pools: Record<string, unknown>[] = [];
    const writes: Array<{ method: string; body: Record<string, unknown> }> = [];
    let failNext = false;
    await adminPage.route('**/x/idc-resource/api/v1/admin/resource/provider-configurations**', async route => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ json: { code: 0, data: { channelTotal: 1, poolTotal: pools.length, channels: [channel], pools, providerCandidates: [{ pluginId: 'demo', displayName: '演示连接器', releaseVersion: '1.0.0', interfaceConfiguration: configuration }] } } });
        return;
      }
      const body = route.request().postDataJSON();
      writes.push({ method: route.request().method(), body });
      if (failNext) { failNext = false; await route.fulfill({ status: 503, json: { code: 503, message: '测试保存暂时不可用' } }); return; }
      if (new URL(route.request().url()).pathname.endsWith('/pools')) {
        const pool = { ...body, poolNo: 'POOL1', channelName: channel.displayName, version: 1 };
        pools.push(pool);
        await route.fulfill({ json: { code: 0, data: { item: pool } } });
      } else {
        Object.assign(channel, body, { version: channel.version + 1 });
        await route.fulfill({ json: { code: 0, data: { item: channel } } });
      }
    });
    const surface = new IsolatedMountPage(adminPage);
    try {
      await surface.mount('/x-assets/idc-resource/v0.1.25/mount.js', '/idc/resource/provider-configuration', 'zh-CN', catalogs['zh-CN']);
      const documentState = await adminPage.evaluate(() => ({ html: document.documentElement.getAttribute('class'), theme: document.documentElement.getAttribute('data-theme'), lang: document.documentElement.lang }));
      await surface.click('编辑');
      const editor = adminPage.getByRole('dialog', { name: '编辑接口' });
      await expect(editor.getByLabel('账号')).toHaveValue('fixture-user');
      await editor.getByLabel('接口名称').fill('');
      await editor.getByRole('button', { name: '确认', exact: true }).click();
      await expect(editor).toContainText('请输入接口名称');
      expect(writes).toHaveLength(0);
      await editor.getByLabel('接口名称').fill('修改后的接口');
      failNext = true;
      await editor.getByRole('button', { name: '确认', exact: true }).click();
      await expect(editor.getByRole('alert').filter({ hasText: '测试保存暂时不可用' })).toBeVisible();
      await expect(editor.getByLabel('接口名称')).toHaveValue('修改后的接口');
      await surface.screenshot('resource-drawer-save-failed');
      await editor.getByRole('button', { name: '确认', exact: true }).click();
      await expect(editor).toBeHidden();
      await expect(surface.rows).toContainText('修改后的接口');
      expect(writes).toHaveLength(2);
      expect(writes[1]).toMatchObject({ method: 'PUT', body: { displayName: '修改后的接口', version: 3, nonSecretSettings: { username: 'fixture-user' } } });
      expect(writes[1].body).not.toHaveProperty('password');
      await surface.screenshot('resource-drawer-saved');
      await surface.click('编辑');
      await editor.getByLabel('密码').fill('fixture-secret-never-sent');
      await editor.getByRole('button', { name: '取消', exact: true }).click();
      await expect(editor).toBeHidden();
      await surface.click('编辑');
      await expect(editor.getByLabel('密码')).toHaveValue('');
      await editor.getByRole('button', { name: '取消', exact: true }).click();
      await expect(editor).toBeHidden();
      await surface.click('资源池绑定');
      await surface.click('新建资源池');
      const poolEditor = adminPage.getByRole('dialog', { name: '新建资源池' });
      await poolEditor.getByLabel('资源池名称').fill('新增测试资源池');
      await poolEditor.getByRole('button', { name: '确认', exact: true }).click();
      await expect(poolEditor).toBeHidden();
      await expect(surface.rows).toContainText('新增测试资源池');
      expect(writes.at(-1)).toMatchObject({ method: 'POST', body: { displayName: '新增测试资源池', channelNo: 'CHANNEL1', resourceKind: 'virtual_server', status: 'disabled', routingPriority: 100, routingWeight: 100 } });
      await surface.screenshot('resource-drawer-pool-created');
      expect(await adminPage.evaluate(() => ({ html: document.documentElement.getAttribute('class'), theme: document.documentElement.getAttribute('data-theme'), lang: document.documentElement.lang }))).toEqual(documentState);
    } finally { await surface.dispose(); }
  });
});
