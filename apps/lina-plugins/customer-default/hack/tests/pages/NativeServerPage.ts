import {fileURLToPath} from 'node:url';
import type { Page } from '@host-tests/support/playwright';
import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';

export class NativeServerPage {
  constructor(readonly page: Page, readonly origin: string, readonly serverNo: string) {}
  get credentialInput() { return this.page.locator('input[aria-label="初始密码"]'); }
  serviceManagement(entitlementNo: string) { return this.page.getByTestId(`manage-service-${entitlementNo}`); }
  openServiceDirectory(directoryNo = '') { return this.page.goto(`${this.origin}/account/product-directories${directoryNo ? '/' + directoryNo : ''}`); }
  openOverview() { return this.page.goto(`${this.origin}/account`); }
  open() { return this.page.goto(`${this.origin}/account/native-servers/${this.serverNo}`); }
  backToServers() { return this.page.getByRole('button', { name: '返回服务器列表', exact: true }).click(); }
  refresh() { return this.page.getByRole('button', { name: '刷新状态', exact: true }).click(); }
  start() { return this.page.getByRole('button', { name: '开机', exact: true }).click(); }
  reboot() { return this.page.getByRole('button', { name: '重启', exact: true }).click(); }
  confirm() { return this.page.getByRole('button', { name: '确认执行', exact: true }).click(); }
  acceptImpact() { return this.page.getByRole('checkbox', { name: '我已了解影响，确认执行', exact: true }).check(); }
  async screenshot(step: string) {
    const now = new Date(); const directory = resolve(fileURLToPath(new URL('../../../../../../temp/',import.meta.url)), now.toISOString().slice(0, 10).replaceAll('-', ''));
    await mkdir(directory, { recursive: true });
    return this.page.screenshot({ path: resolve(directory, now.toISOString().slice(11, 19).replaceAll(':', '') + '-native-server-' + step + '.png'), fullPage: true, animations: 'disabled' });
  }
}
