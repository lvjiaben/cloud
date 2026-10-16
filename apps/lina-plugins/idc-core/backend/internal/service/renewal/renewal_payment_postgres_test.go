// This file verifies PostgreSQL payment and convergence SQL projections and
// rejection gates using isolated temporary tables. TC010 covers real persisted
// billing; this matrix does not claim to validate the full financial chain.

package renewal

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
)

func TestRenewalPaymentRequiresExactFrozenContractPostgreSQL(t *testing.T) {
	ctx, database := newRenewalPostgreSQLDatabase(t)
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// Connection-local projection fixtures isolate the contract comparison from
		// unrelated financial fixture setup. TC010 exercises real persisted billing.
		_, err := database.Exec(ctx, `
CREATE TEMP TABLE plugin_idc_core_business_operation ON COMMIT DROP AS SELECT
 0 AS tenant_id,'op'::text AS operation_no,'quote'::text AS quote_no,'customer'::text AS customer_no,
 'customer'::text AS payer_customer_no,'service'::text AS subject_no,'CNY'::text AS currency_code,
 'invoice'::text AS invoice_no,'price'::text AS price_snapshot_no,'awaiting_financial_settlement'::text AS status_code,
 'local_entitlement'::text AS execution_mode_code,'provider'::text AS capability_evidence_no,
 repeat('a',64) AS capability_evidence_digest,repeat('b',64) AS content_digest,
 1::bigint AS expected_subject_version,'renewal'::text AS operation_type_code,
 'owner'::text AS resource_owner_code,'1.0.0'::text AS channel_no,'v1'::text AS pool_no,1::bigint AS version;
CREATE TEMP TABLE plugin_idc_core_service_entitlement ON COMMIT DROP AS SELECT
 0 AS tenant_id,'service'::text AS entitlement_no,'customer'::text AS customer_no,'product'::text AS product_no,1::bigint AS version,
 'local_entitlement'::text AS renewal_mode_code,'active'::text AS status_code,'recurring'::text AS charge_type_code,
 'provider'::text AS plugin_id,'1.0.0'::text AS plugin_release_version,'v1'::text AS contract_version,
 'owner'::text AS entitlement_owner_id,''::text AS renewal_method,repeat('a',64) AS declaration_digest,
 CURRENT_TIMESTAMP+INTERVAL '1 day' AS expires_at;
CREATE TEMP TABLE plugin_idc_core_billing_invoice ON COMMIT DROP AS SELECT
 0 AS tenant_id,'invoice'::text AS invoice_no,'customer'::text AS customer_no,'CNY'::text AS currency_code,
 CURRENT_TIMESTAMP+INTERVAL '1 hour' AS payment_due_at,'paid'::text AS status_code,
 100::bigint AS total_amount_minor,100::bigint AS paid_amount_minor,0::bigint AS credit_amount_minor;
CREATE TEMP TABLE plugin_idc_core_service_price_snapshot ON COMMIT DROP AS SELECT
 0 AS tenant_id,'price'::text AS price_snapshot_no,'service'::text AS subject_no,
 CURRENT_TIMESTAMP+INTERVAL '31 days' AS projected_expires_at;
CREATE TEMP TABLE plugin_idc_core_business_operation_event ON COMMIT DROP AS SELECT
 'op'::text AS operation_no,1 AS event_sequence;
CREATE TEMP TABLE plugin_idc_core_billing_payment_allocation ON COMMIT DROP AS SELECT
 0 AS tenant_id,'invoice'::text AS invoice_no,'transaction'::text AS ledger_transaction_no,
 'balance'::text AS payment_source_type_code,'apply'::text AS direction_code;
CREATE TEMP TABLE plugin_idc_core_credit_exposure ON COMMIT DROP AS SELECT
 0 AS tenant_id,'exposure'::text AS exposure_no,'customer'::text AS customer_no,'CNY'::text AS currency_code,
 'renewal'::text AS source_type_code,'op'::text AS source_reference,'invoice'::text AS invoice_no;`)
		if err != nil {
			return err
		}
		for _, mode := range []string{"local_entitlement", "provider_confirmed"} {
			method := ""
			evidence := "provider"
			if mode == "provider_confirmed" {
				method, evidence = "renew", "renew"
			}
			if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET execution_mode_code=?,capability_evidence_no=?`, mode, evidence); err != nil {
				return err
			}
			for _, changed := range []string{"", "owner", "release", "contract", "method", "digest"} {
				owner, release, contract, currentMethod, digest := "owner", "1.0.0", "v1", method, strings.Repeat("a", 64)
				switch changed {
				case "owner":
					owner = "other"
				case "release":
					release = "2.0.0"
				case "contract":
					contract = "v2"
				case "method":
					currentMethod = "other"
				case "digest":
					digest = strings.Repeat("c", 64)
				}
				if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET renewal_mode_code=?,entitlement_owner_id=?,plugin_release_version=?,contract_version=?,renewal_method=?,declaration_digest=?`, mode, owner, release, contract, currentMethod, digest); err != nil {
					return err
				}
				for _, stage := range []string{"pending_payment", "paid", "settled_replay", "balance_convergence", "credit_convergence"} {
					caseName := changed
					if caseName == "" {
						caseName = "matching"
					}
					t.Run(mode+"/"+stage+"/"+caseName, func(t *testing.T) {
						assertRenewalPaymentContractStage(t, ctx, database, stage, evidence, changed != "")
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// assertRenewalPaymentContractStage checks successful SQL projections without
// mutating money, and calls the real convergence gate for every mismatch.
func assertRenewalPaymentContractStage(t *testing.T, ctx context.Context, database gdb.DB, stage, evidence string, mismatch bool) {
	t.Helper()
	status, paid, credit := "paid", 100, 0
	if stage == "pending_payment" {
		status, paid = "pending_payment", 0
	} else if stage == "credit_convergence" {
		status, paid, credit = "credit_settled", 0, 100
	}
	operationStatus := "awaiting_financial_settlement"
	if stage == "settled_replay" {
		operationStatus = "succeeded"
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code=?`, operationStatus); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_billing_invoice SET status_code=?,paid_amount_minor=?,credit_amount_minor=?`, status, paid, credit); err != nil {
		t.Fatal(err)
	}
	if stage == "pending_payment" || stage == "paid" || stage == "settled_replay" {
		row, err := loadPaymentPreflight(ctx, database, "customer", "customer", "op")
		if err != nil || row == nil {
			t.Fatalf("load payment projection: row=%+v err=%v", row, err)
		}
		validation := validatePaymentPreflight(ctx, database, row)
		if mismatch && !bizerr.Is(validation, CodeConflict) || !mismatch && validation != nil {
			t.Fatalf("payment eligibility mismatch=%t err=%v", mismatch, validation)
		}
		return
	}
	var (
		row *convergenceRow
		err error
	)
	if stage == "credit_convergence" {
		row, err = lockCreditConvergenceRow(ctx, database, "customer", "customer", "op", "exposure")
	} else {
		row, err = lockConvergenceRow(ctx, database, "customer", "customer", "op", "transaction")
	}
	if err != nil || row == nil {
		t.Fatalf("load convergence projection: row=%+v err=%v", row, err)
	}
	if !mismatch {
		if row.EntitlementContractNo == nil || *row.EntitlementContractNo != evidence || row.EntitlementContractDigest == nil || *row.EntitlementContractDigest != strings.Repeat("a", 64) {
			t.Fatalf("matching frozen convergence contract lost: %+v", row)
		}
		return
	}
	service := &serviceImpl{database: database}
	if stage == "credit_convergence" {
		_, _, err = service.convergeCreditOperation(ctx, "customer", "customer", "op", "exposure", "channel")
	} else {
		_, _, err = service.convergePaidOperation(ctx, "customer", "customer", "op", "transaction", "customer", "customer")
	}
	if !bizerr.Is(err, CodeConflict) {
		t.Fatalf("frozen contract mismatch reached convergence: %v", err)
	}
	operation, err := database.GetOne(ctx, `SELECT status_code,version FROM plugin_idc_core_business_operation WHERE operation_no='op'`)
	if err != nil || operation["status_code"].String() != "awaiting_financial_settlement" || operation["version"].Int64() != 1 {
		t.Fatalf("rejected convergence changed operation: %v err=%v", operation, err)
	}
}
