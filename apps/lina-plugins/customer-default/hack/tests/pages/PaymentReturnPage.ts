import {fileURLToPath} from 'node:url';
import type { Page } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';

// The payment return page consumes only customer-owned server payment facts.
export class PaymentReturnPage {
  constructor(readonly page: Page, readonly origin: string) {}
  open() { return this.page.goto(this.origin + '/payment/return/PAT_return_test'); }
  refresh() { return this.page.getByRole('button', {name: '刷新状态', exact: true}).click(); }
  viewDelivery() { return this.page.getByRole('button', {name: '查看开通进度', exact: true}).click(); }
  async screenshot(step: string) {
    const now = new Date();
    const directory = resolve(fileURLToPath(new URL("../../../../../../", import.meta.url)), "temp", now.toISOString().slice(0,10).replaceAll("-", ""));
    await mkdir(directory, {recursive: true});
    return this.page.screenshot({path:resolve(directory, now.toISOString().slice(11,19).replaceAll(':','') + '-payment-return-' + step + '.png'), fullPage: true, animations: "disabled"});
  }
}
