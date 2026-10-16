// Isolated imported historical payment facts exercise real performance queries.
// These are not commission rows. Replica mode is session-local for historical
// ledger import and exact cleanup; browser requests use normal runtime checks.
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';

export class SalesPerformanceFixture {
  readonly payer: string; readonly referrer: string; readonly replacement: string; readonly name: string;
  constructor(readonly suffix: string) {
    this.payer = `PAY_${suffix}`; this.referrer = `REF_${suffix}`; this.replacement = `NEW_${suffix}`; this.name = `业绩验收${suffix.slice(0, 8)}`;
  }
  seed() {
    const rows: Array<[string,string,number,boolean]> = [['paid','CNY',10000,false],['free','CNY',0,false],['renew','CNY',2000,true],['usd','USD',700,false]];
    execPgSQLStatements(['BEGIN;',
      ...[[this.payer,`付款${this.suffix.slice(0,8)}`],[this.referrer,this.name],[this.replacement,`新归属${this.suffix.slice(0,8)}`]].map(([no,name])=>`INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${no}','individual','active','${name}','${name}');`),
      `INSERT INTO plugin_idc_core_sales_customer_referrer(customer_no,referrer_customer_no,version,effective_at,created_by) VALUES('${this.payer}','${this.referrer}',1,'2026-08-01T00:00:00Z',1);`,
      'SET LOCAL session_replication_role=replica;',
      ...rows.flatMap(([kind,currency,amount,renewal])=> {
        const inv=`I${kind}_${this.suffix}`, order=`O${kind}_${this.suffix}`, item=`T${kind}_${this.suffix}`;
        const orderSQL=renewal?'NULL':`'${order}'`;
        return [
          ...(renewal?[]:[`INSERT INTO plugin_idc_core_commerce_order(order_no,customer_no,quote_no,currency_code,source_code,status_code,risk_decision_code,item_count,subtotal_minor,net_amount_minor,total_amount_minor,checkout_idempotency_digest,checkout_request_digest,payment_due_at,paid_at) VALUES('${order}','${this.payer}','Q${kind}_${this.suffix}','${currency}','migration','paid','approved',1,${amount},${amount},${amount},md5('${order}')||md5('${order}'),md5('${order}')||md5('${order}'),CURRENT_TIMESTAMP+INTERVAL '1 day','2026-09-10');`]),
          `INSERT INTO plugin_idc_core_billing_invoice(invoice_no,customer_no,order_no,source_code,currency_code,status_code,item_count,subtotal_minor,net_amount_minor,total_amount_minor,paid_amount_minor,refundable_amount_minor,source_snapshot_digest,content_digest,payment_due_at,issued_at,settled_at) VALUES('${inv}','${this.payer}',${orderSQL},'migration','${currency}','paid',1,${amount},${amount},${amount},${amount},${amount},repeat('a',64),repeat('b',64),CURRENT_TIMESTAMP+INTERVAL '1 day','2026-09-09','2026-09-10');`,
          `INSERT INTO plugin_idc_core_billing_invoice_item(invoice_item_no,invoice_no,customer_no,currency_code,line_number,item_type_code,source_reference,description_zh_cn,description_en_us,subtotal_minor,net_amount_minor,total_amount_minor,refundable_cap_minor,content_digest) VALUES('${item}','${inv}','${this.payer}','${currency}',1,'manual','${inv}','历史业绩验收','Historical performance fixture',${amount},${amount},${amount},${amount},repeat('a',64));`,
          `INSERT INTO plugin_idc_core_sales_payment_fact(invoice_no,order_no,customer_no,currency_code,operation_type_code,received_amount_minor,received_at,settlement_no,source_digest) VALUES('${inv}',${orderSQL},'${this.payer}','${currency}','${renewal?'renewal':'new_purchase'}',${amount},'2026-09-10','S${kind}_${this.suffix}',repeat('a',64));`,
          `INSERT INTO plugin_idc_core_sales_payment_owner(invoice_no,beneficiary_type_code,beneficiary_reference,manual_referrer_id) SELECT '${inv}','affiliate','${this.referrer}',id FROM plugin_idc_core_sales_customer_referrer WHERE customer_no='${this.payer}' AND version=1;`,
        ];
      }),
      `INSERT INTO plugin_idc_core_billing_refund_allocation(refund_allocation_no,invoice_no,invoice_item_no,customer_no,currency_code,payment_allocation_no,refund_source_type_code,refund_source_reference,ledger_transaction_no,ledger_transaction_type_code,direction_code,amount_minor,content_digest,created_at) VALUES('R_${this.suffix}','Ipaid_${this.suffix}','Tpaid_${this.suffix}','${this.payer}','CNY','A_${this.suffix}','migration','historical-performance','L_${this.suffix}','migration','apply',3000,repeat('a',64),'2026-10-01');`,
      'SET LOCAL session_replication_role=origin;',
      // A later reassignment must not move the September facts to the new owner.
      `INSERT INTO plugin_idc_core_sales_customer_referrer(customer_no,referrer_customer_no,version,effective_at,created_by) VALUES('${this.payer}','${this.replacement}',2,'2026-10-02T00:00:00Z',1);`,
      'COMMIT;',
    ]);
  }
  commissionCount() { return queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_commission_accrual WHERE beneficiary_reference='${this.referrer}'`); }
  cleanup() {
    execPgSQLStatements(['BEGIN;','SET LOCAL session_replication_role=replica;',
      `DELETE FROM plugin_idc_core_sales_payment_owner WHERE beneficiary_reference='${this.referrer}';`,
      ...['sales_payment_fact','billing_refund_allocation','billing_invoice_item','billing_invoice','commerce_order','sales_customer_referrer'].map(table=>`DELETE FROM plugin_idc_core_${table} WHERE customer_no='${this.payer}';`),
      `DELETE FROM plugin_idc_core_customer_subject WHERE customer_no IN('${this.payer}','${this.referrer}','${this.replacement}');`,'COMMIT;']);
  }
}
