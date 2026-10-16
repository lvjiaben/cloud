// This file verifies immutable sales facts on the real migrated PostgreSQL
// schema, including delayed workers, manual referral history, zero settlements,
// duplicate events and refunds that retain the original payment ownership.

package commission

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

// TestSalesPaymentOwnershipPostgreSQL exercises the same owners used by accruals.
func TestSalesPaymentOwnershipPostgreSQL(t *testing.T) {
	for _, scenario := range []struct {
		name, manual, expected string
	}{
		{"manual_wins_at_payment", "ref_manual", "ref_manual"},
		{"manual_clear_blocks_link", "", ""},
		{"never_manual_uses_link", "absent", "ref_legacy"},
		{"future_manual_does_not_reassign", "future", "ref_legacy"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			database, _, _ := newCommissionDatabase(t)
			ctx := context.Background()
			seedCommissionRenewalFixture(t, ctx, database)
			seedPaymentReferralHistory(t, ctx, database, scenario.manual)
			runtime := newPaymentFactConsumer(t, database)
			for range 2 {
				if _, err := runtime.AccrueRenewals(ctx, 10); err != nil {
					t.Fatal(err)
				}
			}
			assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_fact WHERE invoice_no='inv_commission' AND operation_type_code='renewal' AND order_no IS NULL AND received_amount_minor=10000`, 1)
			assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_fact f JOIN plugin_idc_core_billing_invoice i USING(invoice_no) WHERE f.received_at=i.settled_at`, 1)
			assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_owner WHERE beneficiary_type_code='sales' AND beneficiary_reference='101' AND sales_assignment_no='sa_commission'`, 1)
			owners, err := database.GetAll(ctx, `SELECT beneficiary_reference,manual_referrer_id,affiliate_attribution_no FROM plugin_idc_core_sales_payment_owner WHERE beneficiary_type_code='affiliate'`)
			if err != nil {
				t.Fatal(err)
			}
			if scenario.expected == "" {
				if len(owners) != 0 {
					t.Fatalf("explicit cancellation fell back to link: %v", owners)
				}
				assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_accrual WHERE beneficiary_type_code='affiliate'`, 0)
			} else {
				if len(owners) != 1 || owners[0]["beneficiary_reference"].String() != scenario.expected {
					t.Fatalf("payment-time affiliate=%v want=%s", owners, scenario.expected)
				}
				if scenario.expected == "ref_manual" && owners[0]["manual_referrer_id"].Int64() <= 0 || scenario.expected == "ref_legacy" && owners[0]["affiliate_attribution_no"].String() != "attr_payment" {
					t.Fatalf("missing frozen ownership evidence: %v", owners)
				}
				assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_accrual c JOIN plugin_idc_core_sales_payment_owner o USING(invoice_no,beneficiary_type_code,beneficiary_reference) WHERE c.beneficiary_type_code='affiliate'`, 2)
			}
			// Current referral changes again before refund; the refund joins the
			// original invoice owners and does not depend on current relationships.
			if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_customer_referrer(customer_no,referrer_customer_no,version,created_by) SELECT 'commission_customer','ref_later',COALESCE(MAX(version),0)+1,101 FROM plugin_idc_core_sales_customer_referrer WHERE customer_no='commission_customer'`); err != nil {
				t.Fatal(err)
			}
			consumeCommissionRefundFixture(t, ctx, database, runtime, "refund_payment_owner", 1500, 1000)
			refunds, err := database.GetAll(ctx, `WITH refunded AS (
 SELECT invoice_no,SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END) AS amount
 FROM plugin_idc_core_billing_refund_allocation WHERE tenant_id=0 GROUP BY invoice_no
 ) SELECT o.beneficiary_reference,r.amount FROM plugin_idc_core_sales_payment_owner o JOIN refunded r USING(invoice_no) WHERE o.beneficiary_type_code='affiliate'`)
			if err != nil || scenario.expected == "" && len(refunds) != 0 || scenario.expected != "" && (len(refunds) != 1 || refunds[0]["beneficiary_reference"].String() != scenario.expected || refunds[0]["amount"].Int64() != 2500) {
				t.Fatalf("refund changed original attribution: rows=%v err=%v", refunds, err)
			}
			assertCommissionAmount(t, ctx, database, `SELECT SUM(received_amount_minor) FROM plugin_idc_core_sales_payment_fact`, 10000)
			assertCommissionExecFails(t, ctx, database, "payment facts are append-only", `UPDATE plugin_idc_core_sales_payment_fact SET received_amount_minor=1`)
			assertCommissionExecFails(t, ctx, database, "payment owners are append-only", `DELETE FROM plugin_idc_core_sales_payment_owner`)
		})
	}
}

