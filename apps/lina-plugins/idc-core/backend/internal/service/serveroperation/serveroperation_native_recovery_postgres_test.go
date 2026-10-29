// Recovery tests use real native checkout, settlement, online refund and posted
// balance refund owners in an isolated PostgreSQL schema. No provider is called.

package serveroperation

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

func nativeRecoveryReader(t *testing.T, f nativeCommercialFixture) {
	t.Helper()
	reader, err := billing.NewCompletedRefundEvidenceReader(f.database)
	if err != nil {
		t.Fatal(err)
	}
	f.native.refunds = reader
}

func nativeRecoveryPaid(t *testing.T, f nativeCommercialFixture, action serveroperationresourcecap.Action, credit bool) *servercommercialcap.Purchase {
	t.Helper()
	in := servercommercialcap.QuoteRequest{Target: f.target, Action: action, Parameters: json.RawMessage(`{"name":"recovery-backup"}`), IdempotencyKey: "recovery-paid-quote"}
	if action == serveroperationresourcecap.ActionTrafficAdd {
		in.Parameters = nil
		in.PackageNo = "POL_checkout_test"
	}
	q, err := f.native.Quote(f.ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: q.QuoteNo, IdempotencyKey: "recovery-paid-checkout"})
	if err != nil {
		t.Fatal(err)
	}
	return nativeRecoverySettle(t, f, p, credit)
}

