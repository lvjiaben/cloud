import { ServersPage } from '../pages/ServersPage';
import {
  installCustomerServerMock,
  installCustomerTrafficPackageMock,
  reinstallOperationNo,
  serverServiceNo,
} from '../support/customer-server-api-mock';
import { expect, test } from '../fixtures/customer';

test.describe('TC015 客户服务器日常操作', () => {
  test('TC015a: 重装必须选真实镜像且高级设置默认不改', async ({ page }) => {
    const state = await installCustomerServerMock(page);
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();
    await servers.screenshot('customer-server-detail-loaded-zh');

    await page.getByTestId('server-action-reinstall').click();
    const operationDialog = page
      .getByRole('dialog')
      .filter({ hasText: '重装系统' });
    await expect(operationDialog).toContainText('重装会覆盖当前系统');
    await expect(operationDialog).toContainText('高级设置（默认不更改）');
    await servers.screenshot('customer-server-reinstall-dialog-zh');
    await operationDialog.getByRole('button', { name: /确\s*认/ }).click();
    await expect(page.getByText('请选择可用镜像')).toBeVisible();
    await servers.screenshot('customer-server-reinstall-image-required-zh');
    expect(state.authorizationBodies).toHaveLength(0);

    await page.getByTestId('server-reinstall-image').click();
    await page.getByText('Linux / Ubuntu 24.04 LTS', { exact: false }).click();
    await operationDialog.getByRole('button', { name: /确\s*认/ }).click();
    const confirmDialog = page
      .getByRole('dialog')
      .filter({ hasText: '请再次确认本次操作' });
    await confirmDialog.getByRole('button', { name: '确认执行' }).click();

    await expect.poll(() => state.createBodies.length).toBe(1);
    expect(state.authorizationBodies[0]).toMatchObject({
      action: 'reinstall',
      parameters: { os: 101 },
    });
    expect(state.createBodies[0]).toMatchObject({
      action: 'reinstall',
      parameters: { os: 101 },
      proofToken: 'server-operation-proof',
    });
    expect(state.createBodies[0]?.parameters).not.toHaveProperty('port');
    expect(state.createBodies[0]?.parameters).not.toHaveProperty(
      'format_data_disk',
    );
    expect(state.createBodies[0]?.parameters).not.toHaveProperty(
      'system_disk_size',
    );
    await expect(confirmDialog).toHaveCount(0);
    await expect(operationDialog).toHaveCount(0);
    await expect(page.getByText('请选择可用镜像')).toHaveCount(0, {
      timeout: 5_000,
    });
    await servers.screenshot('customer-server-reinstall-submitted-zh');
  });

  test('TC015b: 新密码由客户主动确认后只查看一次', async ({ page }) => {
    const state = await installCustomerServerMock(page);
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();

    await page.getByTestId('server-action-reinstall').click();
    await page.getByTestId('server-reinstall-image').click();
    await page.getByText('Linux / Ubuntu 24.04 LTS', { exact: false }).click();
    await page
      .getByRole('dialog')
      .filter({ hasText: '重装系统' })
      .getByRole('button', { name: /确\s*认/ })
      .click();
    await page
      .getByRole('dialog')
      .filter({ hasText: '请再次确认本次操作' })
      .getByRole('button', { name: '确认执行' })
      .click();

    const reveal = page.getByTestId(`server-output-${reinstallOperationNo}`);
    await expect(reveal).toHaveText('查看新密码（仅一次）');
    await reveal.click();
    await expect(page.getByText('密码只会显示这一次')).toBeVisible();
    expect(state.outputRequests).toBe(0);
    await page
      .getByRole('dialog')
      .filter({ hasText: '密码只会显示这一次' })
      .getByRole('button', { name: '查看新密码（仅一次）' })
      .click();

    const credential = page.getByTestId('server-credential-content');
    await expect(credential).toContainText('用户名: root');
    await expect(credential).toContainText('新密码: Temporary-Password-1!');
    await expect(credential).toContainText('关闭后将永久不能再次查看');
    expect(state.outputRequests).toBe(1);
    await servers.screenshot('customer-server-one-time-password-zh');
    await page
      .getByRole('dialog')
      .filter({ has: credential })
      .getByRole('button', { name: '我已安全保存，关闭' })
      .click();
    await expect(reveal).toHaveCount(0);
    expect(state.outputRequests).toBe(1);
  });

  test('TC015c: 普通开机直接提交且不出现确认框', async ({ page }) => {
    const state = await installCustomerServerMock(page, {
      runtimeStatus: 'stopped',
    });
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();

    await page.getByTestId('server-action-start').click();
    await expect.poll(() => state.createBodies.length).toBe(1);
    expect(state.authorizationBodies[0]).toMatchObject({
      action: 'start',
      parameters: {},
    });
    expect(state.createBodies[0]).toMatchObject({
      action: 'start',
      parameters: {},
    });
    await expect(page.getByText('请再次确认本次操作')).toHaveCount(0);
  });

  test('TC015e: 免费快照仍先确认服务端报价再创建', async ({ page }) => {
    const state = await installCustomerServerMock(page);
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();

    await expect(page.getByTestId('customer-server-accessories')).toContainText(
      '203.0.113.10',
    );
    await servers.screenshot('customer-server-accessories-loaded-zh');
    await page.getByTestId('server-action-snapshot_create').click();
    const actionDialog = page
      .getByRole('dialog')
      .filter({ hasText: '创建快照' });
    await page.getByTestId('server-accessory-object').click();
    await page.getByText('System disk · 80 GiB', { exact: true }).click();
    await page.getByTestId('server-accessory-name').fill('before-release');
    await servers.screenshot('customer-server-snapshot-create-dialog-zh');
    await actionDialog.getByRole('button', { name: /确\s*认/ }).click();
    await page
      .getByRole('dialog')
      .filter({ hasText: '请再次确认本次操作' })
      .getByRole('button', { name: '确认执行' })
      .click();
    const quoteDialog = page
      .getByRole('dialog')
      .filter({ hasText: '本次数量' });
    await expect(quoteDialog).toContainText('¥0.00');
    await servers.screenshot('customer-server-snapshot-quote-zh');
    await quoteDialog.getByRole('button', { name: '确认报价并继续' }).click();

    await expect.poll(() => state.checkoutBodies.length).toBe(1);
    expect(state.quoteBodies[0]).toMatchObject({
      action: 'snapshot_create',
      parameters: { diskObjectNo: 'disk-0001', name: 'before-release' },
    });
  });

  test('TC015f: 快照恢复必须输入当前服务编号', async ({ page }) => {
    const state = await installCustomerServerMock(page, {
      runtimeStatus: 'stopped',
    });
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();

    await page.getByTestId('server-action-snapshot_restore').click();
    const actionDialog = page
      .getByRole('dialog')
      .filter({ hasText: '从快照恢复' });
    await page.getByTestId('server-accessory-object').click();
    await page.getByText('Before upgrade', { exact: true }).click();
    await actionDialog.getByRole('button', { name: /确\s*认/ }).click();
    await expect(
      page.getByText('输入的服务编号与当前服务器不一致'),
    ).toBeVisible();
    await servers.screenshot('customer-server-snapshot-restore-rejected-zh');
    expect(state.createBodies).toHaveLength(0);
  });

  test('TC015g: 转出申请不显示邮箱是否存在', async ({ page }) => {
    const state = await installCustomerServerMock(page);
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();

    await page.getByTestId('server-transfer-open').click();
    const dialog = page
      .getByRole('dialog')
      .filter({ hasText: '把服务器转给其他客户' });
    await expect(dialog).toContainText('无论邮箱是否存在');
    await dialog.getByRole('textbox').fill('recipient@example.test');
    await servers.screenshot('customer-server-transfer-request-zh');
    await dialog.getByRole('button', { name: '提交转移申请' }).click();

    await expect.poll(() => state.transferBodies.length).toBe(1);
    expect(state.transferBodies[0]).toMatchObject({
      recipientEmail: 'recipient@example.test',
    });
    await expect(
      page.getByText('转移申请已受理。如果邮箱对应符合条件的客户', {
        exact: false,
      }),
    ).toBeVisible();
  });

  test('TC015h: 接收方确认前明确历史资料不会转移', async ({ page }) => {
    const state = await installCustomerServerMock(page);
    const servers = new ServersPage(page);
    await servers.login();
    await page.goto('/account/servers');

    await expect(page.getByTestId('incoming-server-transfers')).toContainText(
      'SVC-INCOMING-0002',
    );
    await page.getByRole('button', { name: '确认接收' }).click();
    const dialog = page
      .getByRole('dialog')
      .filter({ hasText: '确认接收这台服务器' });
    await expect(dialog).toContainText('历史账单、合同、发票和工单不会转给你');
    await expect(dialog).toContainText(
      '已经付款、仍有效且只属于这台服务器的快照和备份会一同转入',
    );
    await expect(dialog).toContainText('自动续费会关闭');
    await servers.screenshot('customer-server-transfer-confirm-zh');
    await dialog.getByRole('button', { name: '确认接收' }).click();

    await expect.poll(() => state.confirmedTransfers.length).toBe(1);
    await expect(page.getByText('服务器已转入你的账号。')).toBeVisible();
  });

  test('TC015i: 服务商不支持附属操作时隐藏按钮并降级到关联工单', async ({
    page,
  }) => {
    await installCustomerServerMock(page, { availableActions: ['stop'] });
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();

    await expect(page.getByTestId('server-action-snapshot_create')).toHaveCount(
      0,
    );
    await expect(page.getByTestId('server-action-traffic_add')).toHaveCount(0);
    await expect(page.getByTestId('server-accessory-support')).toBeVisible();
    await page.getByTestId('server-accessory-support').click();
    await expect(page).toHaveURL(
      new RegExp(`/account/support/new\\?serviceNo=${serverServiceNo}`),
    );
  });

  test('TC015j: 升配先展示剩余周期补款并在付款后执行', async ({ page }) => {
    const state = await installCustomerServerMock(page);
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();

    await page.getByTestId('server-resize-open').click();
    const resizeDialog = page
      .getByRole('dialog')
      .filter({ hasText: '变更服务器配置' });
    await expect(resizeDialog).toContainText('磁盘容量只能增加，不能缩小');
    await page.getByTestId('server-resize-group-GROUP-CPU').click();
    await page.getByText('8 核', { exact: true }).click();
    await resizeDialog.getByRole('button', { name: '查看费用并继续' }).click();

    const quoteDialog = page
      .getByRole('dialog')
      .filter({ hasText: '确认立即升级' });
    await expect(quoteDialog).toContainText('当前周期剩余时间');
    await expect(quoteDialog).toContainText('¥12.50');
    await expect(quoteDialog).toContainText('付款后立即升级');
    await servers.screenshot('customer-server-resize-upgrade-quote-zh');
    await quoteDialog.getByRole('button', { name: '确认配置变更' }).click();

    await expect.poll(() => state.resizeQuoteBodies.length).toBe(1);
    expect(state.resizeQuoteBodies[0]).toMatchObject({
      currencyCode: 'CNY',
      revision: 'revision-signed',
      selections: expect.arrayContaining([
        { optionNo: 'OPT-CPU-8', quantity: 1 },
        { optionNo: 'OPT-DISK-80', quantity: 1 },
      ]),
    });
    await expect.poll(() => state.resizeCheckoutBodies.length).toBe(1);
    await expect(page).toHaveURL(
      /\/account\/billing\/invoices\/INV-RESIZE-0001/,
    );
  });
  test('TC015k: 固定容量、售罄禁买、无重置日期提示和权威报价', async ({ page }) => {
    const base = await installCustomerServerMock(page);
    const traffic = await installCustomerTrafficPackageMock(page);
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();
    await servers.openTrafficPackages();
    await expect(servers.trafficPackages).toContainText('有效期按上游重置规则执行，暂未提供具体到期时间');
    await expect(servers.trafficPackages.locator('.vxe-pager')).toContainText('共 2 条记录');
    const soldOut = servers.trafficPackages.locator('.vxe-table--main-wrapper .vxe-body--row').filter({ hasText: '售罄套餐' });
    await expect(soldOut.getByRole('button', { name: '购买' })).toBeDisabled();
    await servers.screenshot('customer-traffic-packages');
    await servers.selectTrafficPackage();
    await expect(servers.trafficConfirmation).toContainText('¥25.00');
    await expect(servers.trafficConfirmation).toContainText('100 GB');
    await expect(servers.trafficConfirmation).toContainText('有效期按上游重置规则执行，暂未提供具体到期时间');
    expect(traffic.quotes[0]).toMatchObject({ packageNo: 'tpk_customer_100', action: 'traffic_add', parameters: {} });
    await servers.screenshot('customer-traffic-confirmation');
    await servers.confirmTrafficPackage();
    await expect(page).toHaveURL(/\/account\/billing\/invoices\/INV-TRAFFIC-0001/u);
    expect(traffic.checkouts).toHaveLength(1);
    expect(base.createBodies).toHaveLength(0);
    expect(base.authorizationBodies).toHaveLength(0);
  });

  test('TC015l: 下单失败保留确认并用相同幂等键重试', async ({ page }) => {
    await installCustomerServerMock(page);
    const traffic = await installCustomerTrafficPackageMock(page);
    traffic.failCheckout = true;
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();
    await servers.openTrafficPackages();
    await servers.selectTrafficPackage();
    await servers.confirmTrafficPackage();
    await expect(page.getByText('下单结果暂时不可用，请重试', { exact: true })).toBeVisible();
    await expect(servers.trafficConfirmation).toBeVisible();
    await servers.screenshot('customer-traffic-checkout-error');
    await servers.confirmTrafficPackage();
    await expect(page).toHaveURL(/\/account\/billing\/invoices\/INV-TRAFFIC-0001/u);
    expect(traffic.checkouts).toHaveLength(2);
    expect(traffic.checkouts[0]).toEqual(traffic.checkouts[1]);
  });
});

test.describe('TC015 customer server operations on English mobile', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC015d: English mobile page is translated and has no horizontal overflow', async ({
    page,
  }) => {
    await installCustomerServerMock(page, { locale: 'en-US' });
    const servers = new ServersPage(page);
    await servers.login();
    await servers.openDetail();
    await expect(servers.root).toContainText('Server actions');
    await expect(servers.root).toContainText('Production Cloud Server');
    await expect(servers.root).not.toContainText('servers.');
    expect(await servers.hasHorizontalOverflow()).toBe(false);
    await servers.screenshot('customer-server-operations-en-mobile');
  });
});
