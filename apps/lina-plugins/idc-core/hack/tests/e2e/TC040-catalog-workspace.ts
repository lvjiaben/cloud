import { test, expect } from '../support/merchant-auth';
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
    await workspace.selectCategory('ctg_00000000000000000000000000000001');
    await workspace.createCategory('安全产品', 'Security products');
    await expect(workspace.categoryCreateModal.getByTestId('catalog-category-create-parent')).toContainText('数字产品');
    await workspace.screenshot('workspace-category-form');
    await workspace.confirmCategory();
    await expect(workspace.categoryCreateModal).toBeHidden();
    const created = state.categoryRequests.find(item => item.method === 'POST');
    expect(created?.body).toMatchObject({ parentCategoryNo: 'ctg_00000000000000000000000000000001', nameZhCn: '安全产品', nameEnUs: 'Security products' });
    await expect(workspace.categoryPanel).toContainText('安全产品');
    await workspace.screenshot('workspace-category-created');
    const all = await workspace.allProducts();
    expect(all.code).toBe(0);
    await expect(workspace.productRow('product-phone')).toBeVisible();
  });
  test('TC-40d: 分类编辑和删除后刷新且不显示回收站', async ({ adminPage }) => {
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
    await expect(workspace.categoryPanel.getByTestId('catalog-category-state')).toHaveCount(0);
    await expect(workspace.categoryPanel).not.toContainText('回收站');
    await workspace.screenshot('workspace-category-deleted');
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
    await adminPage.reload();
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
    await adminPage.reload();
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
    await adminPage.reload();
    const workspace = new CatalogWorkspacePage(adminPage);
    await workspace.goto(); await workspace.selectCategory(category);
    await expect(workspace.categoryPanel.getByTestId(`catalog-category-select-${category}`)).toBeVisible();
    for (const action of ['create', 'edit', 'delete', 'restore']) {
      await expect(workspace.categoryPanel.getByTestId(`catalog-category-${action}-inline`)).toHaveCount(0);
    }
    await expect(adminPage.getByTestId('catalog-product-more-product-ssl')).toHaveCount(0);
    await workspace.screenshot('category-read-only');
  });
  test('TC-40g: 分类商品布局、顺序保存、并发冲突与双语', async ({adminPage, mainLayout}) => {
    const state = await installCatalogManagementMock(adminPage);
    await adminPage.setViewportSize({width:1920,height:1080});
    await adminPage.reload();
    const view = new CatalogWorkspacePage(adminPage);
    let order = 10, revision = 1, reject = false;
    const writes: Record<string,unknown>[] = [];
    const rows = products.map((row,index)=>({...row,sortOrder:10+index*10,availableQuantity:index===0?7:null,provisionedQuantity:index===0?4:0,totalQuantity:index===0?5:0}));
    await adminPage.route('**/admin/catalog/products/product-ssl', async r=>{
      const {mockProductDetail} = await import('../support/catalog-mock');
      const detail:any = mockProductDetail('product-ssl');
      Object.assign(detail.product,{sortOrder:order, descriptionVersion:revision});
      await r.fulfill({json:{code:0,data:detail}});
    });
    await adminPage.route('**/admin/catalog/products/product-ssl/description',async r=>{
      const input = r.request().postDataJSON(); writes.push(input);
      if (reject) return r.fulfill({status:409,json:{code:409,message:'商品已被其他管理员修改，请重新读取。'}});
      expect(input.version).toBe(revision);
      order = input.sortOrder; revision++;
      rows[0]!.sortOrder = order;
      await r.fulfill({json:{code:0,data:{version:revision}}});
    });
    await adminPage.route('**/admin/catalog/products?**',async r=>{
      const url = new URL(r.request().url());
      state.listRequests.push(url);
      const selected = url.searchParams.get('categoryNo');
      const items = [...rows].filter(row=>!selected || row.categoryNo===selected).sort((a,b)=>a.sortOrder-b.sortOrder);
      await r.fulfill({json:{code:0,data:{products:items,total:items.length,nextCursor:''}}});
    });
    await view.goto();
    await expect(view.pageRoot.locator('h1')).toHaveCount(0);
    await expect(view.categoryPanel.getByTestId('catalog-category-create-inline')).toHaveAttribute('aria-label','新建分类');
    await expect(view.categoryPanel.getByTestId('catalog-category-create-inline')).toHaveText('');
    await expect(view.categoryPanel).not.toContainText('回收站');
    await expect(view.categoryPanel).not.toContainText('未分类');
    await expect(view.pageRoot.locator('.catalog-products-header')).toContainText('新建');
    await expect(view.pageRoot.getByTestId('catalog-products-total')).toContainText('共 3 个商品');
    await expect(view.pageRoot.getByTestId('catalog-refresh')).toBeVisible();
    const pager = view.pageRoot.locator('.vxe-pager');
    await expect(pager).toBeInViewport();
    await expect(view.pageRoot.locator('.vxe-toolbar')).toHaveCount(0);
    await expect(view.pageRoot.locator('.vxe-header--column').filter({hasText:'排序值'})).toBeVisible();
    await expect(adminPage.getByTestId('catalog-product-stock-product-ssl')).toHaveText('7');
    await expect(adminPage.getByTestId('catalog-product-provisioned-product-ssl')).toHaveText('4');
    await expect(adminPage.getByTestId('catalog-product-total-product-ssl')).toHaveText('5');
    await expect(adminPage.getByTestId('catalog-product-stock-product-phone')).toHaveText('不限');
    const identity = adminPage.getByTestId('catalog-product-product-ssl');
    const nameBox = await identity.locator('.catalog-product-name').boundingBox();
    const referenceBox = await identity.locator('.catalog-product-reference').boundingBox();
    expect(Math.abs(nameBox!.x-referenceBox!.x)).toBeLessThan(1);
    await expect(identity.locator('.catalog-product-icon .iconify')).toHaveCount(1);
    await expect(view.categoryPanel.locator('.ant-tree-show-line')).toHaveCount(1);
    const draftRow = adminPage.getByTestId('catalog-product-product-manual-service').locator('xpath=ancestor::tr');
    await expect(draftRow.locator('.ant-badge-status-default')).toHaveCount(1);
    await view.screenshot('catalog-redesign-initial');
    await adminPage.getByTestId('catalog-category-search').fill('证书');
    await expect(adminPage.getByTestId(`catalog-category-select-${category}`)).toBeVisible();
    await expect(adminPage.getByTestId('catalog-category-select-ctg_00000000000000000000000000000003')).toHaveCount(0);
    await adminPage.getByTestId('catalog-category-search').fill('');
    await view.selectCategory('ctg_00000000000000000000000000000003');
    await view.openCategorySort(); await view.saveSort(5);
    await expect(view.sortModal).toBeHidden();
    expect(state.categoryRequests.findLast(row=>row.method==='PUT')?.body).toMatchObject({sortOrder:5,version:1,nameZhCn:'实物商品'});
    await expect(view.categoryPanel.locator('.catalog-category-node').first()).toContainText('实物商品');
    await view.openCategorySort(); await expect(adminPage.getByTestId('catalog-sort-value')).toHaveValue('5'); await view.closeModal();
    await view.allProducts();
    await view.openProductSort('product-ssl'); await view.saveSort(50);
    await expect(view.sortModal).toBeHidden();
    await expect(view.pageRoot.locator('.catalog-product-name').first()).toContainText('商务手机');
    await expect(view.pageRoot.locator('.catalog-product-name').last()).toContainText('通配符');
    await adminPage.reload(); await expect(view.pageRoot.locator('.catalog-product-name').last()).toContainText('通配符');
    await view.openProductSort('product-ssl'); await expect(adminPage.getByTestId('catalog-sort-value')).toHaveValue('50');
    reject = true; await view.saveSort(1); await expect(view.sortModal).toBeVisible();
    await expect.poll(()=>writes.length).toBe(2);
    expect(order).toBe(50);
    await expect(adminPage.getByText('商品已被其他管理员修改，请重新读取。',{exact:true})).toBeVisible();
    await view.screenshot('catalog-sort-conflict'); await view.closeModal();
    await mainLayout.switchLanguage('English');
    await expect(view.pageRoot.locator('.catalog-products-header')).toContainText('All products');
    await expect(adminPage.getByTestId('catalog-search-status')).toContainText('Sales status');
    await expect(view.pageRoot.locator('.vxe-header--column').filter({hasText:'Stock'})).toBeVisible();
    await expect(view.pageRoot.locator('.vxe-header--column').filter({hasText:'Provisioned'})).toBeVisible();
    await expect(view.pageRoot.locator('.vxe-header--column').filter({hasText:'Total delivered'})).toBeVisible();
    await expect(view.pageRoot.getByTestId('catalog-product-create-open')).toContainText('New');
    await expect(view.pageRoot.getByTestId('catalog-refresh')).toHaveAttribute('aria-label','Refresh');
    await view.openProductSort('product-ssl'); await expect(view.sortModal).toContainText('Smaller values appear first.'); await view.closeModal();
    await adminPage.setViewportSize({width:390,height:844}); await view.screenshot('catalog-redesign-mobile');
    expect(await adminPage.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  });

  test('TC-40h: 长分类省略、横向滚动和删除保留规则', async({adminPage})=>{
    await installCatalogManagementMock(adminPage);
    const view=new CatalogWorkspacePage(adminPage);
    const longName='这是一个很长很长的商品分类名称用于验证单行省略不会挤压商品表格';
    await adminPage.route('**/admin/catalog/categories?**',async route=>route.fulfill({json:{code:0,data:{categories:[{categoryNo:'ctg_00000000000000000000000000000001',nameZhCn:longName,nameEnUs:'A very long category name to verify truncation',parentCategoryNo:'',sortOrder:10,status:'active',version:1}]}}}));
    let deleted=false,reject=true;const requests:string[]=[];
    await adminPage.route('**/admin/catalog/products?**',async route=>route.fulfill({json:{code:0,data:{products:deleted?products.slice(1):products,total:deleted?2:3,nextCursor:''}}}));
    await adminPage.route(/\/admin\/catalog\/products\/product-ssl(?:\?.*)?$/,async route=>{
      if(route.request().method()==='DELETE'){
        requests.push(route.request().url());
        expect(new URL(route.request().url()).searchParams.get('version')).toBe('1');
        if(reject)return route.fulfill({status:409,json:{code:409,message:'商品已被其他管理员修改，请重新读取。'}});
        deleted=true;return route.fulfill({json:{code:0,data:{}}});
      }
      const {mockProductDetail}=await import('../support/catalog-mock');
      const detail:any=mockProductDetail('product-ssl');detail.product.descriptionVersion=1;
      await route.fulfill({json:{code:0,data:detail}});
    });
    await adminPage.setViewportSize({width:1280,height:900});await adminPage.reload();await view.goto();
    const label=adminPage.getByTestId('catalog-category-select-ctg_00000000000000000000000000000001').locator('span');
    await expect(label).toHaveAttribute('title',longName);
    const sizing=await label.evaluate(el=>({client:el.clientWidth,scroll:el.scrollWidth,height:el.getBoundingClientRect().height,lineHeight:getComputedStyle(el).lineHeight,whiteSpace:getComputedStyle(el).whiteSpace,ellipsis:getComputedStyle(el).textOverflow}));
    expect(sizing.scroll).toBeGreaterThan(sizing.client);expect(sizing.whiteSpace).toBe('nowrap');expect(sizing.ellipsis).toBe('ellipsis');expect(sizing.height).toBeLessThanOrEqual(parseFloat(sizing.lineHeight)+1);
    const body=view.pageRoot.locator('.vxe-table--scroll-x-handle');
    expect(await body.evaluate(el=>el.scrollWidth>el.clientWidth)).toBe(true);
    await body.evaluate(el=>{el.scrollLeft=el.scrollWidth});
    await expect(adminPage.getByTestId('catalog-product-more-product-ssl')).toBeInViewport();
    await view.screenshot('catalog-long-label-horizontal-scroll');
    await view.openProductMore('product-ssl');await adminPage.getByTestId('catalog-product-delete-product-ssl').click();
    await expect(adminPage.getByText('商品将从列表和官网移除。历史订单、已有服务和续费保留，不会删除服务器。',{exact:true})).toBeVisible();
    await adminPage.locator('.ant-popover:visible').getByRole('button',{name:/确\s*定/}).click();
    await expect(adminPage.getByText('商品已被其他管理员修改，请重新读取。',{exact:true})).toBeVisible();
    await expect(adminPage.getByTestId('catalog-product-more-product-ssl')).toBeVisible();
    reject=false;
    await adminPage.locator('.ant-popover:visible').getByRole('button',{name:/确\s*定/}).click();
    await expect(adminPage.getByTestId('catalog-product-more-product-ssl')).toHaveCount(0);
    await expect(adminPage.getByTestId('catalog-products-total')).toContainText('共 2 个商品');
    expect(requests).toHaveLength(2);await adminPage.reload();await expect(adminPage.getByTestId('catalog-product-more-product-ssl')).toHaveCount(0);
    await view.screenshot('catalog-product-delete-keeps-history');
  });

});
