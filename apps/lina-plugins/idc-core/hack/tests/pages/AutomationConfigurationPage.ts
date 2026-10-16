// Own the configuration-center entry and its provider creation dialog.
import type { Page } from '@host-tests/support/playwright';
import { workspacePath } from '@host-tests/fixtures/config';
import { waitForRouteReady } from '@host-tests/support/ui';
export class AutomationConfigurationPage {
 constructor(readonly page: Page) {}
 get center() { return this.page.getByTestId('system-configuration-center'); }
 get dialog() { return this.page.getByRole('dialog',{name:'新增接口',exact:true}); }
 get deletedChannelsToggle() { return this.page.getByTestId('provider-show-deleted'); }
 async goto() { await this.page.goto(workspacePath('/system/configuration-center')); await this.center.waitFor(); }
 async openAutomation() {
  const loaded=this.page.waitForResponse(r=>r.url().includes('/admin/resource/provider-configurations?') && r.request().method()==='GET');
  await this.center.getByRole('tab',{name:'自动化接口',exact:true}).click();
  await this.center.locator('article').filter({has:this.page.getByText('自动化接口',{exact:true})}).getByRole('button',{name:'进入配置',exact:true}).click();
  await loaded;
  await waitForRouteReady(this.page,20000);
  await this.page.getByTestId('provider-configuration-page').waitFor();
  await this.page.getByRole('dialog').filter({has:this.page.getByTestId('provider-configuration-page')}).evaluate(async element=>{await Promise.all(element.getAnimations({subtree:true}).map(animation=>animation.finished.catch(()=>undefined)));});
 }
 async toggleDeletedChannels() {
  const response=this.page.waitForResponse(r=>r.url().includes('/admin/resource/provider-configurations?') && r.request().method()==='GET');
  await this.page.getByTestId('provider-show-deleted').click();
  return (await response).json();
 }
 async waitForChannelCount(total: number) {
  await this.page.getByTestId('provider-configuration-page').locator('.vxe-pager--total').filter({hasText:String(total)}).waitFor();
 }
 async openServerCreation() {
  await this.page.getByRole('tab',{name:'服务器接口',exact:true}).click();
  await this.page.getByTestId('provider-configuration-page').getByRole('button',{name:'新增接口',exact:true}).click();
  await this.dialog.evaluate(async element => {
   await Promise.all(element.getAnimations().map(animation => animation.finished.catch(() => undefined)));
  });
 }
 async closeCreation() { await this.dialog.getByRole('button',{name:/取\s*消/u}).click(); }
 async openProductInterfaces() {
  const loaded=this.page.waitForResponse(response=>response.url().includes('/admin/catalog/automation/interfaces?') && response.request().method()==='GET');
  await this.page.getByRole('tab',{name:'其他产品接口',exact:true}).click();
  return (await loaded).json();
 }
}
