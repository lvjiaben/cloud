import { config, pluginApiPath, workspacePath } from '@host-tests/fixtures/config';
import { expect, test } from '@host-tests/fixtures/auth';
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';
import { CurrencyManagementPage } from '../pages/CurrencyManagementPage';

test.describe('TC-38 货币配置真实业务闭环', () => {
  test('TC-38a: 添加、停用、启用、默认保护和持久化', async ({ adminPage }) => {
    test.setTimeout(120_000);
    const merchantBackup = queryPgScalar("SELECT row_to_json(m)::text FROM plugin_idc_core_merchant_profile m WHERE tenant_id=0");
    const backup = queryPgScalar("SELECT COALESCE(json_agg(t)::text,'[]') FROM (SELECT * FROM plugin_idc_core_merchant_currency WHERE currency_code IN ('USD','CNY')) t");
    const auditFloor = queryPgScalar("SELECT COALESCE(MAX(id),0)::text FROM plugin_idc_core_business_audit_event");
    const view = new CurrencyManagementPage(adminPage);
    execPgSQLStatements(["UPDATE plugin_idc_core_merchant_profile SET default_currency='CNY' WHERE tenant_id=0;", "DELETE FROM plugin_idc_core_merchant_currency WHERE tenant_id=0 AND currency_code='USD';"]);
    try {
      const initial = await view.goto(); expect(initial.code).toBe(0);
      await expect(view.workspace).toContainText('每个币种独立定价和结算，不自动换汇。'); await view.screenshot('initial');
      await view.openCreate(); await view.select('USD'); await expect(view.modal).toContainText('货币'); await view.screenshot('create-modal');
      const created = await view.submit(); expect(created.code, JSON.stringify(created)).toBe(0); expect(created.data).toMatchObject({ currencyCode: 'USD', enabled: true, version: 1 });
      await expect(view.modal).toBeHidden(); await expect(view.workspace).toContainText('美元 (USD)'); expect(queryPgScalar("SELECT enabled::text FROM plugin_idc_core_merchant_currency WHERE currency_code='USD'")).toBe('true'); await view.screenshot('created');
      const disabled = await view.change('USD'); expect(disabled.code).toBe(0); expect(disabled.data.enabled).toBe(false);
      const options = await adminPage.request.get(`${config.backendBaseURL}${pluginApiPath('idc-core', 'currencies')}`); const enabled = await options.json(); expect(enabled.code).toBe(0); expect(enabled.data.currencies).not.toContain('USD');
      const reenabled = await view.change('USD'); expect(reenabled.code).toBe(0); expect(reenabled.data.enabled).toBe(true);
      const defaulted = await view.change('USD', true); expect(defaulted.code).toBe(0); expect(queryPgScalar('SELECT default_currency FROM plugin_idc_core_merchant_profile WHERE tenant_id=0')).toBe('USD');
      await expect(adminPage.getByTestId('currency-toggle-USD')).toHaveCount(0); await expect(adminPage.getByTestId('currency-delete-USD')).toHaveCount(0); await view.screenshot('default-protected');
      await view.goto(); await expect(view.workspace).toContainText('美元 (USD)');
      await view.openCreate(); await view.select('USD'); const duplicate = await view.submit(); expect(duplicate.code).not.toBe(0); await expect(view.modal).toContainText('添加失败，请检查货币是否已存在后重试。'); await view.screenshot('duplicate'); await view.close();
      await view.change('CNY', true);
      const deleted = await view.remove('USD'); expect(deleted.code).toBe(0);
      expect(queryPgScalar("SELECT deleted::text FROM plugin_idc_core_merchant_currency WHERE currency_code='USD'")).toBe('true');
      await expect(view.workspace).not.toContainText('美元 (USD)'); await view.screenshot('deleted');
      const afterDelete = await adminPage.request.get(`${config.backendBaseURL}${pluginApiPath('idc-core', 'currencies')}`);
      expect((await afterDelete.json()).data.currencies).not.toContain('USD');
      await view.openCreate(); await view.select('USD'); const recreated = await view.submit(); expect(recreated.code).toBe(0); expect(recreated.data.version).toBeGreaterThan(defaulted.data.version);
      await expect(view.workspace).toContainText('美元 (USD)'); await view.screenshot('recreated');
      expect(queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_business_audit_event WHERE id>${auditFloor} AND target_type='merchant_currency' AND target_no IN ('USD','CNY')`)).not.toBe('0');
    } finally {
      execPgSQLStatements(["SET session_replication_role = replica;", `DELETE FROM plugin_idc_core_business_number WHERE business_no IN (SELECT audit_no FROM plugin_idc_core_business_audit_event WHERE id>${auditFloor} AND target_type='merchant_currency' AND target_no IN ('USD','CNY'));`, `DELETE FROM plugin_idc_core_business_audit_event WHERE id>${auditFloor} AND target_type='merchant_currency' AND target_no IN ('USD','CNY');`, "SET session_replication_role = origin;", `UPDATE plugin_idc_core_merchant_profile m SET default_currency=b.default_currency,version=b.version,updated_by=b.updated_by,updated_at=b.updated_at FROM json_populate_record(NULL::plugin_idc_core_merchant_profile,'${merchantBackup.replaceAll("'", "''")}') b WHERE m.tenant_id=0;`,"DELETE FROM plugin_idc_core_merchant_currency WHERE currency_code IN ('USD','CNY');",`INSERT INTO plugin_idc_core_merchant_currency SELECT * FROM json_populate_recordset(NULL::plugin_idc_core_merchant_currency, '${backup.replaceAll("'", "''")}');`]);
    }
  });

  test('TC-38b: 系统配置入口可达且不显示无关操作', async ({ adminPage }) => {
    test.setTimeout(60_000);
    const view = new CurrencyManagementPage(adminPage);
    await adminPage.goto(workspacePath('/system/configuration-center'));
    await adminPage.getByRole('tab', { name: '支付与货币' }).click();
    const card = adminPage.locator('.configuration-item').filter({ has: adminPage.getByText('货币配置', { exact: true }) });
    const loaded = adminPage.waitForResponse(r => r.url().includes('/admin/currencies') && r.request().method() === 'GET');
    await card.getByRole('button', { name: '进入配置' }).click();
    expect((await (await loaded).json()).code).toBe(0);
    await expect(view.workspace).toContainText('人民币 (CNY)');
    await expect(view.workspace).toContainText('默认货币');
    await view.screenshot('configuration-entry');
    await expect(view.workspace.getByRole('button', { name: '商品定价' })).toHaveCount(0);
    await expect(view.workspace.getByRole('button', { name: '支付方式' })).toHaveCount(0);
  });
});
