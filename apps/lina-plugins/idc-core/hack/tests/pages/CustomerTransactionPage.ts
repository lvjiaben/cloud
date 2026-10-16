// Customer transaction navigation preserves copying and opens owner-managed details.
import { workspacePath } from '@host-tests/fixtures/config';
import { CustomerWorkspacePage } from './CustomerWorkspacePage';
export class CustomerTransactionPage extends CustomerWorkspacePage {
  async showRecords(kind: 'invoice' | 'order') {
    await this.overviewDrawer.getByRole('tab', { name: kind === 'order' ? /^全部订单/u : /^全部账单/u }).click();
  }
  detailButton(kind: 'invoice' | 'order', reference: string) {
    return this.page.getByTestId(`customer-${kind}-detail-${reference}`);
  }
  async openRecord(kind: 'invoice' | 'order', reference: string) {
    await this.detailButton(kind, reference).click();
    await this.page.getByTestId(kind === 'order' ? 'commerce-order-page' : 'financial-management-page').waitFor({state:'visible'});
  }
  async openInvoiceFromList(reference: string) {
    await this.page.getByTestId(`finance-invoice-detail-${reference}`).filter({visible:true}).click();
  }
  async retryRecord() {
    await this.recordDialog.getByRole('button',{name:/^重\s*试$/u}).click();
  }
  async closeRecord() {
    await this.page.keyboard.press('Escape');
    await this.recordDialog.waitFor({state:'hidden'});
  }
  get recordDialog() { return this.page.getByRole('dialog').last(); }
  async gotoRecord(kind: 'invoice' | 'order', reference: string) {
    await this.page.goto(workspacePath(`/idc/${kind === 'order' ? 'orders' : 'finance'}?${kind === 'order' ? 'orderNo' : 'invoiceNo'}=${encodeURIComponent(reference)}`));
    await this.page.getByTestId(kind === 'order' ? 'commerce-order-page' : 'financial-management-page').waitFor({state:'visible'});
  }
}
