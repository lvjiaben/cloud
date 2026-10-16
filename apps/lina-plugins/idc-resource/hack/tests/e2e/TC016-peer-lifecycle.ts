import { expect, test } from '@host-tests/fixtures/auth';
import type { Page } from '@host-tests/support/playwright';
import { PeerLifecyclePage } from '../pages/PeerLifecyclePage';
import { installResourceWorkspaceMenus } from '../support/resource-workspace-mock';

const basePermissions = ['idc:admin:resource:configuration:list'];
const pluginPermissions = ['plugin:query', 'plugin:business-impact', 'plugin:enable'];
async function fixture(page: Page, permissions = pluginPermissions) {
  await installResourceWorkspaceMenus(page, [...basePermissions, ...permissions]);
  await page.route('**/x/idc-resource/api/v1/admin/resource/provider-configurations**', route => route.fulfill({ json: { code: 0, data: { channels: [], pools: [], channelTotal: 0, poolTotal: 0, providerCandidates: [] } } }));
  const state = { enable: 0, detailReads: 0, impactReads: 0, failure: false, enableFailure: false, malformed: false, changed: false, detail: { id: 'idc-peer', installed: 1, enabled: 1, type: 'dynamic', distribution: 'managed', runtimeState: 'normal', effectiveVersion: 'v0.1.3', discoveredVersion: 'v0.1.3', version: 'v0.1.3', upgradeAvailable: false, newBusinessState: 'stopped' } };
  await page.route('**/api/v1/plugins/idc-peer**', async route => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith('/enable')) { state.enable++; if (state.enableFailure) { await route.fulfill({ status: 503, json: { code: 503, message: 'Fixture write unavailable' } }); return; } await route.fulfill({ json: { code: 0, data: { id: 'idc-peer', enabled: 1 } } }); return; }
    if (url.pathname.endsWith('/business-impact')) {
      state.impactReads++;
      if (state.failure) { await route.fulfill({ status: 503, json: { code: 503, message: 'Fixture unavailable' } }); return; }
      await route.fulfill({ json: { code: 0, data: { id: 'idc-peer', blockingCount: 2, items: [{ totalCount: state.malformed ? 1 : 4, blockingCount: 2 }] } } }); return;
    }
    state.detailReads++;
    await route.fulfill({ json: { code: 0, data: { ...state.detail, newBusinessState: state.changed && state.enable ? 'accepting' : state.detail.newBusinessState } } });
  });
  const screen = new PeerLifecyclePage(page); await screen.goto();
  if (permissions.includes('plugin:query') && permissions.includes('plugin:business-impact')) await screen.open();
  return { state, screen, root: screen.card };
}

test.describe('TC-16 同系统连接器显式授权确认', () => {
  test('TC-16a: 权限必须全部具备，禁止自动启用', async ({ adminPage }) => {
    const { state, root, screen } = await fixture(adminPage, ['plugin:query']);
    await expect(root).toHaveCount(0); await expect(screen.entry).toHaveCount(0); expect(state.detailReads).toBe(0); expect(state.enable).toBe(0);
  });
  test('TC-16b: 只读检查不显示写入口，失败清除旧结果', async ({ adminPage }) => {
    const { state, root, screen } = await fixture(adminPage, ['plugin:query', 'plugin:business-impact']);
    await expect(screen.reconfirmButton).toHaveCount(0);
    await screen.check();
    await expect(screen.result).toContainText('未处理完的业务数量：2');
    state.failure = true; await screen.check();
    await expect(screen.error).toContainText('检查未完成，不能据此停用');
    await expect(screen.result).toHaveCount(0); expect(state.enable).toBe(0);
    await screen.screenshot('peer-lifecycle-failed-closed');
  });
  test('TC-16c: 升级和停用状态拒绝，确认成功仍保持停止新业务', async ({ adminPage, mainLayout }) => {
    const { state, root, screen } = await fixture(adminPage);
    state.detail.runtimeState = 'pending_upgrade';
    await screen.reconfirm();
    await expect(screen.error).toContainText('请先到插件管理检查'); expect(state.enable).toBe(0);
    state.detail.runtimeState = 'normal'; state.detail.enabled = 0;
    await screen.reconfirm();
    await expect(screen.error).toBeVisible(); expect(state.enable).toBe(0);
    state.detail.enabled = 1;
    await screen.reconfirm();
    const dialog = screen.dialog;
    await expect(dialog).toContainText('可能刷新同版本构建'); await expect(dialog).toContainText('不会恢复已经停止的新业务');
    await screen.screenshot('peer-lifecycle-confirm-zh');
    await screen.confirm();
    await expect(dialog).toBeHidden(); await expect(screen.result).toContainText('新业务仍保持停止状态');
    await expect(root).not.toContainText('plugin.idc-resource'); expect(state.enable).toBe(1);
    await screen.screenshot('peer-lifecycle-success-stopped');
    await screen.close();
    await mainLayout.switchLanguage('English');
    await screen.open();
    await expect(screen.reconfirmButton).toBeVisible();
    await expect(root).toContainText('This action only handles the peer connector');
    await screen.reconfirm();
    await expect(screen.dialog).toContainText('may refresh a build with the same version');
    await expect(root).not.toContainText('plugin.idc-resource');
    await screen.screenshot('peer-lifecycle-confirm-en');
    await screen.cancelConfirm(); await screen.close();
    await expect(screen.card).toBeHidden(); await expect(screen.pager).toBeInViewport();
    await test.info().attach('provider-pager-bounds', { body: JSON.stringify({ pager: await screen.pager.boundingBox(), viewport: adminPage.viewportSize() }), contentType: 'application/json' });
    await screen.screenshot('peer-lifecycle-page-pager');
  });
  test('TC-16d: 启用后状态变化拒绝沿用检查结果', async ({ adminPage }) => {
    const { state, root, screen } = await fixture(adminPage); state.changed = true;
    await screen.reconfirm();
    await screen.confirm();
    await expect(screen.error).toContainText('旧结果已清空');
    await expect(screen.result).toHaveCount(0); expect(state.impactReads).toBe(0);
  });
  test('TC-16e: 授权写失败和畸形统计均不显示排空', async ({ adminPage }) => {
    const { state, screen } = await fixture(adminPage);
    state.malformed = true; await screen.check();
    await expect(screen.error).toBeVisible(); await expect(screen.result).toHaveCount(0);
    state.malformed = false; state.enableFailure = true;
    await screen.reconfirm(); await screen.confirm();
    await expect(screen.error).toContainText('旧结果已清空');
    await expect(screen.result).toHaveCount(0); expect(state.enable).toBe(1); expect(state.impactReads).toBe(1);
  });
});
