import { SSHKeysPage } from '../pages/SSHKeysPage';
import { expect, test } from '../fixtures/customer';
import { installCustomerSSHKeyMock } from '../support/customer-ssh-key-api-mock';

const publicKey =
  'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeButBrowserSafePublicKeyMaterial customer@example';

test.describe('TC017 客户 SSH 公钥', () => {
  test('TC017a: 私钥在浏览器端直接拒绝且不会提交接口', async ({ page }) => {
    const state = await installCustomerSSHKeyMock(page);
    const sshKeys = new SSHKeysPage(page);
    await sshKeys.login();
    await sshKeys.open();
    const gridHeight = await page
      .getByTestId('ssh-key-table')
      .evaluate((element) => element.getBoundingClientRect().height);
    expect(gridHeight).toBeGreaterThanOrEqual(320);
    const dialog = await sshKeys.openCreate();
    await dialog.getByRole('textbox').nth(0).fill('Unsafe private key');
    await dialog
      .getByRole('textbox')
      .nth(1)
      .fill('-----BEGIN OPENSSH PRIVATE KEY-----\nsecret');
    await dialog
      .getByRole('button', { name: /保\s*存/u })
      .click({ noWaitAfter: true });

    await expect(page.getByText('系统没有上传它')).toBeVisible();
    expect(state.createBodies).toHaveLength(0);
    await sshKeys.screenshot('customer-ssh-private-key-rejected-zh');
  });

  test('TC017b: 只展示指纹，撤销时提交当前版本且不回显公钥正文', async ({
    page,
  }) => {
    const state = await installCustomerSSHKeyMock(page);
    const sshKeys = new SSHKeysPage(page);
    await sshKeys.login();
    await sshKeys.open();
    const dialog = await sshKeys.openCreate();
    await dialog.getByRole('textbox').nth(0).fill('Production access');
    await dialog.getByRole('textbox').nth(1).fill(publicKey);
    await dialog
      .getByRole('button', { name: /保\s*存/u })
      .click({ noWaitAfter: true });

    await expect(sshKeys.root).toContainText('SHA256:customerSafeFingerprint');
    await expect(sshKeys.root).not.toContainText(
      'FakeButBrowserSafePublicKeyMaterial',
    );
    expect(state.createBodies).toHaveLength(1);
    expect(state.createBodies[0]).toMatchObject({
      authorizedKey: publicKey,
      name: 'Production access',
    });

    await sshKeys.root.getByRole('button', { name: '撤销' }).click();
    await page
      .locator('.ant-popconfirm-buttons .ant-btn-primary')
      .click();
    await expect(sshKeys.root).toContainText('已撤销');
    expect(state.revokeBodies[0]).toMatchObject({ expectedVersion: 1 });
  });
});

test.describe('TC017 customer SSH keys in English', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC017c: English mobile page stays translated and has no horizontal overflow', async ({
    page,
  }) => {
    await installCustomerSSHKeyMock(page, { locale: 'en-US' });
    const sshKeys = new SSHKeysPage(page);
    await sshKeys.login();
    await sshKeys.open();
    await expect(sshKeys.root).toContainText(
      'Your private key always stays on your device',
    );
    await expect(sshKeys.root).not.toContainText('sshKeys.');
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth - window.innerWidth,
      ),
    ).toBeLessThanOrEqual(1);
    await sshKeys.screenshot('customer-ssh-keys-en-mobile');
  });
});
