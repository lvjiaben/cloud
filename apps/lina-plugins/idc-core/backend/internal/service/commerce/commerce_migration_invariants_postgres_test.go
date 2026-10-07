// This file verifies generic refund, historical subject, and trial-unit
// invariants introduced by migrations 085, 087, and 088 on PostgreSQL.
package commerce

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/dialect"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func executeCommerceMigrationStatements(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	name string,
	content string,
) {
	t.Helper()
	statements := dialect.SplitSQLStatements(content)
	if len(statements) == 0 {
		t.Fatalf("migration %s has no executable statements", name)
	}
	for index, statement := range statements {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatalf("execute migration %s statement %d: %v", name, index+1, err)
		}
	}
}

func TestRefundCompositionUsesOneDeferredValidationPerRefundPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioOptionsWithQuantity(t, true, true, 2)
	scenario := prepared.checkout
	const reviewerID = 9011
	management, err := billingservice.NewManagementRefundService(
		scenario.database, refundManagementAuthorizerStub{actor: reviewerID},
		&refundRecentAuthStub{actor: reviewerID}, refundBusinessAuditStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = management.ConfigureRefundPolicy(scenario.ctx, billingservice.RefundPolicyInput{
		CurrencyCode: "CNY", HighAmountThresholdMinor: prepared.order.TotalAmountMinor + 1,
		AutomaticApprovalEnabled: true,
	}, 0); err != nil {
		t.Fatal(err)
	}

	var intent struct {
		IntentNo string `orm:"intent_no"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &intent, `SELECT intent_no
FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=? LIMIT 1`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`ALTER TABLE plugin_idc_core_product_fulfillment_intent DISABLE TRIGGER trg_idc_guard_fulfillment_intent`,
		`ALTER TABLE plugin_idc_core_product_fulfillment_intent DISABLE TRIGGER trg_idc_check_fulfillment_intent_terminal`,
		`UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='completed' WHERE intent_no='` + intent.IntentNo + `'`,
		`ALTER TABLE plugin_idc_core_product_fulfillment_intent ENABLE TRIGGER trg_idc_guard_fulfillment_intent`,
		`ALTER TABLE plugin_idc_core_product_fulfillment_intent ENABLE TRIGGER trg_idc_check_fulfillment_intent_terminal`,
	} {
		if _, err = scenario.database.Exec(scenario.ctx, statement); err != nil {
			t.Fatalf("complete generic fulfillment: %v", err)
		}
	}

	var target struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		EntitlementNo string `orm:"entitlement_no"`
		AmountMinor   int64  `orm:"amount_minor"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &target, `SELECT invoice_item.invoice_item_no,
entitlement.entitlement_no,invoice_item.total_amount_minor AS amount_minor
FROM plugin_idc_core_billing_invoice_item invoice_item
JOIN plugin_idc_core_service_entitlement entitlement
  ON entitlement.order_item_no=invoice_item.order_item_no
WHERE invoice_item.invoice_no=?`, prepared.order.InvoiceNo); err != nil {
		t.Fatal(err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
CREATE TABLE test_refund_validation_probe(refund_no VARCHAR(40) PRIMARY KEY,runs INT NOT NULL DEFAULT 0);
CREATE OR REPLACE FUNCTION test_record_refund_validation() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO test_refund_validation_probe(refund_no,runs) VALUES(OLD.refund_no,1)
    ON CONFLICT(refund_no) DO UPDATE SET runs=test_refund_validation_probe.runs+1;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER test_refund_validation_probe
AFTER DELETE ON plugin_idc_core_refund_composition_validation_queue
FOR EACH ROW EXECUTE FUNCTION test_record_refund_validation();`); err != nil {
		t.Fatalf("install refund validation probe: %v", err)
	}
	refunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	input := billingservice.RefundPreviewInput{
		OrderNo: prepared.order.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: target.InvoiceItemNo, EntitlementNos: []string{target.EntitlementNo},
			Quantity: 2, AmountMinor: target.AmountMinor,
		}},
	}
	preview, err := refunds.PreviewOnlineRefund(scenario.ctx, input)
	if err != nil {
		t.Fatalf("preview quantity two with one entitlement: %v", err)
	}
	refund, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest: preview.ImpactDigest, Reason: billingservice.RefundReasonNoLongerNeeded,
		IdempotencyKey: "refund-single-deferred-validation-0001", Preview: input,
	})
	if err != nil || refund == nil {
		t.Fatalf("submit quantity two with one entitlement: refund=%+v err=%v", refund, err)
	}
	var evidence struct {
		Runs             int `orm:"runs"`
		Queued           int `orm:"queued"`
		SourceTriggers   int `orm:"source_triggers"`
		DeferredTriggers int `orm:"deferred_triggers"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT
COALESCE((SELECT runs FROM test_refund_validation_probe WHERE refund_no=?),0) AS runs,
(SELECT COUNT(*) FROM plugin_idc_core_refund_composition_validation_queue WHERE refund_no=?) AS queued,
(SELECT COUNT(*) FROM pg_trigger trigger_row
 JOIN pg_proc function_row ON function_row.oid=trigger_row.tgfoid
 JOIN pg_class relation_row ON relation_row.oid=trigger_row.tgrelid
 JOIN pg_namespace namespace_row ON namespace_row.oid=relation_row.relnamespace
 WHERE namespace_row.nspname=current_schema() AND NOT trigger_row.tgisinternal AND trigger_row.tgname IN(
 'trg_idc_refund_request_composition','trg_idc_refund_item_composition',
 'trg_idc_refund_source_composition','trg_idc_refund_impact_composition','trg_idc_refund_lock_composition')
 AND function_row.proname='plugin_idc_core_enqueue_refund_composition_validation') AS source_triggers,
(SELECT COUNT(*) FROM pg_trigger trigger_row
 JOIN pg_proc function_row ON function_row.oid=trigger_row.tgfoid
 JOIN pg_class relation_row ON relation_row.oid=trigger_row.tgrelid
 JOIN pg_namespace namespace_row ON namespace_row.oid=relation_row.relnamespace
 WHERE namespace_row.nspname=current_schema() AND NOT trigger_row.tgisinternal
 AND trigger_row.tgname='trg_idc_refund_validation_queue_composition'
 AND trigger_row.tgdeferrable AND trigger_row.tginitdeferred
 AND function_row.proname='plugin_idc_core_run_queued_refund_composition_validation') AS deferred_triggers`,
		refund.RefundNo, refund.RefundNo); err != nil {
		t.Fatal(err)
	}
	if evidence.Runs != 1 || evidence.Queued != 0 || evidence.SourceTriggers != 5 || evidence.DeferredTriggers != 1 {
		t.Fatalf("refund validation was not deduplicated at commit: %+v", evidence)
	}
	assertRefundCompositeBindingsAndImpactUpperBound(
		t, scenario.ctx, scenario.database, refund.RefundNo,
	)
}

func TestRefundWithoutEntitlementImpactPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioOptionsWithQuantity(t, true, true, 1)
	scenario := prepared.checkout
	const reviewerID = 9012
	management, err := billingservice.NewManagementRefundService(
		scenario.database, refundManagementAuthorizerStub{actor: reviewerID},
		&refundRecentAuthStub{actor: reviewerID}, refundBusinessAuditStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = management.ConfigureRefundPolicy(scenario.ctx, billingservice.RefundPolicyInput{
		CurrencyCode: "CNY", HighAmountThresholdMinor: prepared.order.TotalAmountMinor + 1,
		AutomaticApprovalEnabled: true,
	}, 0); err != nil {
		t.Fatal(err)
	}
	var target struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		AmountMinor   int64  `orm:"amount_minor"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &target, `SELECT invoice_item_no,
total_amount_minor AS amount_minor FROM plugin_idc_core_billing_invoice_item
WHERE invoice_no=?`, prepared.order.InvoiceNo); err != nil {
		t.Fatal(err)
	}
	refunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	input := billingservice.RefundPreviewInput{
		OrderNo: prepared.order.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: target.InvoiceItemNo, Quantity: 1, AmountMinor: target.AmountMinor,
		}},
	}
	preview, err := refunds.PreviewOnlineRefund(scenario.ctx, input)
	if err != nil || preview == nil || len(preview.Items) != 1 || len(preview.Items[0].Impacts) != 0 {
		t.Fatalf("preview order-item-only refund: preview=%+v err=%v", preview, err)
	}
	refund, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest: preview.ImpactDigest, Reason: billingservice.RefundReasonNoLongerNeeded,
		IdempotencyKey: "refund-without-entitlement-impact-0001", Preview: input,
	})
	if err != nil || refund == nil {
		t.Fatalf("submit order-item-only refund: refund=%+v err=%v", refund, err)
	}
	impactCount, err := scenario.database.GetCount(scenario.ctx,
		`SELECT 1 FROM plugin_idc_core_refund_entitlement_impact WHERE refund_no=?`, refund.RefundNo)
	if err != nil || impactCount != 0 {
		t.Fatalf("order-item-only refund unexpectedly created entitlement impacts: count=%d err=%v", impactCount, err)
	}
}

