import {expect,test} from '../support/merchant-auth';
import {ProductDirectoryPage} from '../pages/ProductDirectoryPage';
import {installDirectoryMock} from '../support/product-directory-mock';
test.describe('TC-23 商家服务菜单与自动分类',()=>{
 test('TC-23a: 导航只编辑菜单信息并保留商品编辑中的归属',async({adminPage})=>{
  const state=await installDirectoryMock(adminPage);const view=new ProductDirectoryPage(adminPage);
  await view.openMerchant();await expect(view.merchant).not.toContainText('关联商品数');await expect(view.merchant).not.toContainText('plugin.idc-core.productDirectory');
  await view.create();await view.fill('新建产品菜单');await view.editor.getByRole('textbox',{name:'英文名称'}).fill('');await expect(view.editor.getByRole('combobox',{name:'关联商品'})).toHaveCount(0);await view.screenshot('merchant-editor');
  await view.save();await expect(view.editor).toBeHidden();expect(state.writes.at(-1)).not.toHaveProperty('productNos');expect(state.writes.at(-1)).toMatchObject({nameEnUs:'新建产品菜单'});
  await view.edit('云服务器');await view.fill('云服务器目录');await expect(view.editor.getByRole('combobox',{name:'菜单类型'})).toBeDisabled();
  state.failNext=true;await view.save();await expect(view.editor).toBeVisible();await expect(adminPage.getByText('菜单信息冲突，请重新确认',{exact:true})).toBeVisible();await view.screenshot('merchant-conflict');
  await view.save();await expect(view.editor).toBeHidden();expect(state.writes.at(-1)).toMatchObject({nameZhCn:'云服务器目录',nameEnUs:'Cloud servers'});expect(state.writes.at(-1)).not.toHaveProperty('productNos');await expect(view.merchant).toContainText('云服务器目录');
  expect(state.directory.productNos).toEqual(['prd_ui_cloud']);expect(state.directory.productCount).toBe(1);await view.screenshot('merchant-saved');
 });
 test('TC-23b: 后台菜单选中服务分类、深链接刷新和历史恢复',async({page})=>{
  const state=await installDirectoryMock(page,true);const view=new ProductDirectoryPage(page);
  await view.loginCustomer();await expect(view.customer).toContainText('已购云服务器');
  await expect(page.getByTestId('directory-settings')).toHaveCount(0);
  await view.selectMenu('SSL证书');await expect(page).toHaveURL(/product-directories\/dir_ui_ssl$/u);
  await expect(view.customer).toContainText('已购SSL证书');await expect(view.customer.getByRole('button',{name:'SSL证书 (1)',exact:true})).toHaveClass(/ant-btn-primary/u);await view.screenshot('customer-menu-ssl');
  await view.switchLanguage('English');await expect(page.getByRole('menuitem',{name:'SSL certificates',exact:true})).toBeVisible();await expect(view.customer).toContainText('Purchased SSL certificate');
  await view.switchLanguage('简体中文');await expect(page.getByRole('menuitem',{name:'SSL证书',exact:true})).toBeVisible();
  await page.reload();await view.customer.waitFor();await expect(view.customer).toContainText('已购SSL证书');
  await view.selectMenu('云服务器');await expect(view.customer).toContainText('已购云服务器');
  await page.goBack();await expect(view.customer).toContainText('已购SSL证书');
  await view.selectTab('全部服务');await expect(page).toHaveURL(/product-directories$/u);await expect(view.customer).toContainText('已购云服务器');
  expect(state.services).toContain('dir_ui_ssl');expect(state.services).toContain('dir_ui_cloud');expect(state.services).toContain('all');expect(state.writes).toHaveLength(0);
 });
 test('TC-23c: 目录加载失败保留全部服务并可重试',async({page})=>{
  const state=await installDirectoryMock(page,true);state.directoryFailure=true;const view=new ProductDirectoryPage(page);
  await view.loginCustomer();await expect(view.customer).toContainText('目录加载失败');await expect(view.customer).toContainText('已购云服务器');await view.screenshot('customer-directory-error');
  const retry=view.customer.getByRole('button',{name:/^重\s*试$/u});await expect(retry).toBeVisible();state.directoryFailure=false;await retry.click();await expect(view.customer.getByRole('button',{name:'云服务器 (1)',exact:true})).toBeVisible();await expect(view.customer).toContainText('已购云服务器');await expect(page.getByRole('menuitem',{name:'SSL证书',exact:true})).toBeVisible();
 });
 test('TC-23d: 第101个后台菜单自动定位到对应分类页',async({page})=>{
  const state=await installDirectoryMock(page,true);state.paginated=true;const view=new ProductDirectoryPage(page);
  await view.loginCustomer();await view.selectMenu('SSL证书');await expect(view.customer).toContainText('已购SSL证书');
  await expect(view.customer.getByRole('button',{name:'SSL证书 (1)',exact:true})).toHaveClass(/ant-btn-primary/u);
  await page.reload();await view.customer.waitFor();await expect(view.customer.getByRole('button',{name:'SSL证书 (1)',exact:true})).toHaveClass(/ant-btn-primary/u);
 });
 test('TC-23e: 统一两级树形列表显示图标，排序失焦后保存且失败保留输入',async({adminPage})=>{
  const state=await installDirectoryMock(adminPage);state.hierarchy=true;const view=new ProductDirectoryPage(adminPage);
  await view.openMerchant();await expect(view.merchant.getByRole('tab')).toHaveCount(0);
  await expect(view.merchant).not.toContainText('客户打开后看到');await expect(view.merchant).not.toContainText('已有外部链接');await expect(view.merchant).not.toContainText('关联商品数');
  await expect(view.merchant).toContainText('展示名称');await expect(view.merchant).toContainText('菜单类型');
  await expect(view.merchant.getByTestId('directory-name-dir_root')).toHaveText('产品与服务');
  await expect(view.merchant.getByTestId('directory-name-dir_external')).toHaveText('帮助文档');
  await expect(view.merchant.getByTestId('directory-icon-dir_ui_cloud')).toBeVisible();expect(await view.merchant.getByTestId('directory-icon-dir_ui_cloud').evaluate(element=>element.tagName.toLowerCase()==='svg'||Boolean(element.querySelector('svg')))).toBe(true);
  await expect(view.merchant.locator('.vxe-tree--line').first()).toBeVisible();
  await view.enterSort('dir_ui_cloud','7');expect(state.writes).toHaveLength(0);
  state.failNext=true;await view.commitSort('dir_ui_cloud');await expect.poll(()=>state.writes.length).toBe(1);
  await expect(adminPage.getByText('菜单信息冲突，请重新确认',{exact:true})).toBeVisible();await expect(view.sortInput('dir_ui_cloud')).toHaveValue('7');
  await view.screenshot('tree-sort-failed');await view.enterSort('dir_ui_cloud','8');await view.commitSort('dir_ui_cloud');
  await expect.poll(()=>state.writes.length).toBe(2);expect(state.writes[1]).toMatchObject({sortOrder:8,parentDirectoryNo:'dir_root',expectedVersion:1});
  expect(state.directory.productCount).toBe(1);expect(state.directory.productNos).toEqual(['prd_ui_cloud']);
  await expect(view.sortInput('dir_ui_cloud')).toHaveValue('8');await view.screenshot('tree-sort-saved');
 });
 test('TC-23f: 菜单编辑复用完整图标选择器，可搜索预设之外图标并保存父级',async({adminPage})=>{
  const state=await installDirectoryMock(adminPage);state.hierarchy=true;const view=new ProductDirectoryPage(adminPage);
  await view.openMerchant();await view.edit('云服务器');await expect(view.editor).not.toContainText('先创建客户使用的菜单');
  await expect(view.editor.getByTestId('directory-parent')).toContainText('产品与服务');
  await view.openIcons();await expect(view.iconPopover.locator('.grid button')).toHaveCount(36);
  await view.searchIcon('database-zap');await view.selectFilteredIcon();
  await expect(view.editor.getByTestId('directory-icon-picker')).toHaveValue('lucide:database-zap');
  await view.save();await expect(view.editor).toBeHidden();expect(state.writes.at(-1)).toMatchObject({icon:'lucide:database-zap',parentDirectoryNo:'dir_root'});
  await view.screenshot('full-icon-saved');
 });

 test('TC-23g: 商品菜单选择展示插件，保存失败保留选择且不改变商品归属',async({adminPage})=>{
  const state=await installDirectoryMock(adminPage);const view=new ProductDirectoryPage(adminPage);
  await adminPage.route('**/api/v1/plugins/dynamic',route=>route.fulfill({json:{code:0,data:{list:[{id:'idc-resource',installed:1,enabled:1,runtimeState:'normal',customerSurfaces:[{title:'服务器列表',routePath:'/account/plugin/idc-resource/purchased-services',slot:'customer.products',status:'active',bridgeCapabilities:['purchased-services.customer.v1']}]},{id:'disabled',installed:1,enabled:0,customerSurfaces:[{title:'已禁用列表',slot:'customer.products',bridgeCapabilities:['purchased-services.customer.v1']}]}]}}}));
  await view.openMerchant();await view.edit('云服务器');
  const selector=view.editor.getByRole('combobox',{name:'展示插件',exact:true});await expect(selector).toBeVisible();await selector.click();
  await expect(adminPage.getByText('已禁用列表',{exact:true})).toHaveCount(0);
  await adminPage.locator('.ant-select-item-option-content').filter({hasText:'服务器列表'}).click();
  state.failNext=true;await view.save();await expect(view.editor).toBeVisible();await expect(view.editor).toContainText('服务器列表');
  await expect(adminPage.getByText('菜单信息冲突，请重新确认',{exact:true})).toBeVisible();
  await view.screenshot('content-plugin-failed-retains-selection');await view.save();await expect(view.editor).toBeHidden();
  expect(state.writes.at(-1)).toMatchObject({pagePluginIds:['idc-resource']});expect(state.directory.productNos).toEqual(['prd_ui_cloud']);
  await view.edit('云服务器');await expect(view.editor).toContainText('服务器列表');await view.screenshot('content-plugin-selection-saved');
 });

});
