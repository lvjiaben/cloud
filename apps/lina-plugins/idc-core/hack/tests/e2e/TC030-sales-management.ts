// Real API and browser acceptance for usable agent management. Only this test's
// uniquely named plugin records are cleaned up; no host records are changed.
import { randomBytes } from 'node:crypto';
import { expect, test } from '@host-tests/fixtures/auth';
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';
import { SalesManagementPage } from '../pages/SalesManagementPage';
import { SalesPerformanceFixture } from '../support/sales-performance-fixture';
import { SalesPayoutFixture } from '../support/sales-payout-fixture';

test.describe('TC-30 代理销售真实管理', () => {
  test('TC-30c: 无佣金仍有逐币种业绩，月份退款与历史归属正确', async ({ adminPage }) => {
    test.setTimeout(90_000);
    const fixture = new SalesPerformanceFixture(randomBytes(16).toString('hex'));
    const sales = new SalesManagementPage(adminPage);
    try {
      fixture.seed(); expect(fixture.commissionCount()).toBe('0');
      await sales.goto();
      const initial = await sales.openPerformance(); expect(initial.code, JSON.stringify(initial)).toBe(0);
      await expect(sales.performancePanel).toContainText('没有佣金规则也有业绩');
      await expect(sales.performancePanel).toContainText('到账月份');
      await sales.screenshot('performance-initial');
      const result = await sales.searchPerformance('2026-09', fixture.name);
      expect(result.code, JSON.stringify(result)).toBe(0); expect(result.data.total).toBe(2);
      const cny = result.data.items.find((row: {currencyCode: string}) => row.currencyCode === 'CNY');
      expect(cny).toMatchObject({ beneficiaryType: 'affiliate', beneficiaryReference: fixture.referrer, beneficiaryName: fixture.name, paidOrderCount: 2, paidRenewalCount: 1, receivedMinor: '12000', refundedMinor: '3000', netReceivedMinor: '9000' });
      const usd = result.data.items.find((row: {currencyCode: string}) => row.currencyCode === 'USD');
      expect(usd).toMatchObject({ paidOrderCount: 1, paidRenewalCount: 0, receivedMinor: '700', refundedMinor: '0', netReceivedMinor: '700' });
      await expect(sales.performancePanel.getByText(fixture.name, { exact: true })).toHaveCount(2);
      await sales.screenshot('performance-month-filter');
      await sales.showPerformanceMoney();
      await expect(sales.performancePanel).toContainText('累计退款');
      await expect(sales.performancePanel.getByText('CNY 90.00', { exact: true })).toBeVisible();
      await expect(sales.performancePanel.getByText('CNY 30.00', { exact: true })).toBeVisible();
      await sales.screenshot('performance-cumulative-refund');
      const filtered = await sales.searchPerformance('2026-09', fixture.name, 'USD');
      expect(filtered.code).toBe(0); expect(filtered.data.total).toBe(1); expect(filtered.data.items[0].currencyCode).toBe('USD');
      await expect(sales.performancePanel.getByText('CNY', { exact: true })).toHaveCount(0);
      await expect(sales.performancePanel.getByText('USD', { exact: true })).toBeVisible();
      await sales.screenshot('performance-currency-filter');
      const absent = await sales.searchPerformance('2026-09', `新归属${fixture.suffix.slice(0,8)}`);
      expect(absent.code).toBe(0); expect(absent.data.total).toBe(0);
      await expect(sales.performancePanel.getByText('暂无数据', { exact: true }).filter({ visible: true })).toBeVisible();
      await sales.screenshot('performance-new-owner-empty');
      expect(fixture.commissionCount()).toBe('0');
    } finally { fixture.cleanup(); }
  });
  test('TC-30b: 成熟佣金登记凭证、幂等重试及待扣回阻断，消费余额不变', async ({ adminPage }) => {
    test.setTimeout(120_000);
    const fixture = new SalesPayoutFixture(randomBytes(16).toString('hex'));
    const sales = new SalesManagementPage(adminPage);
    const evidence = `银行流水验收 ${fixture.suffix}`;
    try {
      fixture.seed();
      const spending = fixture.spendingState();
      expect(spending).toBe('CNY:1:12345,USD:1:12345');
      expect(fixture.accountState('CNY')).toBe('1000,0,1000,0,3');
      expect(fixture.accountState('USD')).toBe('1000,0,1000,0,3');
      await sales.openCommissionAccounts(fixture.name);
      await expect(sales.text('CNY 10.00').first()).toBeVisible();
      await expect(sales.text('USD 10.00').first()).toBeVisible();
      await sales.screenshot('payout-accounts-funded');
      await sales.openPayout('CNY');
      await expect(sales.drawer).toBeVisible();
      await sales.fill('amount', '6.00'); await sales.fill('evidenceReference', evidence);
      await expect(sales.drawer).toContainText('实际打款时间');
      await sales.screenshot('payout-form');
      const submitted = sales.waitForResponse(response => response.url().includes('/commission/payouts') && response.request().method() === 'POST');
      await sales.button(/确\s*认/u).click();
      const response = await submitted; const result = await response.json();
      expect(result.code, JSON.stringify(result)).toBe(0);
      expect(result.data.item.currencyCode).toBe('CNY');
      expect(result.data.item.amountMinor).toBe('600');
      expect(result.data.item.evidenceReference).toBe(evidence);
      await expect(sales.drawer).toBeHidden();
      expect(fixture.accountState('CNY')).toBe('1000,0,400,600,4');
      expect(fixture.accountState('USD')).toBe('1000,0,1000,0,3');
      expect(fixture.payoutCount()).toBe('1');
      expect(queryPgScalar(`SELECT SUM(amount_minor) FROM plugin_idc_core_sales_commission_offline_payout_item WHERE commission_no='${fixture.commission('CNY')}'`)).toBe('600');
      expect(queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_sales_commission_account_entry WHERE account_no='${fixture.account('CNY')}' AND entry_type_code='payout' AND delta_available_minor=-600 AND delta_paid_minor=600`)).toBe('1');
      expect(fixture.spendingState()).toBe(spending);
      const replay = await sales.replayPayout(response);
      expect(replay.code, JSON.stringify(replay)).toBe(0); expect(replay.data.item.payoutNo).toBe(result.data.item.payoutNo);
      expect(fixture.payoutCount()).toBe('1'); expect(fixture.accountState('CNY')).toBe('1000,0,400,600,4');
      await sales.screenshot('payout-recorded');
      await sales.tab('打款记录').click();
      await expect(sales.text(evidence)).toBeVisible(); await sales.screenshot('payout-evidence');
      await sales.openCommissionAccounts(fixture.name);
      await sales.openPayout('CNY'); await sales.fill('amount', '1.00'); await sales.fill('evidenceReference', `${evidence} 退款后新申请`);
      // A refund arriving after the drawer opens must be rejected by the server.
      fixture.addRecovery();
      const rejected = sales.waitForResponse(next => next.url().includes('/commission/payouts') && next.request().method() === 'POST');
      await sales.button(/确\s*认/u).click(); const rejection = await (await rejected).json();
      expect(rejection.code).not.toBe(0);
      expect(JSON.stringify(rejection)).toContain('IDC_SALES_MANAGEMENT_RECOVERY_DUE');
      await expect(sales.formError).toBeVisible(); await expect(await sales.input('amount')).toHaveValue('1.00');
      await sales.screenshot('payout-refund-rejected');
      await sales.button(/取\s*消/u).click(); await sales.openCommissionAccounts(fixture.name);
      await expect(sales.text('先抵扣待追回款')).toBeVisible();
      await expect(await sales.payoutButton('CNY')).toBeDisabled();
      await expect(await sales.payoutButton('USD')).toBeEnabled();
      await sales.screenshot('payout-recovery-blocked');
      expect(fixture.accountState('CNY')).toBe('1000,0,400,600,4');
      expect(fixture.accountState('USD')).toBe('1000,0,1000,0,3');
      expect(fixture.payoutCount()).toBe('1'); expect(fixture.spendingState()).toBe(spending);
      fixture.addPending();
      const records = await sales.openCommissionRecords(fixture.suffix);
      expect(records.code, JSON.stringify(records)).toBe(0);
      expect(records.data.items.some((item: {status: string}) => item.status === 'pending_maturity')).toBe(true);
      await sales.showRecordStatus();
      await expect(sales.statusLabel('待成熟')).toHaveText('待成熟');
      await sales.statusLabel('待成熟').scrollIntoViewIfNeeded(); await sales.screenshot('commission-pending-translated');
      expect(fixture.accountState('USD')).toBe('1200,200,1000,0,4'); expect(fixture.spendingState()).toBe(spending);
    } finally { fixture.cleanup(); }
  });
  test('TC-30a: 新增修改等级、设置代理、归属与佣金规则真实保存', async ({ adminPage }) => {
    test.setTimeout(120_000);
    const sales = new SalesManagementPage(adminPage);
    const suffix = randomBytes(16).toString('hex'); const name = `代理验收${suffix.slice(0, 8)}`;
    const customer = `CUS_${suffix}`; const customerName = `客户${suffix.slice(0, 8)}`;
    const product = `PRD_${suffix}`, version = `VER_${suffix}`, category = `CAT_${suffix}`, billing = `BIL_${suffix}`, referrer = `CUS_${randomBytes(16).toString('hex')}`;
    const productName = `商品${suffix.slice(0, 8)}`, referrerName = `推广${suffix.slice(0, 8)}`;
    let levelNo = ''; const secondLevel = `ALV_${suffix}`;
    try {
      execPgSQLStatements([`INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${customer}','individual','active','${customerName}','${customerName}');`,
        `INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${referrer}','individual','active','${referrerName}','${referrerName}');`,
        `INSERT INTO plugin_idc_core_customer_individual_profile(customer_no,identity_status_code) VALUES('${customer}','unverified'),('${referrer}','unverified');`,
        `INSERT INTO plugin_idc_core_catalog_category(category_no,name_zh_cn,name_en_us,status_code,created_by,updated_by) VALUES('${category}','${productName}','Sales acceptance','active',1,1);`,
        `INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us,created_by,updated_by) VALUES('${product}','${category}','active','sales-${suffix}','${productName}','Sales acceptance',1,1);`,
        `INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code,source_version,source_fingerprint,created_by) VALUES('${version}','${product}',1,'draft','manual','1',repeat('a',64),1);`,
        `UPDATE plugin_idc_core_catalog_product SET current_version_no='${version}' WHERE product_no='${product}';`,
        `INSERT INTO plugin_idc_core_catalog_billing_rule(billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,price_phase_code,quantity_mode_code,status_code) VALUES('${billing}','${version}','monthly','recurring','month',1,'new_purchase','fixed','active');`,
        `INSERT INTO plugin_idc_core_catalog_price(price_no,version_no,billing_rule_no,currency_code,amount_minor,status_code) VALUES('CPR_${suffix}','${version}','${billing}','CNY',8000,'active');`,
      ]);
      await sales.goto();
      await expect(sales.workspace).toBeVisible();
      await sales.tab('代理等级').click();
      await expect(sales.button('添加代理等级')).toBeVisible();
      await sales.screenshot('initial');
      await sales.addButton.click();
      await expect(sales.drawer).toBeVisible();
      await sales.fill('name', name); await sales.fill('englishName', `Acceptance ${suffix.slice(0, 8)}`);
      await expect(sales.field('status')).toContainText('启用');
      await sales.screenshot('level-form');
      const created = sales.waitForResponse(response => response.url().includes('/sales-management/levels') && response.request().method() === 'POST');
      await sales.button(/确\s*认/u).click();
      const body = await (await created).json(); expect(body.code, JSON.stringify(body)).toBe(0); levelNo = body.data.item.levelNo;
      expect(queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_sales_agent_level WHERE level_no='${levelNo}' AND name_zh_cn='${name}'`)).toBe('1');
      await expect(sales.drawer).toBeHidden();
      await expect(sales.text(name).first()).toBeVisible(); await sales.screenshot('level-created');
      await sales.editLevel(name);
      await sales.fill('englishName', `Updated ${suffix.slice(0, 8)}`);
      const edited = sales.waitForResponse(response => response.url().includes('/sales-management/levels') && response.request().method() === 'PUT');
      await sales.button(/确\s*认/u).click();
      expect((await (await edited).json()).code).toBe(0);
      expect(queryPgScalar(`SELECT name_en_us FROM plugin_idc_core_sales_agent_level WHERE level_no='${levelNo}'`)).toBe(`Updated ${suffix.slice(0, 8)}`);
      await expect(sales.drawer).toBeHidden();
      await sales.levelSearch.fill(name);
      await sales.levelSearch.press('Enter');
      await expect(sales.text(`Updated ${suffix.slice(0, 8)}`)).toBeVisible();
      await sales.screenshot('level-filtered');
      await sales.tab('代理客户').click();
      await sales.addButton.click();
      await sales.fill('customerNo', customerName);
      await sales.select('customerNo', `${customerName} · #${queryPgScalar(`SELECT customer_code FROM plugin_idc_core_customer_subject WHERE customer_no='${customer}'`)}`);
      await sales.select('levelNo', name);
      const assigned = sales.waitForResponse(response => response.url().includes('/sales-management/agents') && response.request().method() === 'POST');
      await sales.button(/确\s*认/u).click();
      const agentBody = await (await assigned).json(); expect(agentBody.code, JSON.stringify(agentBody)).toBe(0);
      expect(queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_sales_agent_qualification WHERE customer_no='${customer}' AND level_no='${levelNo}'`)).toBe('1');
      await expect(sales.drawer).toBeHidden();
      await expect(sales.text(customerName).first()).toBeVisible(); await sales.screenshot('agent-created');
      execPgSQLStatements([`INSERT INTO plugin_idc_core_sales_agent_level(level_no,level_code,name_zh_cn,name_en_us,status_code,version) VALUES('${secondLevel}','change-${suffix}','升级${name}','Updated tier','active',1);`]);
      await sales.editAgent(customerName); await expect(sales.field('levelNo')).not.toHaveClass(/ant-select-disabled/);
      await sales.fill('levelNo', `升级${name}`); await sales.select('levelNo', `升级${name}`);
      const reassigned = sales.waitForResponse(response => response.url().includes('/sales-management/agents') && response.request().method() === 'PUT');
      await sales.button(/确\s*认/u).click(); expect((await (await reassigned).json()).code).toBe(0);
      expect(queryPgScalar(`SELECT level_no FROM plugin_idc_core_sales_agent_qualification WHERE customer_no='${customer}'`)).toBe(secondLevel);
      await expect(sales.drawer).toBeHidden(); await sales.screenshot('agent-level-updated');
      await sales.tab('客户归属').click(); await sales.addButton.click();
      await sales.fill('customerNo', customerName); await sales.select('customerNo', `${customerName} · #${queryPgScalar(`SELECT customer_code FROM plugin_idc_core_customer_subject WHERE customer_no='${customer}'`)}`);
      await sales.fill('referrerCustomerNo', referrerName); await sales.select('referrerCustomerNo', `${referrerName} · #${queryPgScalar(`SELECT customer_code FROM plugin_idc_core_customer_subject WHERE customer_no='${referrer}'`)}`);
      await sales.screenshot('ownership-form');
      const ownership = sales.waitForResponse(response => response.url().includes('/sales-management/ownership') && response.request().method() === 'POST');
      await sales.button(/确\s*认/u).click();
      expect((await (await ownership).json()).code).toBe(0);
      expect(queryPgScalar(`SELECT referrer_customer_no FROM plugin_idc_core_sales_customer_referrer WHERE customer_no='${customer}' ORDER BY version DESC LIMIT 1`)).toBe(referrer);
      await expect(sales.drawer).toBeHidden();
      await sales.tab('佣金规则').click(); await sales.addButton.click();
      await sales.fill('productNo', productName); await sales.select('productNo', productName);
      await sales.select('currencyCode', 'CNY'); await sales.fill('percentage', '5'); await sales.screenshot('commission-rule-form');
      const rule = sales.waitForResponse(response => response.url().includes('/sales-management/commission-rules') && response.request().method() === 'POST');
      await sales.button(/确\s*认/u).click();
      expect((await (await rule).json()).code).toBe(0);
      expect(queryPgScalar(`SELECT rate_basis_points FROM plugin_idc_core_sales_commission_rule WHERE product_no='${product}'`)).toBe('500');
      await expect(sales.drawer).toBeHidden();
      await sales.addButton.click(); await sales.select('currencyCode', 'CNY'); await sales.fill('percentage', '101');
      await sales.button(/确\s*认/u).click();
      await expect(sales.formError).toHaveText('保存失败。请检查必填内容、金额和时间；若数据已变更，请关闭后刷新重新操作。输入内容已保留。');
      await expect(await sales.input('percentage')).toHaveValue('101'); await sales.screenshot('invalid-rule-preserves-input');
      await sales.button(/取\s*消/u).click();
      await sales.tab('佣金账户').click();
      await expect(sales.text('可结算').first()).toBeVisible(); await sales.screenshot('commission-accounts');
    } finally {
      execPgSQLStatements([`DELETE FROM plugin_idc_core_customer_individual_profile WHERE customer_no IN('${customer}','${referrer}');`]);
      execPgSQLStatements(['BEGIN;', 'SET LOCAL session_replication_role=replica;', `DELETE FROM plugin_idc_core_sales_agent_qualification_event WHERE qualification_no IN(SELECT qualification_no FROM plugin_idc_core_sales_agent_qualification WHERE customer_no='${customer}');`, `DELETE FROM plugin_idc_core_sales_agent_qualification WHERE customer_no='${customer}';`, `DELETE FROM plugin_idc_core_sales_agent_level WHERE level_no IN('${levelNo}','${secondLevel}');`, `DELETE FROM plugin_idc_core_sales_customer_referrer WHERE customer_no='${customer}';`, `DELETE FROM plugin_idc_core_sales_purchase_price WHERE product_no='${product}';`, `DELETE FROM plugin_idc_core_sales_commission_rule WHERE product_no='${product}';`, `DELETE FROM plugin_idc_core_catalog_price WHERE version_no='${version}';`, `DELETE FROM plugin_idc_core_catalog_billing_rule WHERE version_no='${version}';`, `DELETE FROM plugin_idc_core_catalog_product_version WHERE version_no='${version}';`, `DELETE FROM plugin_idc_core_catalog_product WHERE product_no='${product}';`, `DELETE FROM plugin_idc_core_catalog_category WHERE category_no='${category}';`, `DELETE FROM plugin_idc_core_customer_subject WHERE customer_no IN('${customer}','${referrer}');`, 'COMMIT;']);
    }
  });
});
