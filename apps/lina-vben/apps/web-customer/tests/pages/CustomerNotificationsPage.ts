import type { Page } from '@playwright/test';

export class CustomerNotificationsPage {
  get marketingPreference() {
    return this.page.getByTestId('notification-consent-email-contact-1');
  }

  get preferences() {
    return this.page.getByTestId('notification-preferences');
  }

  get quietHours() {
    return this.page.getByTestId('notification-quiet-hours');
  }

  get quietHoursEnd() {
    return this.page.getByTestId('notification-quiet-hours-end');
  }

  get quietHoursEnabled() {
    return this.quietHours.getByRole('switch');
  }

  get quietHoursSave() {
    return this.page.getByTestId('notification-quiet-hours-save');
  }

  get quietHoursStart() {
    return this.page.getByTestId('notification-quiet-hours-start');
  }

  get quietHoursTimezone() {
    return this.page.getByTestId('notification-quiet-hours-timezone');
  }

  get securityPreference() {
    return this.page.getByTestId('notification-preference-security-email');
  }

  get supportPreference() {
    return this.page.getByTestId('notification-preference-support-in_app');
  }

  constructor(private readonly page: Page) {}

  async loadLatestSettings() {
    await this.page.getByTestId('notification-preferences-refresh').click();
    await this.waitForIdle();
  }

  async open() {
    await this.page.goto('/account/notifications');
    await this.waitForIdle();
  }

  async openPreferences() {
    await this.page.getByRole('tab', { name: '接收偏好' }).click();
    await this.waitForIdle();
  }

  async saveAgain() {
    await this.supportPreference
      .getByRole('button', { name: '再次保存' })
      .click();
  }

  async saveQuietHours(start: string, end: string) {
    await this.quietHoursStart.fill(start);
    await this.quietHoursEnd.fill(end);
    await this.quietHoursSave.click();
  }

  async saveNewQuietHours(timezone: string, start: string, end: string) {
    await this.quietHoursTimezone.fill(timezone);
    await this.quietHoursStart.fill(start);
    await this.quietHoursEnd.fill(end);
    await this.quietHoursEnabled.click();
    await this.quietHoursSave.click();
  }

  async toggleMarketing() {
    await this.marketingPreference.getByRole('switch').click();
  }

  async toggleSupport() {
    await this.supportPreference.getByRole('switch').click();
  }

  async waitForIdle() {
    await this.page
      .locator('.ant-spin-spinning')
      .waitFor({ state: 'detached' });
    await this.page
      .locator('.ant-spin-container.ant-spin-blur')
      .waitFor({ state: 'detached' });
  }
}
