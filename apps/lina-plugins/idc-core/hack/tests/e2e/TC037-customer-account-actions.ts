// Use isolated local customers and real HTTP mutations; audit failures must roll back rather than be hidden by a mock.
import { createHash, randomBytes } from 'node:crypto';
import { customerTest as test } from '../support/customer-admin-session';
import { expect } from '@host-tests/fixtures/auth';
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';
import { CustomerManagementPage } from '../pages/CustomerManagementPage';

test.describe('TC-37 客户账户真实操作', () => {
 test('TC-37a: 封禁、解封、实名通过和清空真实生效，保留记录且按钮对齐', async ({ adminPage }) => {
  const stamp=randomBytes(16).toString('hex');const number=`CUS_${stamp}`;const name=`客户操作验收${stamp.slice(0,8)}`;
  const digest=createHash('sha256').update(stamp).digest('hex');
  try {
   execPgSQLStatements([
    `INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${number}','individual','active','${name}','${name}');`,
    `INSERT INTO plugin_idc_core_customer_individual_profile(customer_no,identity_status_code) VALUES('${number}','unverified');`,
   ]);
   const view=new CustomerManagementPage(adminPage);await view.goto();await view.searchNamePrefix(name);await expect(view.customerRow(number)).toBeVisible();
   await view.assertToolbarAlignment();await view.screenshot('customer-actions-toolbar-real');
   await view.openOverview(number);
   await view.toggleAccount();await expect(view.overviewDrawer.getByTestId('customer-ban-toggle')).toHaveText(/解\s*封/u);
   expect(queryPgScalar(`SELECT status_code FROM plugin_idc_core_customer_subject WHERE customer_no='${number}'`)).toBe('disabled');
   await view.toggleAccount();await expect(view.overviewDrawer.getByTestId('customer-ban-toggle')).toHaveText(/封\s*禁/u);
   expect(queryPgScalar(`SELECT status_code FROM plugin_idc_core_customer_subject WHERE customer_no='${number}'`)).toBe('active');
   await view.updateVerification('approve',false);
   expect(queryPgScalar(`SELECT identity_status_code FROM plugin_idc_core_customer_individual_profile WHERE customer_no='${number}'`)).toBe('unverified');
   await view.updateVerification('approve');await expect(view.overviewDrawer.getByTestId('customer-verification-approve')).toBeDisabled();
   await expect(view.overviewDrawer).toContainText('已实名');
   expect(queryPgScalar(`SELECT identity_status_code || ':' || manual_verification::text FROM plugin_idc_core_customer_individual_profile WHERE customer_no='${number}'`)).toBe('verified:true');
   await view.screenshot('customer-identity-approved-real');
   execPgSQLStatements([`UPDATE plugin_idc_core_customer_individual_profile SET legal_name='Test Person',legal_name_masked='T***',identity_identifier_masked='110***********0019',identity_lookup_digest='${digest}',identity_case_no='case_old_${stamp.slice(0,16)}' WHERE customer_no='${number}';`]);
   await view.updateVerification('clear');await expect(view.overviewDrawer.getByTestId('customer-verification-approve')).toBeEnabled();
   expect(queryPgScalar(`SELECT (identity_status_code='unverified' AND legal_name='' AND legal_name_masked='' AND identity_identifier_masked='' AND btrim(identity_lookup_digest)='' AND identity_case_no='' AND identity_verified_at IS NULL AND identity_expires_at IS NULL)::text FROM plugin_idc_core_customer_individual_profile WHERE customer_no='${number}'`)).toBe('true');
   expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_customer_account_state_history WHERE customer_no='${number}'`)).toBe('2');
   expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_business_audit_event WHERE target_no='${number}'`)).toBe('4');
   await view.openDetailTab('操作记录');await expect(view.overviewDrawer).toContainText('修改实名');await view.screenshot('customer-identity-cleared-records-real');
  } finally {
   execPgSQLStatements(['BEGIN;','SET LOCAL session_replication_role=replica;',`DO $cleanup$ DECLARE item RECORD; BEGIN FOR item IN SELECT table_name FROM information_schema.columns WHERE table_schema='public' AND column_name='customer_no' AND table_name LIKE 'plugin_idc_core_%' LOOP EXECUTE format('DELETE FROM %I WHERE customer_no = %L',item.table_name,'${number}'); END LOOP; END $cleanup$;`,`DELETE FROM plugin_idc_core_business_audit_event WHERE target_no='${number}';`,'COMMIT;']);
  }
 });
});