func assertRefundCompositeBindingsAndImpactUpperBound(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	refundNo string,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
DROP TRIGGER trg_idc_refund_impact_composition ON plugin_idc_core_refund_entitlement_impact;
DROP TRIGGER trg_idc_refund_source_composition ON plugin_idc_core_refund_source_allocation;
DROP TRIGGER trg_idc_refund_lock_composition ON plugin_idc_core_refund_entitlement_lock;
DROP TRIGGER trg_idc_refund_item_immutable ON plugin_idc_core_refund_request_item;`); err != nil {
		t.Fatalf("isolate structural refund constraints: %v", err)
	}
	tests := []struct {
		name       string
		constraint string
		statement  string
	}{
		{
			name: "impact", constraint: "fk_idc_refund_impact_item",
			statement: `INSERT INTO plugin_idc_core_refund_entitlement_impact(
tenant_id,impact_no,refund_no,refund_item_no,order_item_no,customer_no,currency_code,
entitlement_no,entitlement_version_snapshot,status_snapshot,requires_termination,
impact_digest,created_at)
SELECT tenant_id,'cross-refund-impact','cross-refund-no',refund_item_no,order_item_no,
customer_no,currency_code,entitlement_no,entitlement_version_snapshot,status_snapshot,
requires_termination,repeat('8',64),created_at
FROM plugin_idc_core_refund_entitlement_impact WHERE refund_no='` + refundNo + `' LIMIT 1`,
		},
		{
			name: "source", constraint: "fk_idc_refund_source_item",
			statement: `INSERT INTO plugin_idc_core_refund_source_allocation(
tenant_id,refund_source_no,refund_no,refund_item_no,payment_allocation_no,
payment_source_type_code,payment_source_reference,amount_minor,execution_owner_code,
content_digest,created_at,credit_exposure_no,credit_component_code,
credit_repayment_allocation_no,credit_plan_digest)
SELECT tenant_id,'cross-refund-source','cross-refund-no',refund_item_no,payment_allocation_no,
payment_source_type_code,payment_source_reference,amount_minor,execution_owner_code,
repeat('9',64),created_at,credit_exposure_no,credit_component_code,
credit_repayment_allocation_no,credit_plan_digest
FROM plugin_idc_core_refund_source_allocation WHERE refund_no='` + refundNo + `' LIMIT 1`,
		},
		{
			name: "lock", constraint: "fk_idc_refund_lock_impact",
			statement: `INSERT INTO plugin_idc_core_refund_entitlement_lock(
tenant_id,lock_no,refund_no,refund_item_no,impact_no,entitlement_no,
expected_entitlement_version,released_at,release_reason_code,created_at)
SELECT tenant_id,'cross-refund-lock','cross-refund-no',refund_item_no,impact_no,
entitlement_no,expected_entitlement_version,COALESCE(released_at,CURRENT_TIMESTAMP),
COALESCE(release_reason_code,'cancelled'),created_at
FROM plugin_idc_core_refund_entitlement_lock WHERE refund_no='` + refundNo + `' LIMIT 1`,
		},
	}
	for _, test := range tests {
		if _, err := database.Exec(ctx, test.statement); err == nil ||
			!strings.Contains(err.Error(), test.constraint) {
			t.Fatalf("cross-refund %s was not rejected by %s: %v", test.name, test.constraint, err)
		}
	}
	setup := []struct {
		statement string
		args      []any
	}{
		{statement: `ALTER TABLE plugin_idc_core_refund_entitlement_impact
DROP CONSTRAINT uk_idc_refund_impact_entitlement`},
		{statement: `UPDATE plugin_idc_core_refund_request_item SET quantity=1 WHERE refund_no=?`, args: []any{refundNo}},
		{statement: `INSERT INTO plugin_idc_core_refund_entitlement_impact(
tenant_id,impact_no,refund_no,refund_item_no,order_item_no,customer_no,currency_code,
entitlement_no,entitlement_version_snapshot,status_snapshot,requires_termination,
impact_digest,created_at)
SELECT tenant_id,'refund-impact-over-quantity',refund_no,refund_item_no,order_item_no,
customer_no,currency_code,entitlement_no,entitlement_version_snapshot,status_snapshot,
requires_termination,repeat('7',64),created_at
FROM plugin_idc_core_refund_entitlement_impact WHERE refund_no=? LIMIT 1`, args: []any{refundNo}},
		{statement: `INSERT INTO plugin_idc_core_refund_entitlement_lock(
tenant_id,lock_no,refund_no,refund_item_no,impact_no,entitlement_no,
expected_entitlement_version,released_at,release_reason_code,created_at)
SELECT tenant_id,'refund-lock-over-quantity',refund_no,refund_item_no,
'refund-impact-over-quantity',entitlement_no,expected_entitlement_version,
COALESCE(released_at,CURRENT_TIMESTAMP),COALESCE(release_reason_code,'cancelled'),created_at
FROM plugin_idc_core_refund_entitlement_lock WHERE refund_no=? LIMIT 1`, args: []any{refundNo}},
	}
	for _, statement := range setup {
		if _, err := database.Exec(ctx, statement.statement, statement.args...); err != nil {
			t.Fatalf("prepare impact quantity overflow: %v", err)
		}
	}
	if _, err := database.Exec(ctx,
		`SELECT plugin_idc_core_validate_refund_composition(?)`, refundNo); err == nil ||
		!strings.Contains(err.Error(), "refund composition does not match request") {
		t.Fatalf("impact count above refunded quantity was not rejected: %v", err)
	}
}

func TestRefundValidationConstraintTriggerConcurrentReplayPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}
	migration, err := os.ReadFile(filepath.Join(
		filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql",
		"087-generic-refund-entitlement.sql",
	))
	if err != nil {
		t.Fatal(err)
	}
	const (
		guardStart = "DO $constraint_trigger$"
		guardEnd   = "$constraint_trigger$;"
	)
	start := strings.Index(string(migration), guardStart)
	if start < 0 {
		t.Fatal("refund validation constraint-trigger guard is unavailable")
	}
	endOffset := strings.Index(string(migration[start:]), guardEnd)
	if endOffset < 0 {
		t.Fatal("refund validation constraint-trigger guard is incomplete")
	}
	guard := string(migration[start : start+endOffset+len(guardEnd)])

	if _, err = database.Exec(ctx, `DROP TRIGGER trg_idc_refund_validation_queue_composition
ON plugin_idc_core_refund_composition_validation_queue`); err != nil {
		t.Fatalf("drop refund validation constraint trigger: %v", err)
	}
	const concurrentReplays = 8
	var wait sync.WaitGroup
	errors := make(chan error, concurrentReplays)
	startReplays := make(chan struct{})
	wait.Add(concurrentReplays)
	for range concurrentReplays {
		go func() {
			defer wait.Done()
			<-startReplays
			_, replayErr := database.Exec(ctx, guard)
			errors <- replayErr
		}()
	}
	close(startReplays)
	wait.Wait()
	close(errors)
	for replayErr := range errors {
		if replayErr != nil {
			t.Fatalf("concurrent refund constraint-trigger replay: %v", replayErr)
		}
	}
	var evidence struct {
		Exact int `orm:"exact"`
	}
	if err = database.GetScan(ctx, &evidence, `SELECT COUNT(*) AS exact
FROM pg_constraint AS constraint_row
JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint=constraint_row.oid
WHERE constraint_row.conrelid='plugin_idc_core_refund_composition_validation_queue'::regclass
  AND constraint_row.conname='trg_idc_refund_validation_queue_composition'
  AND constraint_row.contype='t'
  AND constraint_row.condeferrable AND constraint_row.condeferred
  AND trigger_row.tgrelid=constraint_row.conrelid
  AND NOT trigger_row.tgisinternal
  AND trigger_row.tgname='trg_idc_refund_validation_queue_composition'
  AND trigger_row.tgfoid='plugin_idc_core_run_queued_refund_composition_validation()'::regprocedure
  AND trigger_row.tgtype=5 AND trigger_row.tgenabled='O'
  AND trigger_row.tgdeferrable AND trigger_row.tginitdeferred`); err != nil {
		t.Fatal(err)
	}
	if evidence.Exact != 1 {
		t.Fatalf("concurrent replay did not preserve one exact constraint trigger: %d", evidence.Exact)
	}

	if _, err = database.Exec(ctx, `
DROP TRIGGER trg_idc_refund_validation_queue_composition
ON plugin_idc_core_refund_composition_validation_queue;
CREATE TRIGGER trg_idc_refund_validation_queue_composition
AFTER INSERT ON plugin_idc_core_refund_composition_validation_queue
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_run_queued_refund_composition_validation();`); err != nil {
		t.Fatalf("install wrong refund validation trigger identity: %v", err)
	}
	if _, err = database.Exec(ctx, guard); err == nil ||
		!strings.Contains(err.Error(), "catalog contract is invalid") {
		t.Fatalf("wrong refund validation trigger identity was not rejected: %v", err)
	}
}

func TestTrialEntitlementUnitCannotExceedFrozenOrderQuantityPostgreSQL(t *testing.T) {
	ctx, database := prepareTrialUnitInvariantScenario(t)
	var normal struct {
		Count             int `orm:"count"`
		Maximum           int `orm:"maximum"`
		StatementTriggers int `orm:"statement_triggers"`
	}
	if err := database.GetScan(ctx, &normal, `SELECT
(SELECT COUNT(*) FROM plugin_idc_core_trial_entitlement) AS count,
(SELECT MAX(unit_sequence) FROM plugin_idc_core_trial_entitlement) AS maximum,
(SELECT COUNT(*) FROM pg_trigger trigger_row
 JOIN pg_class relation_row ON relation_row.oid=trigger_row.tgrelid
 JOIN pg_namespace namespace_row ON namespace_row.oid=relation_row.relnamespace
 WHERE namespace_row.nspname=current_schema() AND NOT trigger_row.tgisinternal
 AND (trigger_row.tgtype & 1)=0 AND trigger_row.tgname IN(
 'trg_idc_validate_trial_entitlement_units_insert',
 'trg_idc_validate_trial_entitlement_units_update')) AS statement_triggers`); err != nil {
		t.Fatal(err)
	}
	if normal.Count != 2 || normal.Maximum != 2 || normal.StatementTriggers != 2 {
		t.Fatalf("normal trial units were not preserved: %+v", normal)
	}
	_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_trial_entitlement(
tenant_id,entitlement_no,scope_no,customer_no,product_no,version_no,billing_rule_no,
order_no,invoice_no,order_item_no,unit_sequence,currency_code,trial_limit_quantity,
status_code,hold_expires_at,consumed_at,released_at,identity_evidence_digest,
content_digest,version,created_at,updated_at)
SELECT tenant_id,'trial-entitlement-bulk-'||unit_sequence,scope_no,customer_no,product_no,
version_no,billing_rule_no,order_no,invoice_no,order_item_no,unit_sequence+2,currency_code,
trial_limit_quantity,status_code,hold_expires_at,consumed_at,released_at,
identity_evidence_digest,repeat('e',64),1,created_at,updated_at
FROM plugin_idc_core_trial_entitlement`)
	if err == nil || !strings.Contains(err.Error(), "trial entitlement unit exceeds the frozen order item quantity") {
		t.Fatalf("out-of-range bulk trial units were not rejected: %v", err)
	}
	_, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_trial_entitlement(
tenant_id,entitlement_no,scope_no,customer_no,product_no,version_no,billing_rule_no,
order_no,invoice_no,order_item_no,unit_sequence,currency_code,trial_limit_quantity,
status_code,hold_expires_at,consumed_at,released_at,identity_evidence_digest,
content_digest,version,created_at,updated_at)
SELECT tenant_id,'trial-entitlement-out-of-range',scope_no,customer_no,product_no,version_no,
billing_rule_no,order_no,invoice_no,order_item_no,3,currency_code,trial_limit_quantity,
status_code,hold_expires_at,consumed_at,released_at,identity_evidence_digest,
repeat('f',64),1,created_at,updated_at
FROM plugin_idc_core_trial_entitlement ORDER BY unit_sequence LIMIT 1`)
	if err == nil || !strings.Contains(err.Error(), "trial entitlement unit exceeds the frozen order item quantity") {
		t.Fatalf("out-of-range trial unit was not rejected: %v", err)
	}
	if _, err = database.Exec(ctx, `ALTER TABLE plugin_idc_core_trial_entitlement