// seedPaymentReferralHistory creates histories before and after the settled_at
// instant. Replica mode is limited to fixtures for unrelated fulfillment FKs.
func seedPaymentReferralHistory(t *testing.T, ctx context.Context, database gdb.DB, manual string) {
	t.Helper()
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := database.Exec(ctx, `SET LOCAL session_replication_role=replica;
 INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name) VALUES
 ('ref_legacy','individual','active','Legacy referral'),('ref_manual','individual','active','Manual referral'),('ref_later','individual','active','Later referral');
 INSERT INTO plugin_idc_core_sales_affiliate_attribution(attribution_no,link_no,touch_no,referrer_customer_no,referred_customer_no,policy_code,evidence_digest,attributed_at,expires_at)
 VALUES('attr_payment','link_payment','touch_payment','ref_legacy','commission_customer','first_touch',repeat('5',64),CURRENT_TIMESTAMP-INTERVAL '3 days',CURRENT_TIMESTAMP+INTERVAL '1 day');
 INSERT INTO plugin_idc_core_sales_commission_rule(rule_no,beneficiary_type_code,operation_type_code,currency_code,calculation_type_code,rate_basis_points,maturity_days,status_code,effective_at,version,rule_digest)
 VALUES('rule_payment_affiliate','affiliate','renewal','CNY','rate',1000,0,'published',CURRENT_TIMESTAMP-INTERVAL '2 days',1,repeat('6',64));
 UPDATE plugin_idc_core_sales_customer_assignment SET status_code='ended',ended_at=CURRENT_TIMESTAMP WHERE assignment_no='sa_commission';
 INSERT INTO plugin_idc_core_sales_group_member(membership_no,group_no,staff_user_id,effective_at) VALUES('sm_later','sg_commission',202,CURRENT_TIMESTAMP);
 INSERT INTO plugin_idc_core_sales_customer_assignment(assignment_no,customer_no,membership_no,source_code,effective_at) VALUES('sa_later','commission_customer','sm_later','manual',CURRENT_TIMESTAMP);`); err != nil {
			return err
		}
		if manual == "absent" {
			return nil
		}
		if manual == "future" {
			_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_customer_referrer(customer_no,referrer_customer_no,version,created_by) VALUES('commission_customer','ref_later',1,101)`)
			return err
		}
		var value any
		if manual != "" {
			value = manual
		}
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_customer_referrer(customer_no,referrer_customer_no,version,effective_at,created_by) VALUES('commission_customer',?,1,CURRENT_TIMESTAMP-INTERVAL '2 days',101),('commission_customer','ref_later',2,CURRENT_TIMESTAMP,101)`, value)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// newPaymentFactConsumer uses the production commission dependencies only.
func newPaymentFactConsumer(t *testing.T, database gdb.DB) *consumer {
	t.Helper()
	rules, err := saleschannel.NewWithDatabase(database)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewConsumer(database, rules)
	if err != nil {
		t.Fatal(err)
	}
	return runtime.(*consumer)
}

// TestSalesPaymentZeroAndHistoricalReceiptPostgreSQL distinguishes lawful zero
// renewal facts from old receipts whose missing historical owners must not be guessed.
func TestSalesPaymentZeroAndHistoricalReceiptPostgreSQL(t *testing.T) {
	database, _, _ := newCommissionDatabase(t)
	ctx := context.Background()
	seedCommissionRenewalFixture(t, ctx, database)
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, err := database.Exec(ctx, `SET LOCAL session_replication_role=replica;
 INSERT INTO plugin_idc_core_billing_invoice(invoice_no,customer_no,source_code,currency_code,status_code,item_count,subtotal_minor,net_amount_minor,total_amount_minor,paid_amount_minor,refundable_amount_minor,source_snapshot_digest,content_digest,payment_due_at,issued_at,settled_at)
 VALUES('inv_zero_renewal','commission_customer','renewal','USD','paid',1,0,0,0,0,0,repeat('a',64),repeat('b',64),CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP-INTERVAL '1 hour',CURRENT_TIMESTAMP-INTERVAL '1 minute');
 INSERT INTO plugin_idc_core_commission_order_receipt(invoice_no,settlement_no,source_digest) VALUES('inv_commission','historical_receipt',repeat('d',64));`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	runtime := newPaymentFactConsumer(t, database)
	for range 2 {
		if _, err := runtime.AccrueRenewals(ctx, 10); err != nil {
			t.Fatal(err)
		}
	}
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_fact WHERE invoice_no='inv_zero_renewal' AND operation_type_code='renewal' AND currency_code='USD' AND received_amount_minor=0`, 1)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_owner WHERE invoice_no='inv_zero_renewal' AND beneficiary_reference='101'`, 1)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_fact WHERE invoice_no='inv_commission'`, 0)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_accrual`, 0)
}

// TestSalesPaymentOwnerFailureRollsBackPostgreSQL proves receipts cannot hide a
// partially written payment: an owner failure rolls back facts and commissions.
func TestSalesPaymentOwnerFailureRollsBackPostgreSQL(t *testing.T) {
	database, _, _ := newCommissionDatabase(t)
	ctx := context.Background()
	seedCommissionRenewalFixture(t, ctx, database)
	if _, err := database.Exec(ctx, `CREATE FUNCTION reject_payment_owner_fixture() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected owner persistence failure'; END $$;
 CREATE TRIGGER reject_payment_owner_fixture BEFORE INSERT ON plugin_idc_core_sales_payment_owner FOR EACH ROW EXECUTE FUNCTION reject_payment_owner_fixture()`); err != nil {
		t.Fatal(err)
	}
	runtime := newPaymentFactConsumer(t, database)
	if _, err := runtime.AccrueRenewals(ctx, 10); err == nil {
		t.Fatal("owner persistence failure accepted")
	}
	for _, table := range []string{"plugin_idc_core_sales_payment_fact", "plugin_idc_core_sales_payment_owner", "plugin_idc_core_commission_order_receipt", "plugin_idc_core_commission_accrual"} {
		assertCommissionAmount(t, ctx, database, "SELECT COUNT(*) FROM "+table, 0)
	}
	if _, err := database.Exec(ctx, `DROP TRIGGER reject_payment_owner_fixture ON plugin_idc_core_sales_payment_owner`); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.AccrueRenewals(ctx, 10); err != nil {
		t.Fatal(err)
	}
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_sales_payment_fact`, 1)
	assertCommissionAmount(t, ctx, database, `SELECT COUNT(*) FROM plugin_idc_core_commission_order_receipt`, 1)
}
