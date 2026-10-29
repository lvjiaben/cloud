package commerce

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestGenericEntitlementRefundFreezesQuantityAndEntitlementPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioOptionsWithQuantity(t, true, true, 2)
	scenario := prepared.checkout
	const reviewerID = 9010
	management, err := billingservice.NewManagementRefundService(
		scenario.database, refundManagementAuthorizerStub{actor: reviewerID},
		&refundRecentAuthStub{actor: reviewerID}, refundBusinessAuditStub{},
		newFinancialSettlementCreditService(t, scenario),
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
		IntentNo    string `orm:"intent_no"`
		OrderItemNo string `orm:"order_item_no"`
	}
	if err := scenario.database.GetScan(scenario.ctx, &intent, `
SELECT intent_no,order_item_no
FROM plugin_idc_core_product_fulfillment_intent
WHERE order_no=? ORDER BY intent_no LIMIT 1`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{sql: `ALTER TABLE plugin_idc_core_product_fulfillment_intent DISABLE TRIGGER trg_idc_guard_fulfillment_intent`},
		{sql: `ALTER TABLE plugin_idc_core_product_fulfillment_intent DISABLE TRIGGER trg_idc_check_fulfillment_intent_terminal`},
		{sql: `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='completed' WHERE intent_no=?`, args: []any{intent.IntentNo}},
		{sql: `ALTER TABLE plugin_idc_core_product_fulfillment_intent ENABLE TRIGGER trg_idc_guard_fulfillment_intent`},
		{sql: `ALTER TABLE plugin_idc_core_product_fulfillment_intent ENABLE TRIGGER trg_idc_check_fulfillment_intent_terminal`},
	}
	for _, statement := range statements {
		if _, err := scenario.database.Exec(scenario.ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("complete generic fulfillment: %v", err)
		}
	}
	var target struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		EntitlementNo string `orm:"entitlement_no"`
		AmountMinor   int64  `orm:"amount_minor"`
		Quantity      int    `orm:"quantity"`
	}
	if err := scenario.database.GetScan(scenario.ctx, &target, `
SELECT invoice_item.invoice_item_no,entitlement.entitlement_no,
       invoice_item.total_amount_minor AS amount_minor,quantity.quantity
FROM plugin_idc_core_billing_invoice_item invoice_item
JOIN plugin_idc_core_commerce_order_item_quantity_snapshot quantity
  ON quantity.order_item_no=invoice_item.order_item_no
JOIN plugin_idc_core_service_entitlement entitlement
  ON entitlement.order_item_no=invoice_item.order_item_no
WHERE invoice_item.invoice_no=?`, prepared.order.InvoiceNo); err != nil {
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
	_, err = refunds.PreviewOnlineRefund(scenario.ctx, billingservice.RefundPreviewInput{
		OrderNo: prepared.order.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: target.InvoiceItemNo,
			Quantity:      target.Quantity + 1, AmountMinor: target.AmountMinor,
		}},
	})
	if !bizerr.Is(err, billingservice.CodeRefundNotEligible) {
		t.Fatalf("quantity above frozen order quantity error=%v", err)
	}
	input := billingservice.RefundPreviewInput{
		OrderNo: prepared.order.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: target.InvoiceItemNo, EntitlementNos: []string{target.EntitlementNo},
			Quantity: 1, AmountMinor: target.AmountMinor,
		}},
	}
	mismatchedQuantity := input
	mismatchedQuantity.Selections = append([]billingservice.RefundSelectionInput(nil), input.Selections...)
	mismatchedQuantity.Selections[0].Quantity = 2
	partialImpactPreview, err := refunds.PreviewOnlineRefund(scenario.ctx, mismatchedQuantity)
	if err != nil || partialImpactPreview == nil || len(partialImpactPreview.Items) != 1 ||
		partialImpactPreview.Items[0].Quantity != 2 || len(partialImpactPreview.Items[0].Impacts) != 1 {
		t.Fatalf("order-item quantity must stay independent from optional entitlement impact: preview=%+v err=%v", partialImpactPreview, err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
ALTER TABLE plugin_idc_core_service_entitlement DISABLE TRIGGER ALL`); err != nil {
		t.Fatal(err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_service_entitlement SET customer_no='customer-commerce-other'
WHERE entitlement_no=?`, target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if _, err = refunds.PreviewOnlineRefund(scenario.ctx, input); !bizerr.Is(err, billingservice.CodeRefundNotAccessible) {
		t.Fatalf("cross-customer entitlement error=%v", err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_service_entitlement
SET customer_no='customer-commerce-service-1',order_item_no='order-item-other'
WHERE entitlement_no=?`, target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if _, err = refunds.PreviewOnlineRefund(scenario.ctx, input); !bizerr.Is(err, billingservice.CodeRefundNotAccessible) {
		t.Fatalf("cross-order-item entitlement error=%v", err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_service_entitlement SET order_item_no=? WHERE entitlement_no=?`,
		intent.OrderItemNo, target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
ALTER TABLE plugin_idc_core_service_entitlement ENABLE TRIGGER ALL`); err != nil {
		t.Fatal(err)
	}
	queryLogger := &checkoutBenchmarkQueryLogger{ILogger: scenario.database.GetLogger(), counts: make(map[int]int)}
	previousLogger, previousDebug := scenario.database.GetLogger(), scenario.database.GetDebug()
	scenario.database.SetLogger(queryLogger)
	scenario.database.SetDebug(true)
	t.Cleanup(func() {
		scenario.database.SetDebug(previousDebug)
		scenario.database.SetLogger(previousLogger)
	})
	orderItemOnly := input
	orderItemOnly.Selections = append([]billingservice.RefundSelectionInput(nil), input.Selections...)
	orderItemOnly.Selections[0].EntitlementNos = nil
	if _, err = refunds.PreviewOnlineRefund(
		context.WithValue(scenario.ctx, checkoutBenchmarkSampleKey{}, 9301), orderItemOnly,
	); err != nil {
		t.Fatalf("order-item-only preview: %v", err)
	}
	if _, err = refunds.PreviewOnlineRefund(
		context.WithValue(scenario.ctx, checkoutBenchmarkSampleKey{}, 9302), input,
	); err != nil {
		t.Fatalf("order item with entitlement preview: %v", err)
	}
	if withoutEntitlements, withEntitlements := queryLogger.count(9301), queryLogger.count(9302); withoutEntitlements != withEntitlements {
		t.Fatalf("refund preview query count grew with entitlement selection: without=%d with=%d", withoutEntitlements, withEntitlements)
	}
	preview, err := refunds.PreviewOnlineRefund(scenario.ctx, input)
	if err != nil || preview == nil || len(preview.Items) != 1 ||
		preview.Items[0].Quantity != 1 ||
		len(preview.Items[0].Impacts) != 1 ||
		preview.Items[0].Impacts[0].EntitlementNo != target.EntitlementNo {
		t.Fatalf("generic entitlement preview=%+v err=%v", preview, err)
	}
	refund, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest: preview.ImpactDigest, Reason: billingservice.RefundReasonNoLongerNeeded,
		IdempotencyKey: "generic-entitlement-refund-0001", Preview: input,
	})
	if err != nil || refund == nil {
		t.Fatalf("submit generic entitlement refund=%+v err=%v", refund, err)
	}
	var frozen struct {
		Quantity int `orm:"quantity"`
		Impacts  int `orm:"impacts"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &frozen, `
SELECT item.quantity,
       (SELECT count(*) FROM plugin_idc_core_refund_entitlement_impact impact
        WHERE impact.refund_item_no=item.refund_item_no) AS impacts
FROM plugin_idc_core_refund_request_item item WHERE item.refund_no=?`, refund.RefundNo); err != nil {
		t.Fatal(err)
	}
	if frozen.Quantity != 1 || frozen.Impacts != 1 {
		t.Fatalf("generic refund snapshot=%+v", frozen)
	}
	if _, err = refunds.PreviewOnlineRefund(scenario.ctx, input); !bizerr.Is(err, billingservice.CodeRefundNotEligible) {
		t.Fatalf("duplicate entitlement refund error=%v", err)
	}

	approved, err := management.ReviewRefund(scenario.ctx, billingservice.ManagementRefundReviewIntent{
		RequestNo: "generic-entitlement-refund-approve-0001", RefundNo: refund.RefundNo,
		ExpectedVersion: refund.Version, Decision: billingservice.RefundReviewApprove,
		EvidenceDigest: strings.Repeat("a", 64),
	}, "")
	if err != nil || approved == nil || approved.Status != "approved" {
		t.Fatalf("approve generic entitlement refund=%+v err=%v", approved, err)
	}
	localWorker, err := newLocalRefundWorkerTestService(t, scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := localWorker.Run(scenario.ctx, 20); runErr != nil || processed != 1 {
		t.Fatalf("post generic entitlement refund processed=%d err=%v", processed, runErr)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_async_task SET available_at=CURRENT_TIMESTAMP-INTERVAL '1 minute' WHERE task_type='refund_entitlement_termination' AND subject_no=?`, target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	var taskEvidence struct {
		Count           int64  `orm:"count"`
		Status          string `orm:"status"`
		Ready           bool   `orm:"ready"`
		AttemptCount    int    `orm:"attempt_count"`
		MaximumAttempts int    `orm:"max_attempts"`
		Claimable       int64  `orm:"claimable"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &taskEvidence, `SELECT COUNT(*) AS count,COALESCE(MAX(status_code),'') AS status,COALESCE(BOOL_AND(available_at<=CURRENT_TIMESTAMP),FALSE) AS ready,COALESCE(MAX(attempt_count),0) AS attempt_count,COALESCE(MAX(max_attempts),0) AS max_attempts,COUNT(*) FILTER(WHERE tenant_id=0 AND status_code IN('pending','retry_wait') AND attempt_count<max_attempts AND available_at<=CURRENT_TIMESTAMP AND (deadline_at IS NULL OR deadline_at>CURRENT_TIMESTAMP)) AS claimable FROM plugin_idc_core_async_task WHERE task_type='refund_entitlement_termination' AND subject_no=?`, target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if taskEvidence.Count != 1 || taskEvidence.Status != "pending" ||
		!taskEvidence.Ready || taskEvidence.AttemptCount != 0 ||
		taskEvidence.MaximumAttempts != 3 || taskEvidence.Claimable != 1 {
		t.Fatalf("generic refund entitlement task evidence=%+v", taskEvidence)
	}
	assertRejectedTermination := func(status string, version int64) {
		t.Helper()
		txErr := scenario.database.Transaction(scenario.ctx, func(txCtx context.Context, _ gdb.TX) error {
			if _, updateErr := scenario.database.Exec(txCtx, `
UPDATE plugin_idc_core_service_entitlement
SET status_code=?,version=?
WHERE tenant_id=0 AND entitlement_no=?`, status, version, target.EntitlementNo); updateErr != nil {
				return updateErr
			}
			_, updateErr := scenario.database.Exec(txCtx, `
UPDATE plugin_idc_core_refund_entitlement_lock
SET released_at=CURRENT_TIMESTAMP,release_reason_code='succeeded'
WHERE tenant_id=0 AND refund_no=? AND entitlement_no=?`, refund.RefundNo, target.EntitlementNo)
			return updateErr
		})
		if txErr == nil || !strings.Contains(txErr.Error(), "refund composition does not match request") {
			t.Fatalf("unauthorized generic entitlement termination status=%s version=%d error=%v", status, version, txErr)
		}
		var persisted struct {
			StatusCode  string `orm:"status_code"`
			Version     int64  `orm:"version"`
			ActiveLocks int64  `orm:"active_locks"`
		}
		if loadErr := scenario.database.GetScan(scenario.ctx, &persisted, `
SELECT entitlement.status_code,entitlement.version,
       (SELECT COUNT(*) FROM plugin_idc_core_refund_entitlement_lock entitlement_lock
        WHERE entitlement_lock.entitlement_no=entitlement.entitlement_no
          AND entitlement_lock.released_at IS NULL) AS active_locks
FROM plugin_idc_core_service_entitlement entitlement
WHERE entitlement.entitlement_no=?`, target.EntitlementNo); loadErr != nil {
			t.Fatal(loadErr)
		}
		if persisted.StatusCode != "active" || persisted.Version != 1 || persisted.ActiveLocks != 1 {
			t.Fatalf("unauthorized generic entitlement termination was not rolled back: %+v", persisted)
		}
	}
	assertRejectedTermination("suspended", 1)
	assertRejectedTermination("terminated", 3)
	schema, err := scenario.database.GetValue(scenario.ctx, `SELECT current_schema()`)
	if err != nil {
		t.Fatal(err)
	}
	originalDatabaseConfig := gdb.GetAllConfig()
	t.Cleanup(func() {
		if restoreErr := gdb.SetConfig(originalDatabaseConfig); restoreErr != nil {
			t.Errorf("restore generic refund task database config: %v", restoreErr)
		}
	})
	taskDSN := strings.TrimSpace(os.Getenv(commerceIntegrationDSNEnv)) +
		"&search_path=" + schema.String()
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{Link: taskDSN}},
	}); err != nil {
		t.Fatal(err)
	}
	configuredSchema, err := g.DB().GetValue(scenario.ctx, `SELECT current_schema()`)
	if err != nil || configuredSchema.String() != schema.String() {
		t.Fatalf("refund task database schema=%q want=%q err=%v", configuredSchema.String(), schema.String(), err)
	}
	daoSchema, err := dao.AsyncTask.DB().GetValue(scenario.ctx, `SELECT current_schema()`)
	if err != nil || daoSchema.String() != schema.String() {
		t.Fatalf("refund task DAO schema=%q want=%q err=%v", daoSchema.String(), schema.String(), err)
	}
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatal(err)
	}
	worker, err := billingservice.NewRefundEntitlementWorker(scenario.database, tasks)
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := worker.Run(scenario.ctx, 20); runErr != nil || processed != 1 {
		t.Fatalf("terminate generic refund entitlement processed=%d task=%+v err=%v", processed, taskEvidence, runErr)
	}
	var terminated struct {
		StatusCode        string `orm:"status_code"`
		Version           int64  `orm:"version"`
		ActiveLocks       int64  `orm:"active_locks"`
		SucceededTasks    int64  `orm:"succeeded_tasks"`
		SucceededAttempts int64  `orm:"succeeded_attempts"`
		ResultSnapshot    string `orm:"result_snapshot"`
		TaskNo            string `orm:"task_no"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &terminated, `
SELECT entitlement.status_code,entitlement.version,
       (SELECT COUNT(*) FROM plugin_idc_core_refund_entitlement_lock entitlement_lock
        WHERE entitlement_lock.entitlement_no=entitlement.entitlement_no
          AND entitlement_lock.released_at IS NULL) AS active_locks,
       (SELECT COUNT(*) FROM plugin_idc_core_async_task task
        WHERE task.task_type='refund_entitlement_termination'
          AND task.subject_no=entitlement.entitlement_no AND task.status_code='succeeded') AS succeeded_tasks,
       (SELECT COUNT(*) FROM plugin_idc_core_async_task_attempt attempt
        JOIN plugin_idc_core_async_task task ON task.task_no=attempt.task_no
        WHERE task.task_type='refund_entitlement_termination'
          AND task.subject_no=entitlement.entitlement_no AND attempt.status_code='succeeded') AS succeeded_attempts,
       (SELECT task.result_snapshot FROM plugin_idc_core_async_task task
        WHERE task.task_type='refund_entitlement_termination'
          AND task.subject_no=entitlement.entitlement_no LIMIT 1) AS result_snapshot,
       (SELECT task.task_no FROM plugin_idc_core_async_task task
        WHERE task.task_type='refund_entitlement_termination'
          AND task.subject_no=entitlement.entitlement_no LIMIT 1) AS task_no
FROM plugin_idc_core_service_entitlement entitlement
WHERE entitlement.entitlement_no=?`, target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	var terminationEvidence map[string]any
	if err = json.Unmarshal([]byte(terminated.ResultSnapshot), &terminationEvidence); err != nil {
		t.Fatalf("decode generic entitlement termination evidence: %v", err)
	}
	if terminated.StatusCode != "terminated" || terminated.Version != 2 ||
		terminated.ActiveLocks != 0 || terminated.SucceededTasks != 1 ||
		terminated.SucceededAttempts != 1 || terminationEvidence["resultStatus"] != "terminated" ||
		terminationEvidence["refundLockReleased"] != true {
		t.Fatalf("generic entitlement termination evidence=%+v", terminated)
	}

	// Entitlement, refund lock, task, and attempt now commit in one transaction.
	// A second worker pass must therefore find no claimable work and must not
	// mutate the already terminated entitlement.
	if processed, runErr := worker.Run(scenario.ctx, 20); runErr != nil || processed != 0 {
		t.Fatalf("replay generic refund entitlement processed=%d err=%v", processed, runErr)
	}
	var replay struct {
		Version        int64  `orm:"version"`
		AttemptCount   int64  `orm:"attempt_count"`
		ResultSnapshot string `orm:"result_snapshot"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &replay, `
SELECT entitlement.version,task.attempt_count,task.result_snapshot
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_async_task task ON task.subject_no=entitlement.entitlement_no
WHERE entitlement.entitlement_no=? AND task.task_no=?`, target.EntitlementNo, terminated.TaskNo); err != nil {
		t.Fatal(err)
	}
	var replayEvidence map[string]any
	if err = json.Unmarshal([]byte(replay.ResultSnapshot), &replayEvidence); err != nil {
		t.Fatalf("decode generic entitlement replay evidence: %v", err)
	}
	if replay.Version != 2 || replay.AttemptCount != 1 || replayEvidence["resultStatus"] != "terminated" {
		t.Fatalf("generic entitlement termination replay=%+v", replay)
	}
}
