import type { Page } from '@playwright/test';

export class CustomerSupportPage {
  constructor(private readonly page: Page) {}

  get customFields() {
    return this.page.getByTestId('ticket-custom-field-group');
  }

  get entitlement() {
    return this.page.getByTestId('ticket-entitlement');
  }

  async chooseEntitlement(name: string) {
    await this.entitlement.click();
    await this.page
      .locator('.ant-select-dropdown:visible .ant-select-item-option')
      .filter({ hasText: new RegExp(`^${name} · `, 'u') })
      .click();
  }

  async fillBaseTicket() {
    await this.page.getByTestId('ticket-category').click();
    await this.page
      .locator('.ant-select-item-option')
      .filter({ hasText: '网络问题' })
      .click();
    await this.page.getByTestId('ticket-subject').fill('服务器网络异常');
    await this.page
      .getByTestId('ticket-body')
      .fill('服务器无法连接，请协助检查。');
  }

  async fillSevenCustomFieldTypes(prefix = 'ticket-custom-field') {
    await this.page
      .getByTestId(`${prefix}-CF-TEXT`)
      .locator('input')
      .fill('生产环境');
    await this.page
      .getByTestId(`${prefix}-CF-MULTI-LINE`)
      .locator('textarea')
      .fill('恢复网络并确认连接稳定。');
    await this.page
      .getByTestId(`${prefix}-CF-NUMBER`)
      .locator('input')
      .fill('2');

    await this.chooseCustomFieldOption(`${prefix}-CF-SINGLE`, '华东');
    await this.chooseCustomFieldOption(
      `${prefix}-CF-MULTI-SELECT`,
      '网络',
    );
    await this.chooseCustomFieldOption(
      `${prefix}-CF-MULTI-SELECT`,
      '存储',
    );

    await this.page
      .getByTestId(`${prefix}-CF-DATE`)
      .locator('input')
      .fill('2026-08-15');
    await this.page
      .getByTestId(`${prefix}-CF-BOOLEAN`)
      .getByText('是', { exact: true })
      .click();
  }

  async openCreate() {
    await this.page.goto('/account/support/new');
    await this.customFields.waitFor();
  }

  async submit() {
    await this.page.getByTestId('ticket-submit').click();
  }

  private async chooseCustomFieldOption(testId: string, name: string) {
    const field = this.page.getByTestId(testId);
    const combobox = field.getByRole('combobox');
    if ((await combobox.getAttribute('aria-expanded')) !== 'true') {
      await field.locator('.ant-select-selector').click();
    }
    await this.page
      .locator('.ant-select-dropdown:visible .ant-select-item-option')
      .filter({ has: this.page.getByText(name, { exact: true }) })
      .click();
  }
}
