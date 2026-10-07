import type { Page } from '@playwright/test';

export class OperationsWorkbenchPage {
  constructor(readonly page: Page) {}

  readonly root = this.page.locator('[data-testid="operations-workbench"]');
  readonly refresh = this.page.getByRole('button', { name: /刷新数据|Refresh data/u });

  async goto() {
    await this.page.goto('/idc/operations/workbench');
    await this.page.waitForLoadState('networkidle');
  }

  async screenshot(name: string) {
    await this.page.screenshot({ path: `temp/${new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)}-${name}.png`, fullPage: false });
  }
}
