import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import type { Locator, Page } from '@host-tests/support/playwright';
import { expect } from '@host-tests/support/playwright';
import { workspacePath } from '@host-tests/fixtures/config';
import { waitForRouteReady } from '@host-tests/support/ui';

export class AdminExperiencePage {
  readonly root: Locator;
  constructor(readonly page: Page) {
    this.root = page.locator('.plugin-admin-page').first();
  }
  async goto(path: string) {
    await this.page.goto(workspacePath(path), { waitUntil: 'domcontentloaded' });
    await this.root.waitFor({ state: 'visible' });
    await waitForRouteReady(this.page, 30_000);
    await this.page.getByText(/加载菜单中|Loading menu/iu).waitFor({ state: 'hidden' });
  }
  async fitsViewport() {
    await expect.poll(() => this.page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);
    const controls = this.root.locator('.filter-panel input,.filter-panel select,.filter-panel button');
    for (const control of await controls.all()) {
      if (!await control.isVisible()) continue;
      const box = await control.boundingBox();
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(this.page.viewportSize()!.width + 1);
    }
  }
  async expectVisibleGridsFit(requirePager = true) {
    await expect.poll(() => this.root.evaluate((root, requirePager) => {
      const bounds = root.getBoundingClientRect();
      const grids = [...root.querySelectorAll('.vxe-grid')].filter(grid => grid.getBoundingClientRect().width > 0);
      return grids.length > 0 && grids.every(grid => {
        const rect = grid.getBoundingClientRect();
        const parent = grid.parentElement!.getBoundingClientRect();
        const pager = grid.querySelector('.vxe-pager')?.getBoundingClientRect();
        const fitsHeight = requirePager
          ? Boolean(pager && pager.bottom <= innerHeight && pager.bottom <= parent.bottom + 2)
          : rect.bottom <= innerHeight && rect.bottom <= parent.bottom + 2;
        return rect.width >= parent.width - 2 && rect.right <= bounds.right + 1 && fitsHeight;
      });
    }, requirePager)).toBe(true);
  }
  async screenshot(label: string) {
    const now = new Date().toISOString();
    const directory = resolve(process.cwd(), '../../temp', now.slice(0, 10).replaceAll('-', ''));
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({ animations: 'disabled', path: resolve(directory, `${now.slice(11, 19).replaceAll(':', '')}-admin-experience-${label}.png`) });
  }
}
