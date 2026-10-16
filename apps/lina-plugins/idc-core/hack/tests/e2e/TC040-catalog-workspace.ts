import { test, expect } from '../support/catalog-auth';
import { CatalogWorkspacePage } from '../pages/CatalogWorkspacePage';
import { installCatalogManagementMock, products } from '../support/catalog-mock';
import { config } from '@host-tests/fixtures/config';

const category = 'ctg_00000000000000000000000000000002';

test.describe('TC-40 分类商品同页工作区', () => {
  test('TC-40a: 分类筛选与新建预选、分类新增刷新', async ({ adminPage }) => {
    const state = await installCatalogManagementMock(adminPage);
    await adminPage.route('**/admin/catalog/products?**', route => {
      const no = new URL(route.request().url()).searchParams.get('categoryNo');
      const items = no ? products.filter(p => p.categoryNo === no) : products;
      return route.fulfill({ json: { code: 0, data: { products: items, total: items.length, nextCursor: '' }, message: 'success' } });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto();
    await expect(workspace.categoryPanel).toContainText('商品分类');
    await expect(workspace.categoryPanel).not.toContainText('plugin.idc-core');
    await workspace.screenshot('workspace-initial');
    const result = await workspace.selectCategory(category);
    expect(result.code).toBe(0);
    await expect(workspace.productRow('product-ssl')).toBeVisible();
    await expect(workspace.productRow('product-phone')).toHaveCount(0);
    await workspace.screenshot('workspace-category-filter');
    await workspace.openProductCreate();
    await expect(workspace.productCreateModal.getByTestId('catalog-product-create-category')).toContainText('证书服务');
    await workspace.screenshot('workspace-product-preselected');
    await workspace.closeModal();
    await workspace.createCategory('安全产品', 'Security products');
    await expect(workspace.categoryCreateModal.getByTestId('catalog-category-create-parent')).toContainText('证书服务');
    await workspace.screenshot('workspace-category-form');
    await workspace.confirmCategory();
    await expect(workspace.categoryCreateModal).toBeHidden();
    const created = state.categoryRequests.find(item => item.method === 'POST');
    expect(created?.body).toMatchObject({ parentCategoryNo: category, nameZhCn: '安全产品', nameEnUs: 'Security products' });
    await expect(workspace.categoryPanel).toContainText('安全产品');
    await workspace.screenshot('workspace-category-created');
    const all = await workspace.allProducts();
    expect(all.code).toBe(0);
    await expect(workspace.productRow('product-phone')).toBeVisible();
  });
  test('TC-40d: 分类编辑、可恢复删除及恢复后刷新', async ({ adminPage }) => {
    const state = await installCatalogManagementMock(adminPage);
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto();
    await workspace.selectCategory(category);
    await workspace.editCategory('证书产品');
    await expect(workspace.categoryPanel.getByTestId(`catalog-category-select-${category}`)).toHaveText('证书产品');
    expect(state.categoryRequests.find(r => r.method === 'PUT')?.body).toMatchObject({ nameZhCn: '证书产品', version: 1 });
    await workspace.screenshot('workspace-category-edited');
    await workspace.mutateCategory('delete');
    await expect(workspace.categoryPanel.getByTestId(`catalog-category-select-${category}`)).toHaveCount(0);
    expect(state.categoryRequests.find(r => r.method === 'DELETE')?.url.searchParams.get('version')).toBe('2');
    await workspace.showDeletedCategories();
    await workspace.selectCategory(category);
    await expect(workspace.categoryPanel.getByTestId('catalog-category-restore-inline')).toBeVisible();
    await workspace.screenshot('workspace-category-deleted');
    await workspace.mutateCategory('restore');
    await expect(workspace.categoryPanel.getByTestId(`catalog-category-select-${category}`)).toHaveCount(0);
    expect(state.categoryRequests.find(r => r.url.pathname.endsWith('/restore'))?.body).toMatchObject({ version: 3 });
    await workspace.screenshot('workspace-category-restored');
  });
  test('TC-40b: 分类失败可以重试，不阻断商品列表', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    let fail = true;
    await adminPage.route('**/admin/catalog/categories**', route => fail
      ? route.fulfill({ json: { code: 500, message: '分类加载失败', data: null } }) : route.fallback());
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto();
    await expect(workspace.categoryPanel).toContainText('分类加载失败，请重试。');
    await expect(workspace.productRow('product-ssl')).toBeVisible();
    await workspace.screenshot('workspace-category-failed');
    fail = false;
    await workspace.retryCategories();
    await expect(workspace.categoryPanel.getByTestId(`catalog-category-select-${category}`)).toBeVisible();
    await expect(workspace.categoryPanel).not.toContainText('分类加载失败，请重试。');
    await workspace.screenshot('workspace-category-recovered');
  });
  test('TC-40c: 无分类权限隐藏分类入口并不请求分类', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    let categoryCalls = 0;
    adminPage.on('request', r => { if (r.url().includes('/catalog/categories')) categoryCalls++; });
    await adminPage.route('**/api/v1/user/info', async route => {
      const response = await route.fetch({ url: `${config.apiBaseURL}user/info` }); const body = await response.json();
      body.data.permissions = ['idc:admin:catalog:product:list'];
      await route.fulfill({ response, json: body });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto();
    await expect(workspace.categoryPanel).toHaveCount(0);
    await expect(workspace.pageRoot.getByTestId('catalog-search-category')).toHaveCount(0);
    await expect(workspace.productRow('product-ssl')).toBeVisible();
    expect(categoryCalls).toBe(0);
    await workspace.screenshot('workspace-no-category-permission');
  });
  test('TC-40e: 删除失败保留分类，可以重试', async ({ adminPage }) => {
    const state = await installCatalogManagementMock(adminPage);
    let failed = false;
    await adminPage.route('**/admin/catalog/categories/**', route => {
      if (route.request().method() === 'DELETE' && !failed) {
        failed = true;
        return route.fulfill({ json: { code: 500, message: '分类删除失败，请重试', data: null } });
      }
      return route.fallback();
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.selectCategory(category);
    await workspace.mutateCategory('delete');
    await expect(adminPage.getByText('分类删除失败，请重试', { exact: true })).toBeVisible();
    await expect(workspace.categoryPanel.getByTestId(`catalog-category-select-${category}`)).toBeVisible();
    await workspace.screenshot('category-delete-failed');
    await workspace.mutateCategory('delete');
    await expect(workspace.categoryPanel.getByTestId(`catalog-category-select-${category}`)).toHaveCount(0);
    expect(state.categoryRequests.filter(row => row.method === 'DELETE')).toHaveLength(1);
  });
  test('TC-40f: 可查看分类但不能维护', async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    await adminPage.route('**/api/v1/user/info', async route => {
      const response = await route.fetch({ url: `${config.apiBaseURL}user/info` }); const body = await response.json();
      body.data.permissions = ['idc:admin:catalog:product:list', 'idc:admin:catalog:category:list'];
      await route.fulfill({ response, json: body });
    });
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.selectCategory(category);
    await expect(workspace.categoryPanel.getByTestId(`catalog-category-select-${category}`)).toBeVisible();
    for (const action of ['create', 'edit', 'delete', 'restore']) {
      await expect(workspace.categoryPanel.getByTestId(`catalog-category-${action}-inline`)).toHaveCount(0);
    }
    await workspace.screenshot('category-read-only');
  });
});