DISABLE TRIGGER trg_idc_trial_entitlement_guard`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_trial_entitlement
SET unit_sequence=unit_sequence`); err != nil {
		t.Fatalf("valid bulk trial-unit update: %v", err)
	}
	if _, err = database.Exec(ctx, `ALTER TABLE plugin_idc_core_trial_entitlement
ENABLE TRIGGER trg_idc_trial_entitlement_guard`); err != nil {
		t.Fatal(err)
	}
	updated, err := database.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_trial_entitlement`)
	if err != nil || updated != 2 {
		t.Fatalf("bulk trial-unit update did not preserve the set: count=%d err=%v", updated, err)
	}
}

func TestHistoricalServiceSubjectsBackfillAndValidatePostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	if _, err := database.Exec(ctx, `
ALTER TABLE plugin_idc_core_service_price_component_snapshot
  DROP CONSTRAINT ck_idc_price_component_subject,
  DROP CONSTRAINT fk_idc_price_component_subject;
ALTER TABLE plugin_idc_core_service_price_snapshot
  DROP CONSTRAINT ck_idc_price_snapshot_subject,
  DROP CONSTRAINT fk_idc_service_price_service_subject;
ALTER TABLE plugin_idc_core_business_operation_event
  DROP CONSTRAINT ck_idc_business_operation_event_subject,
  DROP CONSTRAINT fk_idc_business_operation_event_subject;
