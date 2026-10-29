import { expect } from '@host-tests/support/playwright';
import { CatalogEditorTabsPage } from './CatalogEditorTabsPage';

export class CatalogBulkSafetyPage extends CatalogEditorTabsPage {
  get batchDialog() { return this.page.getByRole('dialog').filter({has:this.page.getByTestId('catalog-option-batch-input')}); }
  get saveReminder() { return this.page.getByTestId('catalog-save-unsaved-hint'); }
  get leaveDialog() { return this.page.getByRole('dialog').filter({hasText:'有修改尚未保存'}); }
  async openBatch() { await this.parameterModal.getByTestId('catalog-option-batch-open').click(); await expect(this.batchDialog).toBeVisible(); }
  async pasteItems(text:string) { await this.batchDialog.getByTestId('catalog-option-batch-input').fill(text); }
  async applyBatch() { await this.batchDialog.getByRole('button',{name:'添加到子项',exact:true}).click(); await expect(this.batchDialog).toBeHidden(); }
  async cancelBatch() { await this.batchDialog.getByRole('button',{name:/^取\s*消$/}).click(); await expect(this.batchDialog).toBeHidden(); }
  async priceFirstItem() {
    const prices=this.parameterModal.getByTestId('catalog-offer-field-prices');
    await prices.getByTestId('catalog-offer-price-amount').first().fill('5.00');
    await prices.getByTestId('catalog-offer-price-renewal').first().fill('6.00');
    await prices.getByTestId('catalog-offer-price-setup').first().fill('0.00');
  }
  async copyItem() { await this.parameterModal.getByTestId('catalog-option-child-copy').click(); }
  async selectItem(index:number) { await this.parameterModal.getByTestId(`catalog-option-child-${index}`).click(); }
  async changeName(name:string) { await this.descriptionName.fill(name); }
  async beforeUnloadBlocked() {
    return this.page.evaluate(() => {
      const event = new Event('beforeunload',{cancelable:true});
      window.dispatchEvent(event);
      return event.defaultPrevented;
    });
  }
  async cancelRefresh() {
    const pendingDialog = this.page.waitForEvent('dialog');
    // Chromium can leave the cancelled reload pending until the navigation timeout.
    const pendingReload = this.page.reload({timeout:2000}).catch(error => {
      if (error.name !== 'TimeoutError' && !String(error).includes('ERR_ABORTED')) throw error;
    });
    const dialog = await pendingDialog;
    const type = dialog.type();
    await dialog.dismiss();
    await pendingReload;
    return type;
  }
  async cancelBrowserClose() {
    const pendingDialog = this.page.waitForEvent('dialog');
    await this.page.close({runBeforeUnload:true});
    const dialog = await pendingDialog;
    const type = dialog.type();
    await dialog.dismiss();
    return type;
  }
  async prepareBackNavigation() {
    await this.closeEditor();await expect(this.detailDrawer).toBeHidden();
    // Build router-owned history through menu clicks, as in the real workspace.
    // Direct page.goto hash entries do not have Vue Router's history position.
    await this.page.getByRole('menuitem',{name:'运营数据',exact:true}).click();
    await expect(this.pageRoot).toBeHidden();
    const catalog = this.page.getByRole('menuitem',{name:'商品管理',exact:true});
    if (!(await catalog.isVisible())) await this.page.getByText('商品与定价',{exact:true}).click();
    await catalog.click();await expect(this.pageRoot).toBeVisible();
    await this.openProductDetail('product-manual-service');
  }
  async leaveByBack() { await this.page.goBack(); }
  async closeByEscape() { await this.descriptionName.press('Escape'); }
  async closeEditor() { await this.page.getByTestId('catalog-editor-back').click(); }
  async continueEditing() { await this.leaveDialog.getByRole('button',{name:'继续编辑',exact:true}).click(); await expect(this.leaveDialog).toBeHidden(); }
  async discard() { await this.leaveDialog.getByRole('button',{name:'丢弃并关闭',exact:true}).click(); await expect(this.detailDrawer).toBeHidden(); }
}
