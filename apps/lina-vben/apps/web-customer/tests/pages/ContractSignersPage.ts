import type { Page } from '@playwright/test';

export class ContractSignersPage {
  constructor(readonly page: Page) {}

  get root() {
    return this.page.getByTestId('contract-signer-page');
  }

  get firstSigner() {
    return this.page.locator('.signer-card').first();
  }

  async open() {
    await this.page.goto('/account/contracts/signers');
    await this.root.waitFor({ state: 'visible' });
  }

  async addSigner() {
    await this.page.getByTestId('contract-signer-add').click();
    await this.page.getByTestId('contract-signer-name').fill('测试经办人');
    await this.page
      .getByTestId('contract-signer-identity')
      .fill('110105199001011234');
    await this.page.getByTestId('contract-signer-phone').fill('13800138000');
    await this.page.getByTestId('contract-signer-consent').check();
    await this.page.getByRole('button', { name: /确\s*认/ }).click();
    await this.firstSigner.waitFor({ state: 'visible' });
  }

  async startAndCheckIdentity() {
    const popupPromise = this.page.waitForEvent('popup');
    await this.firstSigner.getByRole('button', { name: '前往本人实名' }).click();
    const popup = await popupPromise;
    await popup.waitForLoadState('domcontentloaded');
    await popup.close();
    await this.firstSigner
      .getByRole('button', { name: '检查实名结果' })
      .click();
  }

  async verifyPhone() {
    await this.firstSigner.getByRole('button', { name: '验证手机号' }).click();
    await this.page.getByTestId('contract-signer-phone-prompt').waitFor();
    await this.page.getByTestId('contract-signer-phone-answer').fill('7');
    await this.page
      .getByRole('button', { name: '验证并发送短信验证码' })
      .click();
    await this.page.getByTestId('contract-signer-phone-code').fill('123456');
    await this.page.getByRole('button', { name: '确认手机号' }).click();
    await this.page
      .locator('.ant-modal-wrap')
      .last()
      .waitFor({ state: 'hidden' });
  }

  async disableSigner() {
    await this.firstSigner.getByRole('button', { name: '停用' }).click();
    await this.page.getByRole('button', { name: '确认停用' }).click();
  }
}