ALTER TABLE plugin_idc_core_business_operation
  DROP CONSTRAINT ck_idc_business_operation_subject,
  DROP CONSTRAINT fk_idc_business_operation_price_subject;
ALTER TABLE plugin_idc_core_service_instance DISABLE TRIGGER ALL;
ALTER TABLE plugin_idc_core_service_price_snapshot DISABLE TRIGGER ALL;
ALTER TABLE plugin_idc_core_service_price_component_snapshot DISABLE TRIGGER ALL;
ALTER TABLE plugin_idc_core_business_operation DISABLE TRIGGER ALL;
ALTER TABLE plugin_idc_core_business_operation_event DISABLE TRIGGER ALL;
INSERT INTO plugin_idc_core_service_instance(
 tenant_id,service_no,customer_no,source_code,product_no,version_no,billing_rule_no,
 configuration_snapshot_no,product_type_code,currency_code,binding_no,pool_no,
 entitlement_status_code,runtime_status_code,provider_task_status_code,renewal_mode_code,
 period_unit_code,period_count,version)
VALUES(0,'legacy-service-subject','legacy-customer','manual','legacy-product','legacy-version',
 'legacy-billing','legacy-configuration','cloud_server','CNY','legacy-binding','legacy-pool',
 'pending_activation','pending_provision','none','unresolved','once',1,1);
