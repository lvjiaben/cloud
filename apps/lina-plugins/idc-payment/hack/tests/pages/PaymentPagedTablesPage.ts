import type { Locator, Page } from '@playwright/test';
import { workspacePath } from '../../../../../../hack/tests/fixtures/config';
import { capturePaymentScreenshot } from '../support/screenshot';

export type PaymentDirectory = 'channels' | 'health' | 'batches' | 'differences' | 'funds';

/** Owns official pager interactions on formal and isolated payment grids. */
export class PaymentPagedTablesPage {
  constructor(readonly page: Page, readonly standalone?: Locator) {}
  table(kind: PaymentDirectory) {
    if (this.standalone) {
      const index = kind === 'health' || kind === 'funds' ? 0 : kind === 'batches' ? 1 : 2;
      return this.standalone.locator('.vxe-grid').nth(index);
    }
    return this.page.getByTestId(`payment-${kind}-table`);
  }
  rows(kind: PaymentDirectory) { return this.table(kind).locator('.vxe-table--main-wrapper .vxe-body--row'); }
  headers(kind: PaymentDirectory) { return this.table(kind).locator('.vxe-table--main-wrapper .vxe-header--row'); }
  pager(kind: PaymentDirectory) { return this.table(kind).locator('.vxe-pager'); }
  total(kind: PaymentDirectory) { return this.pager(kind).locator('.vxe-pager--total'); }
  active(kind: PaymentDirectory) { return this.pager(kind).locator('.vxe-pager--num-btn.is--active'); }
  async open(kind: PaymentDirectory) {
    const route = kind === 'channels' ? '/idc/payment-channels' : kind === 'funds' ? '/idc/payment-exception-funds' : '/idc/payment-reconciliation';
    await this.page.goto(workspacePath(route), { waitUntil: 'domcontentloaded' });
    await this.table(kind).waitFor({ state: 'visible' });
  }
  async last(kind: PaymentDirectory) { await this.pager(kind).locator('.vxe-pager--end-btn').click(); }
  async first(kind: PaymentDirectory) { await this.pager(kind).locator('.vxe-pager--home-btn').click(); }
  async next(kind: PaymentDirectory) { await this.pager(kind).locator('.vxe-pager--next-btn').click(); }
  async previous(kind: PaymentDirectory) { await this.pager(kind).locator('.vxe-pager--prev-btn').click(); }
  async size(kind: PaymentDirectory, size: number) {
    await this.pager(kind).locator('.vxe-select').click();
    await this.page.locator('.vxe-select-option').filter({ hasText: new RegExp(`^${size}\\s*(条/页|/page)$`, 'u') }).click();
  }
  async refresh(kind: PaymentDirectory) {
    await this.table(kind).locator('.vxe-toolbar').getByRole('button', { name: /^(刷\s*新|Refresh)$/u }).click();
  }
  async selectChannel(label: string) {
    await this.table('batches').locator('.ant-select').first().click();
    const popup = this.page.locator('.ant-select-dropdown:visible');
    await popup.getByText(label, { exact: true }).click();
  }
  async surfaces(kind: PaymentDirectory) {
    await this.page.mouse.move(0, 0);
    return this.table(kind).evaluate(root => {
      function visibleColor(element: Element): string {
        let current: Element | null = element;
        while (current) {
          const value = getComputedStyle(current).backgroundColor;
          if (value !== 'transparent' && value !== 'rgba(0, 0, 0, 0)') return value;
          current = current.parentElement;
        }
        return 'transparent';
      }
      function themeColor(token: string): string {
        const probe = document.createElement('span');
        probe.style.backgroundColor = `hsl(${getComputedStyle(document.documentElement).getPropertyValue(token)})`;
        document.body.append(probe);
        const value = getComputedStyle(probe).backgroundColor;
        probe.remove();
        return value;
      }
      const visible = (selector: string) => [...root.querySelectorAll(selector)]
        .filter(element => element.getBoundingClientRect().height > 0).map(visibleColor);
      return {
        header: themeColor('--muted'),
        panel: themeColor('--card'),
        headers: visible('.vxe-table--main-wrapper .vxe-header--column'),
        panels: visible('.vxe-table--main-wrapper .vxe-body--column,.vxe-toolbar,.vxe-pager'),
      };
    });
  }
  async screenshot(kind: PaymentDirectory, name: string) {
    await this.pager(kind).scrollIntoViewIfNeeded();
    return capturePaymentScreenshot(this.page, name);
  }
}
