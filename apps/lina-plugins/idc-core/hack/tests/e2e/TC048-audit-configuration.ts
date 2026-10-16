// Confirm the real menu entry reaches both automation configuration surfaces.
import path from 'node:path';
import { randomBytes } from 'node:crypto';
import { CatalogPricingFixture } from '../support/catalog-pricing-fixture';
import { CatalogManagementPage } from '../pages/CatalogManagementPage';
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';
import { test,expect } from '../support/catalog-auth';
import { AutomationConfigurationPage } from '../pages/AutomationConfigurationPage';
test.describe('TC-48 自动化配置入口',()=>{
 test('TC-48a: 系统配置打开服务器和其他产品接口，能进入新增接口',async({adminPage})=>{
  const view=new AutomationConfigurationPage(adminPage);
  await view.goto(); await view.openAutomation();
  await expect(adminPage.getByRole('tab',{name:'服务器接口',exact:true})).toBeVisible();
  await expect(adminPage.getByTestId('provider-configuration-page')).toBeVisible();
  await view.openServerCreation(); await expect(view.dialog).toBeVisible();
  await expect(view.dialog).toContainText('连接器');
  await adminPage.screenshot({path:path.resolve(process.cwd(),'../../temp/20261015/224000-audit-provider-create.png')});
  await view.closeCreation(); expect((await view.openProductInterfaces()).code).toBe(0);
  await expect(adminPage.getByRole('button',{name:'新增接口',exact:true})).toBeVisible();
  await adminPage.screenshot({path:path.resolve(process.cwd(),'../../temp/20261015/224000-audit-product-interface.png')});
 });
});

test('TC-48b: 附加商品保存写入审计，旧版本失败不覆盖', async ({ adminPage }) => {
 const fixture = new CatalogPricingFixture(randomBytes(16).toString('hex'));
 const catalog = new CatalogManagementPage(adminPage);
 const productCondition = " WHERE product_no='" + fixture.product() + "'";
 const auditCondition = " WHERE target_no='" + fixture.product() + "' AND operation_code='catalog.product-addons.update'";
 try {
  fixture.seed();
  await catalog.goto(); await catalog.searchNamePrefix(fixture.productName());
  await catalog.openProductDetail(fixture.product()); await catalog.selectEditSection('附加商品');
  const response = adminPage.waitForResponse(r => r.request().method() === 'PUT' && r.url().endsWith('/' + fixture.product() + '/addons'));
  await catalog.saveAddons();
  const saved = await (await response).json(); expect(saved.code, JSON.stringify(saved)).toBe(0);
  expect(queryPgScalar('SELECT version FROM plugin_idc_core_catalog_product' + productCondition)).toBe('2');
  expect(queryPgScalar('SELECT count(*) FROM plugin_idc_core_business_audit_event' + auditCondition + " AND result_code='succeeded'")).toBe('1');
  await adminPage.screenshot({ path: path.resolve(process.cwd(), '../../temp/20261015/224000-audit-addons-saved.png') });
  const result = await catalog.writeAddons(fixture.product(), [fixture.product(1)], 2); expect(result.code, JSON.stringify(result)).toBe(0);
  expect(queryPgScalar('SELECT addon_product_no FROM plugin_idc_core_catalog_product_addon' + productCondition)).toBe(fixture.product(1));
  const conflict = await catalog.writeAddons(fixture.product(), [], 2); expect(conflict.code).not.toBe(0);
  expect(queryPgScalar('SELECT version FROM plugin_idc_core_catalog_product' + productCondition)).toBe('3');
  expect(queryPgScalar('SELECT count(*) FROM plugin_idc_core_business_audit_event' + auditCondition)).toBe('2');
 } finally {
  execPgSQLStatements(['BEGIN;', 'SET LOCAL session_replication_role=replica;',
   'DELETE FROM plugin_idc_core_business_audit_event' + auditCondition + ';',
   'DELETE FROM plugin_idc_core_catalog_product_addon' + productCondition + ';', 'COMMIT;']);
  fixture.cleanup();
 }
});