func nativeRecoverySettle(t *testing.T, f nativeCommercialFixture, p *servercommercialcap.Purchase, credit bool) *servercommercialcap.Purchase {
	t.Helper()
	var settled *billing.FinancialSettlement
	var err error
	if credit {
		nativeRecoveryCreditAccount(t, f)
		settled, err = f.settlementService(t).SettleOrderWithCredit(f.ctx, billing.CreditOrderSettlementInput{OrderNo: p.OrderNo, ActorType: "customer", ActorID: 101})
	} else {
		f.payNativeInvoice(t, p.InvoiceNo, p.TotalAmountMinor)
		settled, err = f.settlementService(t).SettleOrder(f.ctx, p.OrderNo)
	}
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := NewServerOperationSettlementConsumer(f.database, f.native.evidence)
	if err != nil {
		t.Fatal(err)
	}
	var event outbox.Event
	if err = f.database.GetScan(f.ctx, &event, `SELECT event_no,event_type,event_version,topic,aggregate_type,aggregate_no,correlation_no,payload,metadata FROM plugin_idc_core_outbox_event WHERE aggregate_no=? AND correlation_no=?`, p.OrderNo, settled.SettlementNo); err != nil {
		t.Fatal(err)
	}
	router, err := fulfillment.NewRoutedConsumer(&accessoryChainGeneric{}, consumer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := router.Consume(f.ctx, outbox.ClaimedEvent{Event: event}); err != nil {
		t.Fatalf("settlement error=%v", err)
	}
	nativeRecoveryReader(t, f)
	return p
}

func nativeRecoveryRefund(t *testing.T, f nativeCommercialFixture, p *servercommercialcap.Purchase, amount int64, run bool) {
	t.Helper()
	if _, err := f.database.Exec(f.ctx, `INSERT INTO plugin_idc_core_refund_policy_version(policy_no,currency_code,policy_version,high_amount_threshold_minor,automatic_approval_enabled,content_digest,configured_by) VALUES('POL_recovery','CNY',1,100000,TRUE,repeat('a',64),101)`); err != nil {
		t.Fatal(err)
	}
	credit, err := billing.NewCreditUsageService(f.database)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := billing.NewAccountRefundService(f.database, cancellationPrincipal{}, trafficTestAudit{}, credit)
	if err != nil {
		t.Fatal(err)
	}
	item, err := f.database.GetValue(f.ctx, `SELECT invoice_item_no FROM plugin_idc_core_billing_invoice_item WHERE invoice_no=? ORDER BY line_number LIMIT 1`, p.InvoiceNo)
	if err != nil {
		t.Fatal(err)
	}
	in := billing.RefundPreviewInput{OrderNo: p.OrderNo, Selections: []billing.RefundSelectionInput{{InvoiceItemNo: item.String(), Quantity: 1, AmountMinor: amount}}}
	preview, err := owner.PreviewOnlineRefund(f.ctx, in)
	if err != nil {
		t.Fatalf("refund preview: %+v", err)
	}
	if _, err = owner.SubmitOnlineRefund(f.ctx, billing.RefundSubmissionInput{Preview: in, PreviewDigest: preview.ImpactDigest, Reason: billing.RefundReasonNoLongerNeeded, IdempotencyKey: "recovery-real-refund"}); err != nil {
		t.Fatalf("refund submit: %+v", err)
	}
	if !run {
		return
	}
	nativeRecoveryRunRefund(t, f)
}

func nativeRecoveryRunRefund(t *testing.T, f nativeCommercialFixture) {
	t.Helper()
	events, err := outbox.NewEnqueuer(f.database, outbox.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := billing.NewLocalRefundWorker(f.database, events, tasks)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := worker.Run(f.ctx, 10); err != nil || n < 1 {
		t.Fatalf("actual refund worker n=%d: %+v", n, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_refund_request WHERE status_code='succeeded'`, 1)
}

func TestNativeReadyParentRecoveryPostgreSQL(t *testing.T) {
	f, p := nativeFreeExecution(t)
	if _, err := f.database.Exec(f.ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='suspended' WHERE entitlement_no=?`, f.target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() { defer group.Done(); _, err := f.native.ReconcileReady(f.ctx, 10); errs <- err }()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='manual_review' AND last_error_code='server_commercial.ready_blocked.parent_inactive'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation_event WHERE reason_code='server_commercial.ready_blocked.parent_inactive'`, 1)
	if _, err := f.database.Exec(f.ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active' WHERE entitlement_no=?`, f.target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if n, err := f.native.ReconcileReady(f.ctx, 10); err != nil || n != 1 {
		t.Fatalf("resume n=%d error=%+v", n, err)
	}
	if err := f.native.WithReadyOperation(f.ctx, p.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) { return "task_recovered", nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := f.native.ReconcileReady(f.ctx, 10); err != nil {
		t.Fatal(err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='executing' AND provider_task_no='task_recovered'`, 1)
	if err := f.native.Complete(f.ctx, servercommercialcap.Completion{OperationNo: p.OperationNo, EntitlementNo: f.target.EntitlementNo, TaskNo: "task_recovered", Status: servercommercialcap.CompletionUnknown, EvidenceDigest: strings.Repeat("a", 64), ObservedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if n, err := f.native.ReconcileReady(f.ctx, 10); err != nil || n != 0 {
		t.Fatalf("unknown was recovered n=%d err=%v", n, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='manual_review' AND provider_task_no='task_recovered'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE status_code='pending_binding'`, 1)
}

func TestNativeReadyActualRefundPostgreSQL(t *testing.T) {
	for _, action := range []serveroperationresourcecap.Action{serveroperationresourcecap.ActionBackupCreate, serveroperationresourcecap.ActionTrafficAdd} {
		t.Run(string(action), func(t *testing.T) {
			f := newNativeCommercialFixture(t)
			p := nativeRecoveryPaid(t, f, action, false)
			nativeRecoveryRefund(t, f, p, p.TotalAmountMinor, true)
			if n, err := f.native.ReconcileReady(f.ctx, 10); err != nil || n != 1 {
				t.Fatalf("cancel n=%d error=%+v", n, err)
			}
			if n, err := f.native.ReconcileReady(f.ctx, 10); err != nil || n != 0 {
				t.Fatalf("replay n=%d error=%+v", n, err)
			}
			f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='cancelled' AND financially_settled_at IS NOT NULL AND execution_started_at IS NULL AND provider_task_no IS NULL`, 1)
			if action == serveroperationresourcecap.ActionBackupCreate {
				f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE status_code='creation_cancelled' AND object_no IS NULL AND NOT auto_renew_enabled`, 1)
				nativeLifecycleMigration(t, f.database, "151-native-server-accessory-lifecycle.sql")
			} else {
				assertTrafficPackageOccupancy(t, f.accessoryCheckoutFixture, 1)
			}
		})
	}
}

func TestNativeReadyPendingPartialRefundPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	p := nativeRecoveryPaid(t, f, serveroperationresourcecap.ActionBackupCreate, false)
	nativeRecoveryRefund(t, f, p, p.TotalAmountMinor/2, true)
	if n, err := f.native.ReconcileReady(f.ctx, 10); err != nil || n != 1 {
		t.Fatalf("partial hold n=%d error=%+v", n, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='manual_review' AND last_error_code='server_commercial.ready_blocked.refund_pending'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE status_code='pending_binding'`, 1)
	if rows, err := f.native.ListReady(f.ctx, 10); err != nil || len(rows) != 0 {
		t.Fatalf("blocked operation still ready: %v / %v", rows, err)
	}
}

// nativeRecoveryCreditAccount uses the public approval owner to produce a real
// credit account, rather than fabricating exposure or refund movement rows.
func nativeRecoveryCreditAccount(t *testing.T, f nativeCommercialFixture) {
	t.Helper()
	owner, err := billing.NewCreditApprovalService(f.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.ConfigureCreditPolicy(f.ctx, billing.CreditPolicyInput{CurrencyCode: "CNY", MaximumLimitMinor: 50000, DualReviewThresholdMinor: 20000, BillingDay: 15, RepaymentPeriodDays: 10, GracePeriodDays: 3, ConfiguredBy: 101, IdempotencyKey: "recovery-credit-policy"}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	app, err := owner.SubmitCreditApplication(f.ctx, billing.CreditApplicationInput{CustomerNo: f.target.CustomerNo, CurrencyCode: "CNY", RequestedLimitMinor: 8000, RequestedValidFrom: now.Add(-time.Minute), RequestedValidUntil: now.AddDate(1, 0, 0), RiskEvidenceReference: "private:synthetic/recovery-credit", RiskEvidenceDigest: strings.Repeat("e", 64), RequestActorType: "customer", RequestedBy: 101, IdempotencyKey: "recovery-credit-application"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.ReviewCreditApplication(f.ctx, billing.CreditReviewInput{ApplicationNo: app.ApplicationNo, Decision: billing.CreditDecisionApprove, ReviewerID: 102, CommentDigest: strings.Repeat("f", 64), EvidenceDigest: strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
}

func TestNativeReadyActualCreditRefundPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	p := nativeRecoveryPaid(t, f, serveroperationresourcecap.ActionBackupCreate, true)
	nativeRecoveryRefund(t, f, p, p.TotalAmountMinor, true)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_credit_account_movement WHERE source_type_code='credit_refund' AND movement_type_code='refund'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_billing_refund_allocation`, 0)
	if n, err := f.native.ReconcileReady(f.ctx, 10); err != nil || n != 1 {
		t.Fatalf("credit cancel n=%d error=%+v", n, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='cancelled' AND financially_settled_at IS NOT NULL`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE status_code='creation_cancelled'`, 1)
}

func TestNativeReadyReversedRefundRemainsBlockedPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	p := nativeRecoveryPaid(t, f, serveroperationresourcecap.ActionBackupCreate, false)
	nativeRecoveryRefund(t, f, p, p.TotalAmountMinor, true)
	var allocation struct{ RefundAllocationNo, LedgerTransactionNo string }
	if err := f.database.GetScan(f.ctx, &allocation, `SELECT refund_allocation_no,ledger_transaction_no FROM plugin_idc_core_billing_refund_allocation WHERE invoice_no=? AND direction_code='apply'`, p.InvoiceNo); err != nil {
		t.Fatal(err)
	}
	balances, err := billing.NewBalanceService(f.database)
	if err != nil {
		t.Fatal(err)
	}
	reversal, err := balances.ReverseTransaction(f.ctx, billing.ReversalInput{OriginalTransactionNo: allocation.LedgerTransactionNo, SourceReference: "recovery-refund-reversal", SourceSnapshotDigest: strings.Repeat("c", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.native.invoices.ReverseRefund(f.ctx, billing.InvoiceRefundReversalInput{OriginalRefundAllocationNo: allocation.RefundAllocationNo, LedgerTransactionNo: reversal.TransactionNo, SourceReference: "recovery-allocation-reversal", AmountMinor: p.TotalAmountMinor}); err != nil {
		t.Fatal(err)
	}
	if n, err := f.native.ReconcileReady(f.ctx, 10); err != nil || n != 1 {
		t.Fatalf("reversed hold n=%d error=%+v", n, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation WHERE status_code='manual_review' AND last_error_code='server_commercial.ready_blocked.refund_pending'`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE status_code='pending_binding'`, 1)
	proof, err := f.native.refunds.ReadCompletedRefund(f.ctx, billing.CompletedRefundScope{OrderNo: p.OrderNo, InvoiceNo: p.InvoiceNo, CustomerNo: "CUS_foreign", CurrencyCode: "CNY"})
	if err != nil || proof.HasRequest || proof.CompletedAmountMinor != 0 {
		t.Fatalf("foreign refund proof leaked %+v / %v", proof, err)
	}
}
