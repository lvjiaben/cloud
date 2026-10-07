package billing

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/asynctask"
)

func TestCreditReconciliationCreatesOneBoundedDurableAlertForProjectionDrift(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	account := activateCreditAccountForUsageTest(
		t, ctx, database, "customer-ledger", 8_000, "reconciliation",
	)
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatalf("create reconciliation task service: %v", err)
	}
	service, err := NewCreditReconciliationService(database, tasks)
	if err != nil {
		t.Fatalf("create credit reconciliation service: %v", err)
	}

	clean, err := service.ScanCreditReconciliation(ctx, "", 100)
	if err != nil {
		t.Fatalf("scan clean credit accounts: %v", err)
	}
	if clean.ScannedCount != 1 || len(clean.Items) != 0 || clean.AlertTaskNo != "" {
		t.Fatalf("clean credit scan produced an alert: %+v", clean)
	}

	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_exposure (
    tenant_id, exposure_no, credit_account_no, customer_no, currency_code,
    source_type_code, source_reference, original_principal_minor,
    unbilled_principal_minor, status_code, source_snapshot_digest,
    content_digest, version
) VALUES (0, 'credit-reconciliation-drift', ?, 'customer-ledger', 'CNY',
          'manual', 'credit-reconciliation-drift', 1, 1, 'unbilled',
          repeat('1', 64), repeat('2', 64), 1)`, account.CreditAccountNo); err != nil {
		t.Fatalf("seed cross-projection credit drift: %v", err)
	}

	alerted, err := service.ScanCreditReconciliation(ctx, "", 100)
	if err != nil {
		t.Fatalf("scan drifted credit account: %v", err)
	}
	if alerted.Reused || alerted.ScannedCount != 1 || len(alerted.Items) != 1 ||
		alerted.AlertTaskNo == "" || alerted.Items[0].CreditAccountNo != account.CreditAccountNo ||
		alerted.Items[0].ProjectedUnbilledMinor != 0 ||
		alerted.Items[0].ExposureUnbilledMinor != 1 ||
		!containsCreditDiscrepancy(alerted.Items[0].DiscrepancyCodes,
			"exposure_unbilled_minor") {
		t.Fatalf("unexpected credit reconciliation alert: %+v", alerted)
	}
	replayed, err := service.ScanCreditReconciliation(ctx, "", 100)
	if err != nil || !replayed.Reused || replayed.AlertTaskNo != alerted.AlertTaskNo ||
		len(replayed.Items) != 1 {
		t.Fatalf("credit reconciliation replay=%+v err=%v", replayed, err)
	}

	var taskFacts struct {
		TaskCount int64  `orm:"task_count"`
		TaskType  string `orm:"task_type"`
		Status    string `orm:"status_code"`
		Priority  int    `orm:"priority"`
		Payload   string `orm:"payload"`
	}
	if err = database.GetScan(ctx, &taskFacts, `
SELECT COUNT(*) AS task_count, MAX(task_type) AS task_type,
       MAX(status_code) AS status_code, MAX(priority) AS priority,
       MAX(payload::text) AS payload
FROM plugin_idc_core_async_task
WHERE task_no = ?`, alerted.AlertTaskNo); err != nil {
		t.Fatalf("load credit reconciliation task: %v", err)
	}
	if taskFacts.TaskCount != 1 || taskFacts.TaskType != creditReconciliationTaskType ||
		taskFacts.Status != "pending" || taskFacts.Priority != 100 {
		t.Fatalf("unexpected credit reconciliation task facts: %+v", taskFacts)
	}
	var payload creditReconciliationAlertPayload
	if err = json.Unmarshal([]byte(taskFacts.Payload), &payload); err != nil {
		t.Fatalf("decode credit reconciliation task payload: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].CreditAccountNo != account.CreditAccountNo ||
		payload.Fingerprint == "" || strings.Contains(taskFacts.Payload, "customer-ledger") {
		t.Fatalf("unexpected or non-redacted reconciliation payload: %+v", payload)
	}
}

func TestCreditReconciliationRejectsUnboundedScan(t *testing.T) {
	t.Parallel()
	service := &creditReconciliationServiceImpl{}
	if _, err := service.ScanCreditReconciliation(
		context.Background(), "", maximumCreditReconciliationBatch+1,
	); err == nil {
		t.Fatal("credit reconciliation accepted an unbounded page")
	}
}

func containsCreditDiscrepancy(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
