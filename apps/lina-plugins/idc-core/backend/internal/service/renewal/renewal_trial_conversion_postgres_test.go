// This file exercises persisted conversion quotes/confirmation from an existing
// trial entitlement. The checkout helper supplies configuration evidence;
// supplier recovery and original trial purchase are separate test boundaries.

package renewal

import (
	"context"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
	"lina-plugin-idc-core/backend/internal/service/triallifecycle"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
)

func TestTrialConversionQuoteAndConfirmationPostgreSQL(t *testing.T) {
	runTrialConversionPostgreSQL(t, false, false)
}

func TestTrialConversionZeroAmountPostgreSQL(t *testing.T) {
	runTrialConversionPostgreSQL(t, true, false)
}

func TestActiveTrialConversionPreservesRemainingTrialPostgreSQL(t *testing.T) {
	runTrialConversionPostgreSQL(t, false, true)
}

func runTrialConversionPostgreSQL(t *testing.T, free, active bool) {
	t.Helper()
	ctx, database := newRenewalPostgreSQLDatabase(t)
	// The shared task DAO uses the default group outside domain transactions.
	// Bind it to this test's isolated schema and restore the prior configuration.
	originalConfig := gdb.GetAllConfig()
	if err := gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: {*database.GetConfig()}}); err != nil {
		t.Fatal(err)
	}
	database = g.DB()
	t.Cleanup(func() {
		if err := database.Close(ctx); err != nil {
			t.Error(err)
		}
		if err := gdb.SetConfig(originalConfig); err != nil {
			t.Error(err)
		}
	})
	reader, principal, fixture := newGenericRenewalCheckout(t, ctx, database)
	// This test begins with a provisioned local trial. It does not claim that an
	// SSL connector supports trial suspension; the provider is not called here.
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement
 SET charge_type_code='trial',entitlement_owner_id='idc-resource',status_code='suspended',
 started_at=CURRENT_TIMESTAMP-INTERVAL '2 days',expires_at=CURRENT_TIMESTAMP-INTERVAL '1 hour' WHERE entitlement_no=?`, fixture.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if active {
		if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active',expires_at=CURRENT_TIMESTAMP+INTERVAL '1 day' WHERE entitlement_no=?`, fixture.EntitlementNo); err != nil {
			t.Fatal(err)
		}
	}
	expectedStatus := "suspended"
	if active {
		expectedStatus = "active"
	}
	evaluator, err := newDefaultQuoteAdjustments(database)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := triallifecycle.NewRecoveryScheduler(database, tasks)
	if err != nil {
		t.Fatal(err)
	}
	if free {
		pricing, priceErr := salesmanagement.NewPricing(database)
		if priceErr != nil {
			t.Fatal(priceErr)
		}
		evaluator, err = commerce.NewAgentQuoteAdjustmentEvaluator(pricing, evaluator)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_purchase_price
 (tenant_id,price_no,customer_no,product_no,currency_code,charge_type_code,period_unit_code,period_count,adjustment_type_code,fixed_price_minor,status_code,version,content_digest)
 SELECT 0,'APR_trial_zero','CUS_renewal_owner',product_no,currency_code,charge_type_code,period_unit_code,period_count,'fixed_price',0,'active',1,repeat('a',64)
 FROM plugin_idc_core_commerce_quote_item_snapshot WHERE quote_item_no=?`, fixture.QuoteItemNo); err != nil {
			t.Fatal(err)
		}
	}
	runtime, err := NewWithTrialConversion(ctx, database, principal, reader, evaluator, recovery)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := runtime.ListTrialConversions(ctx, 1, 20)
	if err != nil || listed.Total != 1 || len(listed.Items) != 1 || listed.Items[0].EntitlementNo != fixture.EntitlementNo || len(listed.Items[0].Periods) == 0 {
		t.Fatalf("trial list=%+v err=%v", listed, err)
	}
	originalCustomer := principal.audience.CustomerNo
	principal.audience.CustomerNo = "cus_other_conversion"
	foreign, foreignErr := runtime.ListTrialConversions(ctx, 1, 20)
	principal.audience.CustomerNo = originalCustomer
	if foreignErr != nil || foreign.Total != 0 || len(foreign.Items) != 0 {
		t.Fatalf("foreign list=%+v err=%v", foreign, foreignErr)
	}
	input := QuoteInput{EntitlementNo: fixture.EntitlementNo, FormalBillingRuleNo: "bil_renewal_ssl_month", RenewalCycles: 1, IdempotencyKey: "convert-existing-trial"}
	quote, err := runtime.QuoteCurrentCustomer(ctx, input)
	if err != nil || quote == nil || quote.TotalAmountMinor != func() int64 {
		if free {
			return 0
		}
		return 12500
	}() {
		t.Fatalf("conversion quote=%+v err=%v", quote, err)
	}
	replay, err := runtime.QuoteCurrentCustomer(ctx, input)
	if err != nil || replay == nil || replay.QuoteNo != quote.QuoteNo || !replay.Reused {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	ordinary := input
	ordinary.FormalBillingRuleNo = ""
	ordinary.IdempotencyKey = "trial-ordinary-rejected"
	if _, err = runtime.QuoteCurrentCustomer(ctx, ordinary); !bizerr.Is(err, CodeConflict) && !bizerr.Is(err, CodeCapabilityUnavailable) {
		t.Fatalf("ordinary trial renewal: %v", err)
	}
	operation, err := runtime.ConfirmCurrentCustomer(ctx, ConfirmInput{QuoteNo: quote.QuoteNo, IdempotencyKey: "confirm-existing-trial"})
	if err != nil || operation == nil || operation.Status != func() string {
		if free {
			return "succeeded"
		}
		return "awaiting_financial_settlement"
	}() {
		t.Fatalf("confirm=%+v err=%+v", operation, err)
	}
	if free {
		converted, conversionErr := database.GetOne(ctx, `SELECT charge_type_code,status_code FROM plugin_idc_core_service_entitlement WHERE entitlement_no=?`, fixture.EntitlementNo)
		if conversionErr != nil || converted["charge_type_code"].String() != "recurring" || converted["status_code"].String() != "suspended" {
			t.Fatalf("free conversion=%v err=%v", converted, conversionErr)
		}
		tasksCount, taskErr := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_async_task WHERE task_type='trial.entitlement.resume' AND subject_no=?`, fixture.EntitlementNo)
		if taskErr != nil || tasksCount.Int() != 1 {
			t.Fatalf("free recovery=%v err=%v", tasksCount, taskErr)
		}
		confirmedAgain, replayErr := runtime.ConfirmCurrentCustomer(ctx, ConfirmInput{QuoteNo: quote.QuoteNo, IdempotencyKey: "confirm-existing-trial"})
		if replayErr != nil || confirmedAgain.OperationNo != operation.OperationNo || !confirmedAgain.Reused {
			t.Fatalf("free replay=%+v err=%v", confirmedAgain, replayErr)
		}
		return
	}
	count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_async_task WHERE task_type='trial.entitlement.resume'`)
	if err != nil || count.Int() != 0 {
		t.Fatalf("unpaid recovery count=%v err=%v", count, err)
	}
	row, err := database.GetOne(ctx, `SELECT charge_type_code,status_code,expires_at FROM plugin_idc_core_service_entitlement WHERE entitlement_no=?`, fixture.EntitlementNo)
	if err != nil || row["charge_type_code"].String() != "trial" || row["status_code"].String() != expectedStatus {
		t.Fatalf("unpaid changed facts=%v err=%v", row, err)
	}
	if _, err = runtime.PayCurrentCustomer(ctx, PayInput{OperationNo: operation.OperationNo}); !bizerr.Is(err, CodeBalanceInsufficient) {
		t.Fatalf("unfunded conversion=%v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='suspended' WHERE product_no='prd_renewal_ssl'`); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.PayCurrentCustomer(ctx, PayInput{OperationNo: operation.OperationNo}); !bizerr.Is(err, CodePriceUnavailable) {
		t.Fatalf("stopped conversion debited: %v", err)
	}
	stopped, stoppedErr := runtime.ListTrialConversions(ctx, 1, 20)
	if stoppedErr != nil || stopped.Total != 1 || len(stopped.Items) != 1 || len(stopped.Items[0].Periods) != 0 {
		t.Fatalf("stopped trial offered periods=%+v err=%v", stopped, stoppedErr)
	}
	// An unknown/currently changed version must never replace a trial config.
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		impl := runtime.(*serviceImpl)
		facts, err := lockOwnedEntitlementForQuote(ctx, database, principal.audience.CustomerNo, fixture.EntitlementNo)
		if err != nil {
			return err
		}
		if err = impl.prepareTrialConversion(ctx, facts, "missing-formal-period"); !bizerr.Is(err, CodePriceUnavailable) {
			t.Fatalf("unknown period=%v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='active' WHERE product_no='prd_renewal_ssl'`); err != nil {
		t.Fatal(err)
	}
	balance, err := billing.NewBalanceService(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(ctx, billing.ConfirmedRechargeInput{CustomerNo: principal.audience.CustomerNo,
		CurrencyCode: "CNY", AmountMinor: 50000, SourceReference: "conversion-test-recharge", SourceSnapshotDigest: strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
	finance, err := billing.NewAccountFinanceCommandServiceWithRenewals(database, principal, renewalPostgreSQLSettlementStub{}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	paidInvoice, err := finance.PayAccountInvoiceFromBalance(ctx, billing.AccountBalancePaymentInput{InvoiceNo: operation.InvoiceNo, IdempotencyKey: "generic-conversion-payment"})
	if err != nil || paidInvoice == nil || paidInvoice.StatusCode != "paid" {
		t.Fatalf("generic paid conversion=%+v err=%v", paidInvoice, err)
	}
	settlement, err := runtime.PayCurrentCustomer(ctx, PayInput{OperationNo: operation.OperationNo})
	if err != nil || settlement == nil || settlement.Operation.Status != "succeeded" {
		t.Fatalf("paid conversion=%+v err=%v", settlement, err)
	}
	paid, err := database.GetOne(ctx, `SELECT e.charge_type_code,e.status_code,e.expires_at,e.current_price_snapshot_no,
 i.settled_at FROM plugin_idc_core_service_entitlement e
 JOIN plugin_idc_core_billing_invoice i ON i.invoice_no=? WHERE e.entitlement_no=?`, operation.InvoiceNo, fixture.EntitlementNo)
	if err != nil {
		t.Fatal(err)
	}
	baseTime := paid["settled_at"].Time().UTC()
	if row["expires_at"].Time().After(baseTime) {
		baseTime = row["expires_at"].Time().UTC()
	}
	wantExpiry := baseTime.AddDate(0, 1, 0)
	if paid["charge_type_code"].String() != "recurring" || paid["status_code"].String() != expectedStatus ||
		paid["current_price_snapshot_no"].String() != quote.PriceSnapshotNo || paid["expires_at"].Time().Sub(wantExpiry).Abs() > time.Microsecond {
		t.Fatalf("paid facts=%v expiry=%s", paid, wantExpiry)
	}
	count, err = database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_async_task
 WHERE task_type='trial.entitlement.resume' AND subject_no=? AND customer_no=?`, fixture.EntitlementNo, principal.audience.CustomerNo)
	if err != nil || count.Int() != func() int {
		if active {
			return 0
		}
		return 1
	}() {
		t.Fatalf("missing durable recovery count=%v err=%v", count, err)
	}
	listed, err = runtime.ListTrialConversions(ctx, 1, 20)
	if err != nil || listed.Total != 0 || len(listed.Items) != 0 {
		t.Fatalf("converted trial still listed=%+v err=%v", listed, err)
	}
	replayPayment, err := runtime.PayCurrentCustomer(ctx, PayInput{OperationNo: operation.OperationNo})
	if err != nil || replayPayment == nil || !replayPayment.Reused {
		t.Fatalf("payment replay=%+v err=%v", replayPayment, err)
	}

	if !active {
		resource := &conversionRecoveryResource{status: lifecycleresourcecap.TaskStatusSucceeded, unavailable: true}
		worker, workerErr := triallifecycle.New(database, tasks, resource, conversionNoNotifications{})
		if workerErr != nil {
			t.Fatal(workerErr)
		}
		if workerErr = worker.Run(ctx, 8); workerErr == nil {
			t.Fatal("unavailable resource source must report a retryable observation failure")
		}
		state, stateErr := database.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_service_entitlement WHERE entitlement_no=?`, fixture.EntitlementNo)
		if stateErr != nil || state.String() != "suspended" || resource.creates != 1 {
			t.Fatalf("failed supplier changed state=%v creates=%d err=%v", state, resource.creates, stateErr)
		}
		resource.unavailable = false
		if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP-INTERVAL '1 minute' WHERE subject_no=? AND task_type='trial.entitlement.resume' AND status_code='retry_wait'`, fixture.EntitlementNo); err != nil {
			t.Fatal(err)
		}
		if workerErr = worker.Run(ctx, 8); workerErr != nil {
			t.Fatal(workerErr)
		}
		recovered, recoveryErr := database.GetOne(ctx, `SELECT status_code,expires_at FROM plugin_idc_core_service_entitlement WHERE entitlement_no=?`, fixture.EntitlementNo)
		if recoveryErr != nil || recovered["status_code"].String() != "active" || !recovered["expires_at"].Time().Equal(paid["expires_at"].Time()) {
			t.Fatalf("recovery=%v err=%v", recovered, recoveryErr)
		}
		if resource.request.Action != lifecycleresourcecap.ActionResume || resource.request.CustomerNo != originalCustomer || resource.request.ServiceNo != fixture.EntitlementNo {
			t.Fatalf("wrong supplier target=%+v", resource.request)
		}
	}

}

// conversionRecoveryResource controls only the external result; the payment,
// durable scheduling, retries and entitlement projection use production services.
type conversionRecoveryResource struct {
	status      lifecycleresourcecap.TaskStatus
	unavailable bool
	creates     int
	request     lifecycleresourcecap.TaskRequest
}

func (r *conversionRecoveryResource) CreateTask(_ context.Context, request lifecycleresourcecap.TaskRequest) (*lifecycleresourcecap.Task, error) {
	r.creates++
	r.request = request
	if r.unavailable {
		return nil, gerror.New("resource source temporarily unavailable")
	}
	return r.result(), nil
}
func (r *conversionRecoveryResource) ReadTask(_ context.Context, request lifecycleresourcecap.TaskReadRequest) (*lifecycleresourcecap.Task, error) {
	if request.Request != r.request {
		return nil, gerror.New("recovery request changed")
	}
	return r.result(), nil
}
func (r *conversionRecoveryResource) result() *lifecycleresourcecap.Task {
	result := &lifecycleresourcecap.Task{ProviderTaskNo: "provider-conversion-recovery", Status: r.status, CapabilityEvidenceNo: "conversion-capability", CapabilityEvidenceDigest: strings.Repeat("a", 64), EvidenceDigest: strings.Repeat("b", 64), ObservedAt: time.Now()}
	if r.status == lifecycleresourcecap.TaskStatusFailed {
		result.FailureCode = "provider.operation_failed"
	}
	return result
}

type conversionNoNotifications struct{}

func (conversionNoNotifications) Enqueue(context.Context, notificationcap.EventRequest) (notificationcap.EventReceipt, error) {
	return notificationcap.EventReceipt{}, gerror.New("formal conversion must not send an old trial reminder")
}
