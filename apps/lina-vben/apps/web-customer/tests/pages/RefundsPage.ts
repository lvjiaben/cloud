import type { Locator, Page } from '@playwright/test';

import { captureCustomerScreenshot } from '../support/screenshot';

export class RefundsPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId('customer-refund-page');
  }

  async login() {
    await this.page.goto('/auth/login');
    const inputs = this.page.locator('input');
    await inputs.nth(0).fill('customer');
    await inputs.nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login' }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async open(orderNo = '') {
    const suffix = orderNo ? `?orderNo=${encodeURIComponent(orderNo)}` : '';
    await this.page.goto(`/account/refunds${suffix}`);
    await this.root.waitFor({ state: 'visible' });
  }

  async selectWholeItem(invoiceItemNo: string, amountMinor: string) {
    await this.page.getByTestId(`refund-whole-${invoiceItemNo}`).click();
    await this.page
      .getByTestId(`refund-amount-${invoiceItemNo}`)
      .fill(amountMinor);
  }

  async selectEntitlement(
    invoiceItemNo: string,
    entitlementNo: string,
    amountMinor: string,
  ) {
    await this.page.getByTestId(`refund-entitlement-${entitlementNo}`).click();
    await this.page
      .getByTestId(`refund-amount-${invoiceItemNo}`)
      .fill(amountMinor);
  }

  async setQuantity(invoiceItemNo: string, quantity: number) {
    await this.page
      .getByTestId(`refund-quantity-${invoiceItemNo}`)
      .fill(String(quantity));
  }

  async openPreview() {
    await this.page.getByTestId('refund-preview').click();
    await this.page
      .getByRole('dialog')
      .filter({ hasText: /确认退款金额和服务影响|Confirm refund amount/u })
      .waitFor({ state: 'visible' });
  }

  async chooseReason(label: string) {
    await this.page.getByTestId('refund-reason').click();
    await this.page
      .locator('.ant-select-item-option')
      .filter({ hasText: label })
      .click();
  }

  async confirmImpact() {
    await this.page.getByTestId('refund-impact-confirm').click();
  }

  async submit() {
    await this.page.getByTestId('refund-submit').click();
  }

  refund(refundNo: string) {
    return this.page.getByTestId(`customer-refund-${refundNo}`);
  }

  async screenshot(description: string) {
    return captureCustomerScreenshot(this.page, description);
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
  }
}
