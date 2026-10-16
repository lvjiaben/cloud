import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import type { Page } from '@host-tests/support/playwright';
import { ProviderConfigurationPage } from './ProviderConfigurationPage';

/** Explicit peer authorization interactions within the actual provider page. */
export class PeerLifecyclePage extends ProviderConfigurationPage {
  constructor(page: Page) { super(page); }
  get entry() { return this.page.getByTestId('peer-lifecycle-entry'); }
  async close() { await this.page.keyboard.press('Escape'); await this.card.waitFor({ state: 'hidden' }); }
  get pager() { return this.root.locator('.ant-tabs-tabpane-active .vxe-pager'); }
  async cancelConfirm() { await this.dialog.getByRole('button', { name: /取消|Cancel/u }).click(); await this.dialog.waitFor({ state: 'hidden' }); }
  async open() { await this.entry.click(); }
  get card() { return this.page.getByTestId('peer-lifecycle'); }
  get error() { return this.card.getByTestId('peer-lifecycle-error'); }
  get result() { return this.card.getByTestId('peer-lifecycle-result'); }
  get dialog() { return this.page.getByRole('dialog').filter({ has: this.page.getByRole('button', { name: /确认重新执行|Confirm and run again/u }) }); }
  get reconfirmButton() { return this.card.getByRole('button', { name: /重新确认连接器授权|Reconfirm connector authorization/u }); }
  get confirmButton() { return this.dialog.getByRole('button', { name: /确认重新执行|Confirm and run again/u }); }
  async check() { await this.card.getByRole('button', { name: /检查未完成业务|Check unfinished business/u }).click(); }
  async reconfirm() { await this.reconfirmButton.click(); }
  async confirm() { await this.confirmButton.click(); }
  async screenshot(description: string) {
    await this.page.evaluate(async () => {
      await Promise.all(document.getAnimations().filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation => animation.finished.catch(() => undefined)));
    });
    const date = new Date().toISOString().replace(/[-:.TZ]/gu, '');
    const directory = resolve(process.cwd(), '../../temp', date.slice(0, 8));
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${date.slice(8)}-${description}.png`);
    await this.page.screenshot({ fullPage: false, path }); return path;
  }
}
