import type { Page } from '@playwright/test';

import { expect } from '../fixtures/customer';
import { captureCustomerScreenshot } from '../support/screenshot';

/** Visual assertions only: no business writes and no shell preference changes. */
export class CustomerInnerPages {
  constructor(readonly page: Page) {}

  async login() {
    await this.page.goto('/auth/login');
    await this.page.locator('input').nth(0).fill('customer');
    await this.page.locator('input').nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login', exact: true }).click();
    await this.page.waitForURL(/\/account$/u);
    await expect(this.page.getByTestId('customer-overview')).toBeVisible();
  }

  async inspect(path: string, screenshotName: string) {
    await this.page.goto(path);
    const inner = this.page.locator('.customer-inner-page').first();
    await expect(inner).toBeVisible();
    await expect(
      inner.locator('.customer-inner-heading').first(),
    ).toBeVisible();
    await expect(this.page.locator('.ant-spin-spinning')).toHaveCount(0);
    await expect(this.page.locator('.ant-skeleton:visible')).toHaveCount(0);

    // Vben enters pages with a short transform animation. Check settled geometry,
    // not an intermediate frame, without disabling the user's page animation.
    await expect
      .poll(() =>
        inner.evaluate((element) => {
          const bounds = element.getBoundingClientRect();
          const main = document.querySelector('main')!.getBoundingClientRect();
          return bounds.left >= main.left && bounds.right <= main.right + 1;
        }),
      )
      .toBe(true);

    const geometry = await inner.evaluate((element) => {
      const main = document.querySelector('main')!;
      const rect = element.getBoundingClientRect();
      const mainRect = main.getBoundingClientRect();
      const heading = element.querySelector<HTMLElement>(
        '.customer-inner-heading',
      )!;
      const sidebar = document.querySelector('aside')!;
      return {
        headingPadding: getComputedStyle(heading).paddingLeft,
        left: rect.left,
        mainLeft: mainRect.left,
        mainRight: mainRect.right,
        right: rect.right,
        sidebarWidth: sidebar.getBoundingClientRect().width,
        viewport: window.innerWidth,
        width: document.documentElement.scrollWidth,
      };
    });
    expect(geometry.width, `${path}: document overflow`).toBeLessThanOrEqual(
      geometry.viewport,
    );
    expect(geometry.left, `${path}: hidden by sidebar`).toBeGreaterThanOrEqual(
      geometry.mainLeft,
    );
    expect(geometry.right, `${path}: content overflow`).toBeLessThanOrEqual(
      geometry.mainRight + 1,
    );
    expect(geometry.sidebarWidth).toBe(geometry.viewport < 768 ? 0 : 204);
    expect(geometry.headingPadding).toBe(
      geometry.viewport < 768 ? '12px' : '20px',
    );
    await captureCustomerScreenshot(this.page, screenshotName);
  }

  async inspectTable() {
    const grid = this.page.locator('.vxe-grid:visible').first();
    await expect(grid).toBeVisible();
    const pager = grid.locator('.vxe-pager');
    await expect(pager).toBeVisible();
    const geometry = await grid.evaluate((element) => {
      const header = element.querySelector('.vxe-table--header-wrapper')!;
      const pager = element.querySelector('.vxe-pager')!;
      const body = element.querySelector('.vxe-table--body-wrapper')!;
      return {
        bodyHeight: body.getBoundingClientRect().height,
        headerBackground: getComputedStyle(header).backgroundColor,
        pagerBackground: getComputedStyle(pager).backgroundColor,
        pagerBottom: pager.getBoundingClientRect().bottom,
        viewportHeight: window.innerHeight,
      };
    });
    expect(geometry.bodyHeight).toBeGreaterThan(80);
    expect(geometry.headerBackground).not.toBe('rgb(255, 255, 255)');
    expect(geometry.pagerBackground).toBe('rgb(255, 255, 255)');
    expect(geometry.pagerBottom).toBeLessThanOrEqual(geometry.viewportHeight);
  }
}
