import { test, expect } from '../support/catalog-auth';
import { CatalogWorkspacePage } from '../pages/CatalogWorkspacePage';
import { installCatalogManagementMock, products } from '../support/catalog-mock';
import { config } from '@host-tests/fixtures/config';

test.describe('TC-42 商品复制草稿', () => {
  test('TC-42a: 复制失败保留输入，重试成功后显示新草稿', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    const rows = products.map(row => ({ ...row }));
    const requests: Record<string, unknown>[] = [];
    await adminPage.route('**/admin/catalog/products?**', route => route.fulfill({ json: { code: 0, data: { products: rows, total: rows.length, nextCursor: '' }, message: 'success' } }));
    await adminPage.route('**/admin/catalog/products/*/copies', async route => {
      const body = route.request().postDataJSON(); requests.push(body);
      if (requests.length === 1) return route.fulfill({ json: { code: 500, message: '复制未完成，请重试', data: null } });
      const copied = { ...rows[0]!, productNo: 'product-copied', versionNo: 'ver-copied', versionNumber: 1, publicSlug: body.publicSlug, nameZhCn: body.nameZhCn, nameEnUs: body.nameEnUs, source: 'copy', status: 'draft', versionStatus: 'draft', publishedAt: null };
      rows.push(copied);
      return route.fulfill({ json: { code: 0, data: copied, message: 'success' } });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto();
    await workspace.openCopy('product-ssl');
    await expect(workspace.copyModal).toContainText('新商品为草稿，库存为0');
    await expect(adminPage.getByRole('dialog').filter({ has: workspace.copyModal })).toContainText('复制商品');
    await workspace.fillCopy('证书副本', 'Certificate copy', 'certificate-copy');
    await workspace.screenshot('copy-form');
    await workspace.confirmCopy();
    await expect(workspace.copyModal).toBeVisible();
    await expect(adminPage.getByTestId('catalog-copy-name-zh')).toHaveValue('证书副本');
    await expect(adminPage.getByTestId('catalog-copy-slug')).toHaveValue('certificate-copy');
    await expect(adminPage.getByText('复制未完成，请重试', { exact: true })).toBeVisible();
    await workspace.screenshot('copy-failure');
    await workspace.confirmCopy();
    await expect(workspace.copyModal).toBeHidden();
    expect(requests).toHaveLength(2);
    expect(requests[0]).toEqual(requests[1]);
    expect(requests[1]).toEqual({ sourceVersionNo: 'version-ssl-1', nameZhCn: '证书副本', nameEnUs: 'Certificate copy', publicSlug: 'certificate-copy' });
    await expect(workspace.productRow('product-copied')).toContainText('证书副本');
    await expect(workspace.productRow('product-ssl')).toContainText('通配符 SSL 证书');
    await workspace.screenshot('copy-succeeded');
  });
  test('TC-42b: 只有新建权限不能复制全部配置', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    let calls = 0;
    await adminPage.route('**/admin/catalog/products/*/copies', route => { calls++; return route.abort(); });
    await adminPage.route('**/api/v1/user/info', async route => {
      const response = await route.fetch({ url: `${config.apiBaseURL}user/info` });
      const body = await response.json();
      body.data.permissions = ['idc:admin:catalog:product:list', 'idc:admin:catalog:product:create', 'idc:admin:catalog:category:list'];
      await route.fulfill({ response, json: body });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto();
    await expect(adminPage.getByTestId('catalog-product-create-open')).toBeVisible();
    await expect(adminPage.getByTestId('catalog-product-copy-product-ssl')).toHaveCount(0);
    expect(calls).toBe(0);
    await workspace.screenshot('copy-permission');
  });
});
