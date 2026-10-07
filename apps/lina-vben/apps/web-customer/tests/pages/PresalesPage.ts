import type { Locator, Page } from '@playwright/test';

type InputFiles = Parameters<Locator['setInputFiles']>[0];

export class PresalesPage {
  constructor(private readonly page: Page) {}

  get accessUnavailable() {
    return this.page.getByTestId('presales-access-unavailable');
  }

  get accessiblePrompt() {
    return this.page.getByTestId('presales-accessible-prompt');
  }

  get challengeImage() {
    return this.page.getByTestId('presales-challenge-image');
  }

  get challengeUnavailable() {
    return this.page.getByTestId('presales-challenge-unavailable');
  }

  get codePanel() {
    return this.page.getByTestId('presales-code-panel');
  }

  get detail() {
    return this.page.getByTestId('presales-detail');
  }

  get linkDeliveryUncertain() {
    return this.page.getByTestId('presales-link-delivery-uncertain');
  }

  get renewal() {
    return this.page.getByTestId('presales-renewal');
  }

  get renewalNeedsSafeTicket() {
    return this.page.getByTestId('presales-renewal-needs-safe-ticket');
  }

  get retryStatus() {
    return this.page.getByTestId('presales-retry-status');
  }

  get replyAttachments() {
    return this.page.getByTestId('presales-reply-attachments');
  }

  get replyInput() {
    return this.page.getByTestId('presales-reply');
  }

  get submissionUncertain() {
    return this.page.getByTestId('presales-submit-uncertain');
  }

  async answerChallenge(answer = '5') {
    await this.page.getByTestId('presales-challenge-answer').fill(answer);
    await this.page.getByTestId('presales-send-code').click();
  }

  async fillInquiry(
    destination: string,
    subject = '裸金属服务器交付咨询',
    body = '请说明交付时间和可选配置。',
  ) {
    await this.page.getByTestId('presales-destination').fill(destination);
    await this.page.getByTestId('presales-category').click();
    await this.page
      .locator('.ant-select-item-option')
      .filter({ hasText: '购买咨询' })
      .click();
    await this.page.getByTestId('presales-subject').fill(subject);
    await this.page.getByTestId('presales-body').fill(body);
  }

  async fillSevenCustomFieldTypes() {
    const prefix = 'presales-custom-field';
    await this.page
      .getByTestId(`${prefix}-CF-TEXT`)
      .locator('input')
      .fill('新建项目');
    await this.page
      .getByTestId(`${prefix}-CF-MULTI-LINE`)
      .locator('textarea')
      .fill('需要确认交付区域和存储配置。');
    await this.page
      .getByTestId(`${prefix}-CF-NUMBER`)
      .locator('input')
      .fill('3');
    await this.chooseCustomFieldOption(`${prefix}-CF-SINGLE`, '华东');
    await this.chooseCustomFieldOption(`${prefix}-CF-MULTI-SELECT`, '网络');
    await this.chooseCustomFieldOption(`${prefix}-CF-MULTI-SELECT`, '存储');
    await this.page
      .getByTestId(`${prefix}-CF-DATE`)
      .locator('input')
      .fill('2026-08-15');
    await this.page
      .getByTestId(`${prefix}-CF-BOOLEAN`)
      .getByText('是', { exact: true })
      .click();
  }

  async openAccess(token: string, ticketNo?: string) {
    const query = ticketNo ? `?ticket=${encodeURIComponent(ticketNo)}` : '';
    await this.page.goto(`/auth/presales/access/${token}${query}`);
  }

  async openCreate() {
    await this.page.goto('/auth/presales');
  }

  async reply(message: string) {
    await this.replyInput.fill(message);
    await this.submitReply();
  }

  async selectReplyAttachment(file: InputFiles) {
    await this.replyAttachments
      .locator('input[type="file"]')
      .setInputFiles(file);
  }

  async submitReply() {
    await this.page.getByTestId('presales-reply-submit').click();
  }

  async requestChallenge() {
    await this.page.getByTestId('presales-start-verification').click();
  }

  async selectAccessibleChallenge() {
    await this.page
      .getByTestId('presales-challenge')
      .getByText('无障碍算术题', { exact: true })
      .click();
  }

  async submitCode(code = '123456') {
    await this.page.getByTestId('presales-code').fill(code);
    await this.page.getByTestId('presales-submit').click();
  }

  async waitForIdle() {
    await this.page
      .locator('.ant-spin-spinning')
      .waitFor({ state: 'detached' });
    await this.page
      .locator('.ant-spin-container.ant-spin-blur')
      .waitFor({ state: 'detached' });
  }

  async downloadAttachment(name: string) {
    const download = this.page.waitForEvent('download');
    await this.page.getByRole('button', { name: `下载附件 ${name}` }).click();
    return download;
  }

  async close() {
    await this.page.getByTestId('presales-close').click();
    await this.page
      .locator('.ant-modal-confirm')
      .getByRole('button', { name: /问题已解决|Problem solved/u })
      .click();
  }

  async rate(stars: number, feedback = '') {
    await this.page.getByTestId('presales-rate').click();
    await this.page
      .getByTestId('presales-rating-stars')
      .locator('.ant-rate-star')
      .nth(stars - 1)
      .click();
    if (feedback) {
      await this.page.getByTestId('presales-rating-feedback').fill(feedback);
    }
    await this.page.getByTestId('presales-rating-submit').click();
  }

  async reopen() {
    await this.page.getByTestId('presales-reopen').click();
  }

  async renewWithAccessibleChallenge(destination: string) {
    await this.page
      .getByTestId('presales-renewal-destination')
      .fill(destination);
    await this.page.getByTestId('presales-renewal-start').click();
    await this.page
      .getByTestId('presales-renewal-challenge')
      .getByText('无障碍算术题', { exact: true })
      .click();
    await this.page.getByTestId('presales-renewal-challenge-answer').fill('5');
    await this.page.getByTestId('presales-renewal-send-code').click();
    await this.page.getByTestId('presales-renewal-code').fill('654321');
    await this.page.getByTestId('presales-renewal-submit').click();
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
