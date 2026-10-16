import { expect, test } from '../support/catalog-auth';
import { ProductMemberMenuPage } from '../pages/ProductMemberMenuPage';
import { installCatalogManagementMock, mockProductDetail } from '../support/catalog-mock';

// UI fixtures prove selection/write contracts; PostgreSQL tests prove atomic reassignment.
test.describe("TC-51 商品会员导航归属", () => {
test('TC-51a: 新商品可选第二页产品菜单，系统菜单不能选', async ({ adminPage }) => {
  const state = await installCatalogManagementMock(adminPage);
  const menu = (directoryNo: string, nameZhCn: string, navigationType = 'product') => ({
    directoryNo, nameZhCn, nameEnUs: nameZhCn, navigationType, statusCode: 'enabled',
    productNos: [], products: [], productCount: 0, sortOrder: 0, version: 1, icon: 'lucide:server', targetUrl: '',
  });
  await adminPage.route('**/admin/product-directories?**', route => {
    const page = Number(new URL(route.request().url()).searchParams.get('page'));
    const items = page === 1 ? Array.from({ length: 100 }, (_, i) => menu('system-' + i, '系统菜单' + i, 'system')) : [menu('dir-last', '云服务器')];
    return route.fulfill({ json: { code: 0, data: { items, total: 101 } } });
  });
  const view = new ProductMemberMenuPage(adminPage);
  await view.goto();
  await view.openProductCreate();
  await view.selectProductCategory();
  await view.createMenu.click();
  await expect(view.menuOptions).not.toContainText('系统菜单');
  await view.menuOptions.getByText('云服务器', { exact: true }).click();
  await view.fillNewProduct();
  await view.createProduct();
  await expect(view.productCreateModal).toBeHidden();
  expect(state.productCreateRequests.at(-1)?.body).toMatchObject({ directoryNo: 'dir-last' });
  await view.screenshot('member-menu-created');
});

test('TC-51b: 商品编辑切换会员菜单，失败保留选择并可重试', async ({ adminPage }) => {
  await installCatalogManagementMock(adminPage);
  const menus = ['旧服务菜单', '新服务菜单'].map((name, index) => ({
    directoryNo: 'dir-' + index, nameZhCn: name, nameEnUs: name, navigationType: 'product', statusCode: 'enabled',
    productNos: [], products: [], productCount: 0, sortOrder: 0, version: 1, icon: 'lucide:server', targetUrl: '',
  }));
  await adminPage.route('**/admin/product-directories?**', route => route.fulfill({ json: { code: 0, data: { items: menus, total: 2 } } }));
  await adminPage.route('**/admin/catalog/products/product-manual-service', route => {
    const data = mockProductDetail('product-manual-service');
    Object.assign(data.product, { directoryNo: 'dir-0', descriptionVersion: 1 });
    return route.fulfill({ json: { code: 0, data } });
  });
  let fail = true; const writes: Array<Record<string, unknown>> = [];
  await adminPage.route('**/admin/catalog/products/product-manual-service/description', route => {
    writes.push(route.request().postDataJSON());
    return route.fulfill({ json: fail ? { code: 409, message: '保存失败，请重试' } : { code: 0, data: { version: 2 } } });
  });
  const view = new ProductMemberMenuPage(adminPage);
  await view.goto(); await view.openProductDetail('product-manual-service');
  await view.editDescription();
  const select = view.editMenu;
  await expect(select).toContainText('旧服务菜单');
  await view.selectMenu('新服务菜单', true);
  await view.saveDescription();
  await expect(adminPage.getByText('保存失败，请重试', { exact: true })).toBeVisible();
  await expect(select).toContainText('新服务菜单');
  fail = false; await view.saveDescription();
  await expect(view.descriptionEditor).toHaveCount(0);
  expect(writes).toHaveLength(2);
  expect(writes[1]).toMatchObject({ directoryNo: 'dir-1', version: 1 });
  await view.screenshot('member-menu-updated');
});

});
