import {expect,test} from '../support/catalog-auth';
import {ProductDirectoryPage} from '../pages/ProductDirectoryPage';
import {installDirectoryMock} from '../support/product-directory-mock';
test.describe('TC-23 商家服务菜单与自动分类',()=>{
 test('TC-23a: 导航只编辑菜单信息并保留商品编辑中的归属',async({adminPage})=>{
  const state=await installDirectoryMock(adminPage);const view=new ProductDirectoryPage(adminPage);
  await view.openMerchant();await expect(view.merchant).toContainText('关联商品数');await expect(view.merchant).not.toContainText('plugin.idc-core.productDirectory');
  await view.create();await view.fill('新建产品菜单');await expect(view.editor.getByRole('combobox',{name:'关联商品'})).toHaveCount(0);await view.screenshot('merchant-editor');
  await view.save();await expect(view.editor).toBeHidden();expect(state.writes.at(-1)).not.toHaveProperty('productNos');
  await view.edit('云服务器');await view.fill('云服务器目录');
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
});
