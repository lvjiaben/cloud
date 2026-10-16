// Isolated historical commission fixtures for real HTTP/UI settlement tests.
// Trigger suppression is limited to importing historical invoice evidence and
// exact cleanup; commission projection and payout writes use normal triggers.
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';

export class SalesPayoutFixture {
  readonly customer: string;
  readonly name: string;
  constructor(readonly suffix: string) { this.customer = `CUS_${suffix}`; this.name = `结算验收${suffix.slice(0, 8)}`; }
  account(currency: string) { return `CA${currency}_${this.suffix}`; }
  commission(currency: string) { return `CM${currency}_${this.suffix}`; }
  balance(currency: string) { return `BA${currency}_${this.suffix}`; }
  seed() {
    execPgSQLStatements(['BEGIN;',
      `INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${this.customer}','individual','active','${this.name}','${this.name}');`,
      `INSERT INTO plugin_idc_core_customer_individual_profile(customer_no,identity_status_code) VALUES('${this.customer}','unverified');`,
      ...['CNY','USD'].flatMap(currency => this.seedCurrency(currency)), 'COMMIT;']);
  }
  private seedCurrency(currency: string) {
    const account = this.account(currency), commission = this.commission(currency);
    const invoice = `IN${currency}_${this.suffix}`, item = `IT${currency}_${this.suffix}`;
    const balance = this.balance(currency), cash = `CA${currency}_${this.suffix}`, tx = `TX${currency}_${this.suffix}`;
    return [
      'SET LOCAL session_replication_role=replica;',
      `INSERT INTO plugin_idc_core_billing_invoice(invoice_no,customer_no,source_code,currency_code,status_code,item_count,subtotal_minor,net_amount_minor,total_amount_minor,paid_amount_minor,refundable_amount_minor,source_snapshot_digest,content_digest,payment_due_at,issued_at,settled_at) VALUES('${invoice}','${this.customer}','migration','${currency}','paid',1,10000,10000,10000,10000,10000,repeat('a',64),repeat('b',64),CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP-INTERVAL '2 day',CURRENT_TIMESTAMP-INTERVAL '1 day');`,
      `INSERT INTO plugin_idc_core_billing_invoice_item(invoice_item_no,invoice_no,customer_no,currency_code,line_number,item_type_code,source_reference,description_zh_cn,description_en_us,subtotal_minor,net_amount_minor,total_amount_minor,refundable_cap_minor,content_digest) VALUES('${item}','${invoice}','${this.customer}','${currency}',1,'manual','${invoice}','历史实收验收','Historical receipt acceptance',10000,10000,10000,10000,repeat('a',64));`,
      `INSERT INTO plugin_idc_core_commission_accrual(commission_no,invoice_no,invoice_item_no,beneficiary_type_code,beneficiary_reference,currency_code,rule_version,basis_amount_minor,invoice_item_amount_minor,original_amount_minor,pending_amount_minor,matured_amount_minor,reversed_amount_minor,status_code,matures_at) VALUES('${commission}','${invoice}','${item}','affiliate','${this.customer}','${currency}',1,10000,10000,1000,0,1000,0,'matured',CURRENT_TIMESTAMP-INTERVAL '1 day');`,
      'SET LOCAL session_replication_role=origin;',
      `INSERT INTO plugin_idc_core_sales_commission_account(account_no,beneficiary_type_code,beneficiary_reference,currency_code) VALUES('${account}','affiliate','${this.customer}','${currency}');`,
      `INSERT INTO plugin_idc_core_sales_commission_account_entry(entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor) VALUES('EA${currency}_${this.suffix}','${account}','${currency}',1,'accrual','e2e_migration','${commission}',repeat('a',64),1000,1000,0,0,0,0,1000,1000,0,0,0,0);`,
      `INSERT INTO plugin_idc_core_sales_commission_account_entry(entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor) VALUES('EM${currency}_${this.suffix}','${account}','${currency}',2,'maturity','e2e_maturity','${commission}',repeat('b',64),0,-1000,1000,0,0,0,1000,0,1000,0,0,0);`,
      `INSERT INTO plugin_idc_core_billing_ledger_account(account_no,owner_type_code,owner_reference,customer_no,account_type_code,normal_balance_code,currency_code) VALUES('${balance}','customer','${this.customer}','${this.customer}','customer_balance','credit','${currency}'),('${cash}','system','e2e_${this.suffix}',NULL,'merchant_cash','debit','${currency}');`,
      `INSERT INTO plugin_idc_core_billing_ledger_transaction(transaction_no,transaction_type_code,currency_code,source_type_code,source_reference,source_snapshot_digest,entry_count,debit_total_minor,credit_total_minor,content_digest) VALUES('${tx}','recharge','${currency}','e2e_recharge','${tx}',repeat('a',64),2,12345,12345,repeat('b',64));`,
      `INSERT INTO plugin_idc_core_billing_ledger_entry(entry_no,transaction_no,currency_code,line_number,account_no,direction_code,entry_role_code,amount_minor,reason_code,content_digest) VALUES('EB${currency}_${this.suffix}','${tx}','${currency}',1,'${balance}','credit','balance',12345,'e2e_recharge',repeat('a',64)),('EC${currency}_${this.suffix}','${tx}','${currency}',2,'${cash}','debit','cash',12345,'e2e_recharge',repeat('b',64));`,
      `INSERT INTO plugin_idc_core_billing_balance_snapshot(snapshot_no,account_no,currency_code,normal_balance_code,transaction_no,sequence_no,opening_balance_minor,debit_amount_minor,credit_amount_minor,closing_balance_minor,content_digest) VALUES('SB${currency}_${this.suffix}','${balance}','${currency}','credit','${tx}',1,0,0,12345,12345,repeat('a',64)),('SC${currency}_${this.suffix}','${cash}','${currency}','debit','${tx}',1,0,12345,0,12345,repeat('b',64));`,
    ];
  }
  addRecovery() {
    const commission = this.commission('CNY'), reversal = `REV_${this.suffix}`, refund = `REF_${this.suffix}`;
    execPgSQLStatements(['BEGIN;',
      `INSERT INTO plugin_idc_core_commission_refund_reversal(reversal_no,commission_no,refund_no,source_event_no,amount_minor) VALUES('${reversal}','${commission}','${refund}','EVT_${this.suffix}',100);`,
      `INSERT INTO plugin_idc_core_sales_commission_recovery(recovery_no,account_no,commission_no,refund_no,reversal_no,amount_minor) VALUES('REC_${this.suffix}','${this.account('CNY')}','${commission}','${refund}','${reversal}',100);`,
      'COMMIT;']);
  }
  addPending() {
    const account = this.account('USD'), commission = `PEN_${this.suffix}`;
    execPgSQLStatements(['BEGIN;',
      `INSERT INTO plugin_idc_core_commission_accrual(commission_no,invoice_no,invoice_item_no,beneficiary_type_code,beneficiary_reference,currency_code,rule_version,basis_amount_minor,invoice_item_amount_minor,original_amount_minor,pending_amount_minor,matured_amount_minor,reversed_amount_minor,status_code,matures_at) VALUES('${commission}','INUSD_${this.suffix}','ITUSD_${this.suffix}','affiliate','${this.customer}','USD',1,10000,10000,200,200,0,0,'pending_maturity',CURRENT_TIMESTAMP+INTERVAL '1 day');`,
      `INSERT INTO plugin_idc_core_sales_commission_account_entry(entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor) VALUES('EP_${this.suffix}','${account}','USD',3,'accrual','e2e_pending','${commission}',repeat('a',64),200,200,0,0,0,0,1200,200,1000,0,0,0);`,
      'COMMIT;']);
  }
  accountState(currency: string) {
    return queryPgScalar(`SELECT credited_amount_minor||','||pending_amount_minor||','||available_amount_minor||','||paid_amount_minor||','||version FROM plugin_idc_core_sales_commission_account WHERE account_no='${this.account(currency)}'`);
  }
  spendingState() {
    return queryPgScalar(`SELECT string_agg(a.currency_code||':'||s.sequence_no||':'||s.closing_balance_minor,',' ORDER BY a.currency_code,s.sequence_no) FROM plugin_idc_core_billing_ledger_account a JOIN plugin_idc_core_billing_balance_snapshot s ON s.account_no=a.account_no WHERE a.customer_no='${this.customer}'`);
  }
  payoutCount() { return queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_sales_commission_offline_payout WHERE account_no IN('${this.account('CNY')}','${this.account('USD')}')`); }
  cleanup() {
    const accounts = `'${this.account('CNY')}','${this.account('USD')}'`;
    execPgSQLStatements(['BEGIN;','SET LOCAL session_replication_role=replica;',
      `DELETE FROM plugin_idc_core_sales_commission_offline_payout_item WHERE commission_no IN('${this.commission('CNY')}','${this.commission('USD')}');`,
      ...['sales_commission_offline_payout','sales_commission_recovery_offset','sales_commission_recovery','sales_commission_account_entry'].map(table => `DELETE FROM plugin_idc_core_${table} WHERE account_no IN(${accounts});`),
      `DELETE FROM plugin_idc_core_commission_refund_reversal WHERE commission_no IN('${this.commission('CNY')}','${this.commission('USD')}');`,
      `DELETE FROM plugin_idc_core_commission_accrual WHERE beneficiary_reference='${this.customer}';`,
      `DELETE FROM plugin_idc_core_sales_commission_account WHERE account_no IN(${accounts});`,
      ...['billing_balance_snapshot','billing_ledger_entry'].map(table=>`DELETE FROM plugin_idc_core_${table} WHERE account_no IN('${this.balance('CNY')}','${this.balance('USD')}',${accounts});`),
      `DELETE FROM plugin_idc_core_billing_ledger_transaction WHERE transaction_no IN('TXCNY_${this.suffix}','TXUSD_${this.suffix}');`,
      `DELETE FROM plugin_idc_core_billing_ledger_account WHERE account_no IN('${this.balance('CNY')}','${this.balance('USD')}',${accounts});`,
      `DELETE FROM plugin_idc_core_billing_invoice_item WHERE customer_no='${this.customer}';`,
      `DELETE FROM plugin_idc_core_billing_invoice WHERE customer_no='${this.customer}';`,
      `DELETE FROM plugin_idc_core_customer_individual_profile WHERE customer_no='${this.customer}';`,
      `DELETE FROM plugin_idc_core_customer_subject WHERE customer_no='${this.customer}';`,'COMMIT;']);
  }
}
