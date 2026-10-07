import type { Page } from '@playwright/test';
import { workspacePath } from '@host-tests/fixtures/config';

/** Real host/customer navigation; deliberately never registers network routes. */
export class SSLLiveDiscoveryPage {
  // Memory-only test evidence, retained so post-revocation checks reuse the
  // exact session rather than accidentally testing an already-cleared store.
  private customerAccessToken = '';
  constructor(readonly admin: Page) {}

  async openAssistedCustomer(customerCode: string) {
    await this.admin.goto(workspacePath('/idc/customers'));
    const root = this.admin.getByTestId('customer-management-page');
    await root.getByTestId('customer-search-code').fill(customerCode);
    const searched=this.admin.waitForResponse(response=>{
      const url=new URL(response.url());
      return url.pathname.endsWith('/admin/customers') && url.searchParams.get('customerCode')===customerCode;
    });
    await root.getByRole('button',{name:/搜\s*索/u}).click();
    await searched;
    const rows=root.locator('.vxe-table--main-wrapper .vxe-table--body-wrapper tr.vxe-body--row');
    await rows.filter({hasText:customerCode}).first().waitFor();
    const rowIndex=await rows.evaluateAll((elements,code)=>elements.findIndex(element=>Array.from(element.querySelectorAll('.vxe-cell')).some(cell=>cell.textContent?.trim()===code)),customerCode);
    if(rowIndex<0) throw new Error('Requested fixture customer is not visible');
    await root.locator('[data-testid^="customer-overview-"]:visible').nth(rowIndex).click();
    const drawer=this.admin.getByTestId('customer-overview-drawer');
    await drawer.getByTestId('customer-assistance-open').click();
    const dialog=this.admin.getByRole('dialog').last();
    await dialog.getByTestId('customer-assistance-reason').fill('SSL 真实页面只读回归，不执行证书或供应商操作');
    await dialog.getByTestId('customer-assistance-confirm').check();
    const pending=this.admin.context().waitForEvent('page');
    await dialog.getByRole('button',{name:/确\s*认/u}).click();
    const customer=await pending;
    await customer.getByTestId('customer-assistance-banner').waitFor();
    await customer.getByTestId('customer-overview').waitFor();
    return customer;
  }

  async openCertificates(customer:Page) {
    await customer.locator('li.vben-sub-menu').filter({hasText:'产品与服务'}).getByText('SSL 证书',{exact:true}).click();
    await customer.waitForURL(/\/account\/plugins\/yunsu-ssl\/ssl-certificates/u);
    const frame=customer.frameLocator('iframe[data-testid="plugin-sandbox-frame"]');
    await frame.locator('.vxe-grid').waitFor({state:'attached'});
    await customer.getByTestId('plugin-sandbox-skeleton').waitFor({state:'detached'});
    await frame.getByTestId('ssl-loading').waitFor({state:'hidden'});
    return frame;
  }

  async discovery(customer:Page, mode:'anonymous'|'current'|'invalid') {
    if (mode==='current' && !this.customerAccessToken) {
      this.customerAccessToken=await customer.evaluate(()=>{
        const key=Object.keys(localStorage).find(key=>key.endsWith('core-access'));
        return key ? JSON.parse(localStorage.getItem(key) || '{}').accessToken : '';
      });
      if (!this.customerAccessToken) throw new Error('Assisted customer session is missing');
    }
    return customer.evaluate(async ({mode,access})=>{
      const headers:Record<string,string>={};
      if(mode!=='anonymous') headers.Authorization=`Bearer ${mode==='current' ? access : 'deliberately-invalid-fixture'}`;
      const response=await fetch('/api/v1/plugins/dynamic',{headers});
      if(response.status!==200) return {status:response.status,surfaces:[]};
      const body=await response.json();
      return {status:response.status,surfaces:(body.data?.list ?? []).flatMap((item:{customerSurfaces?:Array<{pluginId:string;entryUrl:string}>})=>item.customerSurfaces ?? [])};
    },{mode,access:mode==='current' ? this.customerAccessToken : ''});
  }

  async endAssistance() {
    const end=this.admin.getByTestId('customer-overview-drawer').getByRole('button',{name:'立即结束协助'});
    if(await end.count()) {
      await end.click();
      await end.waitFor({state:'detached'});
    }
  }
}