INSERT INTO plugin_idc_core_service_price_snapshot(
 tenant_id,price_snapshot_no,service_no,subject_type_code,subject_no,customer_no,phase_code,
 product_no,version_no,billing_rule_no,configuration_snapshot_no,currency_code,purchase_cycles,
 period_unit_code,period_count,renewal_mode_code,tax_policy_no,price_includes_tax,
 tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,
 rounding_mode_code,rounding_quantum_minor,component_count,subtotal_minor,discount_minor,
 net_amount_minor,tax_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
VALUES(0,'legacy-price-subject','legacy-service-subject','service',NULL,'legacy-customer',
 'new_purchase','legacy-product','legacy-version','legacy-billing','legacy-configuration','CNY',
 1,'once',1,'unresolved','legacy-tax',FALSE,'exempt',NULL,'general','total','half_up',1,
 1,100,0,100,0,100,repeat('a',64),repeat('b',64));
INSERT INTO plugin_idc_core_service_price_component_snapshot(
 tenant_id,component_snapshot_no,price_snapshot_no,service_no,subject_type_code,subject_no,
 line_number,component_type_code,charge_phase_code,source_reference,quantity,
 unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES(0,'legacy-component-subject','legacy-price-subject','legacy-service-subject','service',NULL,
 1,'base','total','legacy-base',1,100,100,repeat('c',64),repeat('d',64));
INSERT INTO plugin_idc_core_business_operation(
 tenant_id,operation_no,operation_type_code,customer_no,service_no,subject_type_code,subject_no,
 currency_code,price_snapshot_no,status_code,financial_settlement_required,execution_mode_code,
 pool_no,idempotency_digest,request_digest,content_digest,expected_service_version,
 expected_subject_version,version)
VALUES(0,'legacy-operation-subject','refund','legacy-customer','legacy-service-subject',
 'service',NULL,'CNY',NULL,'quoted',FALSE,'manual_review','legacy-pool',
 repeat('e',64),repeat('f',64),repeat('1',64),1,NULL,1);
INSERT INTO plugin_idc_core_business_operation_event(
 tenant_id,event_no,operation_no,service_no,subject_type_code,subject_no,event_sequence,
 from_status_code,to_status_code,reason_code,actor_type_code,actor_reference,evidence_digest,content_digest)
VALUES(0,'legacy-event-subject','legacy-operation-subject','legacy-service-subject','service',NULL,
 1,NULL,'quoted','migration.backfill','migration','migration',repeat('2',64),repeat('3',64));
ALTER TABLE plugin_idc_core_service_instance ENABLE TRIGGER ALL;
ALTER TABLE plugin_idc_core_service_price_snapshot ENABLE TRIGGER ALL;
ALTER TABLE plugin_idc_core_service_price_component_snapshot ENABLE TRIGGER ALL;
ALTER TABLE plugin_idc_core_business_operation ENABLE TRIGGER ALL;
ALTER TABLE plugin_idc_core_business_operation_event ENABLE TRIGGER ALL;`); err != nil {
		t.Fatalf("seed unambiguous historical service subjects: %v", err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}
	migration, err := os.ReadFile(filepath.Join(
		filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql",
		"088-generic-entitlement-renewal.sql",
	))
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		executeCommerceMigrationStatements(
			t, ctx, database, "088-generic-entitlement-renewal.sql", string(migration),
		)
	}
	var evidence struct {
		PriceSubject     string `orm:"price_subject"`
		ComponentSubject string `orm:"component_subject"`
		OperationSubject string `orm:"operation_subject"`
		ExpectedVersion  int64  `orm:"expected_version"`
		EventSubject     string `orm:"event_subject"`
		Validated        int    `orm:"validated"`
		RenewalValidated int    `orm:"renewal_validated"`
	}
	if err = database.GetScan(ctx, &evidence, `SELECT
(SELECT subject_no FROM plugin_idc_core_service_price_snapshot WHERE price_snapshot_no='legacy-price-subject') AS price_subject,
(SELECT subject_no FROM plugin_idc_core_service_price_component_snapshot WHERE component_snapshot_no='legacy-component-subject') AS component_subject,
(SELECT subject_no FROM plugin_idc_core_business_operation WHERE operation_no='legacy-operation-subject') AS operation_subject,
(SELECT expected_subject_version FROM plugin_idc_core_business_operation WHERE operation_no='legacy-operation-subject') AS expected_version,
(SELECT subject_no FROM plugin_idc_core_business_operation_event WHERE event_no='legacy-event-subject') AS event_subject,
(SELECT COUNT(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace
 AND convalidated AND conname IN(
 'ck_idc_price_snapshot_subject','fk_idc_service_price_service_subject',
 'ck_idc_price_component_subject','fk_idc_price_component_subject',
 'ck_idc_business_operation_subject','fk_idc_business_operation_price_subject',
 'ck_idc_business_operation_event_subject','fk_idc_business_operation_event_subject')) AS validated,
(SELECT COUNT(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace
 AND convalidated AND conname IN(
 'ck_idc_fulfillment_policy_renewal','ck_idc_product_fulfillment_renewal',
 'ck_idc_product_delivery_renewal','ck_idc_service_entitlement_renewal',
 'fk_idc_service_renewal_policy_entitlement',
 'fk_idc_service_renewal_quote_request_entitlement')) AS renewal_validated`); err != nil {
		t.Fatal(err)
	}
	if evidence.PriceSubject != "legacy-service-subject" ||
		evidence.ComponentSubject != "legacy-service-subject" ||
		evidence.OperationSubject != "legacy-service-subject" || evidence.ExpectedVersion != 1 ||
		evidence.EventSubject != "legacy-service-subject" || evidence.Validated != 8 ||
		evidence.RenewalValidated != 6 {
		t.Fatalf("historical service subjects were not safely backfilled: %+v", evidence)
	}
	if _, err = database.Exec(ctx, `
ALTER TABLE plugin_idc_core_business_operation
  DROP CONSTRAINT ck_idc_business_operation_subject;
DO $ambiguous_fixture$
BEGIN
  EXECUTE 'ALTER TABLE plugin_idc_core_business_operation DISABLE TRIGGER trg_idc_business_operation_intent_guard';
  EXECUTE 'ALTER TABLE plugin_idc_core_business_operation DISABLE TRIGGER trg_idc_business_operation_event_composition';
  UPDATE plugin_idc_core_business_operation SET expected_subject_version=2
  WHERE operation_no='legacy-operation-subject';
  EXECUTE 'ALTER TABLE plugin_idc_core_business_operation ENABLE TRIGGER trg_idc_business_operation_intent_guard';
  EXECUTE 'ALTER TABLE plugin_idc_core_business_operation ENABLE TRIGGER trg_idc_business_operation_event_composition';
END;
$ambiguous_fixture$;`); err != nil {
		t.Fatalf("prepare ambiguous historical evidence: %v", err)
	}
	var migrationErr error
	for _, statement := range dialect.SplitSQLStatements(string(migration)) {
		if _, migrationErr = database.Exec(ctx, statement); migrationErr != nil {
			break
		}
	}
	if migrationErr == nil ||
		!strings.Contains(migrationErr.Error(), "historical service subject evidence is ambiguous") {
		t.Fatalf("ambiguous historical service evidence was not rejected: %v", migrationErr)
	}
	var failureSafety struct {
		DisabledTriggers int `orm:"disabled_triggers"`
		SubjectFKs       int `orm:"subject_fks"`
	}
	if err = database.GetScan(ctx, &failureSafety, `SELECT
(SELECT COUNT(*) FROM pg_trigger trigger_row
 JOIN pg_class relation_row ON relation_row.oid=trigger_row.tgrelid
 JOIN pg_namespace namespace_row ON namespace_row.oid=relation_row.relnamespace
 WHERE namespace_row.nspname=current_schema() AND NOT trigger_row.tgisinternal
 AND trigger_row.tgenabled='D' AND trigger_row.tgname IN(
 'trg_idc_service_price_immutable','trg_idc_service_price_component_immutable',
 'trg_idc_business_operation_intent_guard','trg_idc_business_operation_event_composition',
 'trg_idc_business_operation_event_immutable','trg_idc_business_operation_event_item_composition',
 'trg_idc_business_operation_subject')) AS disabled_triggers,
(SELECT COUNT(*) FROM pg_constraint WHERE connamespace=current_schema()::regnamespace AND conname IN(
 'fk_idc_price_component_subject','fk_idc_business_operation_price_subject',
 'fk_idc_business_operation_event_subject')) AS subject_fks`); err != nil {
		t.Fatal(err)
	}
	if failureSafety.DisabledTriggers != 0 || failureSafety.SubjectFKs != 3 {
		t.Fatalf("failed migration left unsafe database state: %+v", failureSafety)
	}
}

func prepareTrialUnitInvariantScenario(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)
	seedCommerceTrialRule(t, ctx, database)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_billing_rule
SET trial_quantity=2 WHERE billing_rule_no='billing-commerce-service-trial';
INSERT INTO plugin_idc_core_customer_individual_profile(
customer_no,legal_name,legal_name_masked,identity_identifier_masked,identity_lookup_digest,
identity_case_no,identity_status_code,identity_verified_at,identity_expires_at)
VALUES('customer-commerce-service-1','Trial Customer','T**** C*******','110***********001X',
repeat('a',64),'trial-unit-identity-case','verified',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 year')`); err != nil {
		t.Fatal(err)
	}
	reader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	events, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatal(err)
	}
	credits, err := billingservice.NewCreditUsageService(database)
	if err != nil {
		t.Fatal(err)
	}
	settlements, err := billingservice.NewFinancialSettlementService(database, events, credits)
	if err != nil {
		t.Fatal(err)
	}
	identity := commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-1", UserID: 101,
	}}
	service, err := newService(database, resolver, identity, rand.Reader, settlements)
	if err != nil {
		t.Fatal(err)
	}
	purchase := trialPurchaseConfiguration(t, ctx, reader)
	cart, err := service.OpenAccountCart(ctx, "CNY")
	if err != nil {
		t.Fatal(err)
	}
	cart, err = service.AddAccountItem(ctx, AddItemInput{
		ExpectedCartVersion: cart.Version, Quantity: 2, Purchase: purchase,
	})
	if err != nil {
		t.Fatal(err)
	}
	quote, err := service.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: cart.Version})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CheckoutAccountQuote(ctx, CheckoutRequest{
		QuoteNo: quote.QuoteNo, IdempotencyKey: "trial-unit-invariant-checkout-0001",
	}); err != nil {
		t.Fatal(err)
	}
	return ctx, database
}
