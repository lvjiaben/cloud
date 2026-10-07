import type { Page } from '@host-tests/support/playwright';
import { CustomerManagementPage } from '../../../../idc-core/hack/tests/pages/CustomerManagementPage';

/** HTTP rejections can be plain text; successful envelopes must remain strict JSON. */
function decodeBridgeProbe(result: { status: number; text: string }): { code: number | null; data?: any; errorCode?: string } {
  const rejected = result.status < 200 || result.status >= 300;
  let body: unknown;
  try { body = JSON.parse(result.text); }
  catch (error) {
    if (!rejected) throw error;
    // null explicitly means no business code was present. The real HTTP status
    // remains separate and must never be replaced with a fabricated success code.
    return { code: null };
  }
  if (!body || typeof body !== 'object' || !('code' in body) || typeof body.code !== 'number') {
    throw new Error(`Invalid bridge probe envelope (HTTP ${result.status})`);
  }
  if (rejected && body.code === 0) throw new Error(`Rejected bridge probe returned a success code (HTTP ${result.status})`);
  return body as { code: number; data?: any; errorCode?: string };
}

/** Real customer entry and bridge probes; never intercepts assets or business requests. */
export class CloudServerPage {
  private access = '';
  private grants = new Map<string, string>();
  private management: CustomerManagementPage;
  constructor(readonly admin: Page) { this.management = new CustomerManagementPage(admin); }

  async openCustomer(customerCode: string) {
    await this.management.goto();
    await this.management.pageRoot.getByTestId('customer-search-code').fill(customerCode);
    const pending = this.admin.waitForResponse(response => {
      const url = new URL(response.url());
      return url.pathname.endsWith('/admin/customers') && url.searchParams.get('customerCode') === customerCode;
    });
    await this.management.pageRoot.getByRole('button', { name: /搜\s*索/u }).click();
    await pending;
    await this.management.openOverviewByPublicId(customerCode);
    const customer = await this.management.startAssistance('云服务器只读回归，检查页面票据与客户归属，不执行服务器操作');
    await customer.getByTestId('customer-assistance-banner').waitFor();
    await customer.getByTestId('customer-overview').waitFor();
    this.access = await customer.evaluate(() => {
      const key = Object.keys(localStorage).find(value => value.endsWith('core-access'));
      return key ? JSON.parse(localStorage.getItem(key) || '{}').accessToken : '';
    });
    if (!this.access) throw new Error('Assisted customer authentication is unavailable');
    return customer;
  }

  async openServers(customer: Page) {
    await customer.locator('li.vben-sub-menu').filter({ hasText: '产品与服务' }).getByText('云服务器', { exact: true }).click();
    await customer.waitForURL(/\/account\/plugins\/idc-zjmfcloud\/zjmfcloud-servers/u);
    const frame = customer.frameLocator('iframe[data-testid="plugin-sandbox-frame"]');
    await frame.getByTestId('server-skeleton').waitFor({ state: 'hidden' });
    await customer.getByTestId('plugin-sandbox-skeleton').waitFor({ state: 'detached' });
    return frame;
  }

  async grant(customer: Page, path: string, mode: 'anonymous' | 'current' | 'invalid' = 'current') {
    const result = await customer.evaluate(async ({ access, path, mode }) => {
      const headers: Record<string, string> = { 'Content-Type': 'application/json' };
      if (mode !== 'anonymous') headers.Authorization = `Bearer ${mode === 'current' ? access : 'invalid-fixture'}`;
      const response = await fetch('/api/v1/plugins/page-bridge/grant', { method: 'POST', headers, body: JSON.stringify({ pluginId: 'idc-zjmfcloud', host: 'customer', surfaceKey: 'servers', surfaceVersion: 'v1', capability: 'customer-servers.read', method: 'GET', path }) });
      return { status: response.status, text: await response.text() };
    }, { access: this.access, path, mode });
    const body = decodeBridgeProbe(result);
    if (mode !== 'current' && result.status !== 401 && result.status !== 403) throw new Error(`Unauthenticated grant must be rejected by HTTP status, received ${result.status}`);
    if (result.status === 200 && body.code === 0 && body.data?.grant) this.grants.set(path, body.data.grant);
    return { status: result.status, code: body.code, owner: body.data?.ownerPluginId ?? '' };
  }

  async read(customer: Page, path: string, grantPath?: string, mode: 'current' | 'missing' | 'tampered' = 'current') {
    const grant = mode === 'missing' ? '' : this.grants.get(grantPath ?? path) ?? '';
    const result = await customer.evaluate(async ({ access, path, grant, mode }) => {
      const headers: Record<string, string> = { Authorization: `Bearer ${access}` };
      if (grant) headers['X-Lina-Plugin-Page-Grant'] = mode === 'tampered' ? `${grant}tampered` : grant;
      const response = await fetch(`/x/idc-resource/api/v1/${path}`, { headers });
      return { status: response.status, text: await response.text() };
    }, { access: this.access, path, grant, mode });
    const body = decodeBridgeProbe(result);
    return { status: result.status, code: body.code, data: body.data, errorCode: body.errorCode };
  }

  async endAssistance() {
    const end = this.management.overviewDrawer.getByRole('button', { name: '立即结束协助' });
    if (await end.count()) { await end.click(); await end.waitFor({ state: 'detached' }); }
  }
}
