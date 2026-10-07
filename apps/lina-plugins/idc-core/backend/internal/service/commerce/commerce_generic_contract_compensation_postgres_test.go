// This file verifies required-contract compensation against the generic
// fulfillment intent, task, delivery, and entitlement model in PostgreSQL.

package commerce

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	contractcompensationservice "lina-plugin-idc-core/backend/internal/service/contractcompensation"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

type contractCompensationAuditStub struct {
	err   error
	calls int
}

func (s *contractCompensationAuditStub) AppendBusinessTransition(
	_ context.Context,
	_ idccap.BusinessAuditTransition,
) error {
	s.calls++
	return s.err
}

func (s *contractCompensationAuditStub) AppendBusinessBatchTransition(
	_ context.Context,
	_ idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, s.err
}

func TestRequiredContractFailureCancelsOnlyGenericPreSupplierWorkPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioOptions(t, true, false)
	scenario := prepared.checkout
	if _, err := scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_commerce_order
SET requires_contract=TRUE,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND order_no=? AND status_code='paid'`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	audit := &contractCompensationAuditStub{}
	service, err := contractcompensationservice.New(scenario.database, audit)
	if err != nil {
		t.Fatal(err)
	}
	input := requiredContractFailureInput(prepared.order.OrderNo, "contract-generic-safe")
	result, err := service.CompensateContractFailure(scenario.ctx, input)
	if err != nil || result == nil || result.StatusCode != "refund_pending" ||
		result.RefundRequestNo == "" || result.Reused || audit.calls != 1 {
		t.Fatalf("generic safe compensation result=%+v audit=%d err=%v", result, audit.calls, err)
	}

	var evidence struct {
		OrderStatus, IntentStatus, TaskStatus, TransitionReason string
		RefundRequests, RefundEvents                            int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `
SELECT orders.status_code AS order_status,
       intent.status_code AS intent_status,
       task.status_code AS task_status,
       transition.reason_code AS transition_reason,
       (SELECT count(*) FROM plugin_idc_core_billing_refund_request request
         WHERE request.order_no=orders.order_no) AS refund_requests,
       (SELECT count(*) FROM plugin_idc_core_outbox_event event
         WHERE event.event_type='billing.refund.requested' AND event.aggregate_no=?) AS refund_events
FROM plugin_idc_core_commerce_order orders
JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.order_no=orders.order_no
JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=intent.intent_no
JOIN plugin_idc_core_product_fulfillment_system_transition transition
  ON transition.intent_no=intent.intent_no AND transition.event_code='cancelled'
WHERE orders.order_no=?`, result.RefundRequestNo, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	if evidence.OrderStatus != "refund_pending" || evidence.IntentStatus != "canceled" ||
		evidence.TaskStatus != "canceled" || evidence.TransitionReason != "required_contract_terminal" ||
		evidence.RefundRequests != 1 || evidence.RefundEvents != 1 {
		t.Fatalf("generic safe compensation evidence=%+v", evidence)
	}
	replayed, err := service.CompensateContractFailure(scenario.ctx, input)
	if err != nil || replayed == nil || !replayed.Reused || replayed.RefundRequestNo != result.RefundRequestNo || audit.calls != 1 {
		t.Fatalf("generic compensation replay=%+v audit=%d err=%v", replayed, audit.calls, err)
	}
}

func TestRequiredContractFailurePreservesCompletedEntitlementPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenario(t)
	scenario := prepared.checkout
	extensions := &fulfillmentWorkerExtensions{}
	contexts := &fulfillmentWorkerContexts{}
	sealer, err := productfulfillmentservice.NewProviderEvidenceSealer(
		"test-key-v1", []byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := productfulfillmentservice.NewWorker(
		scenario.database, extensions, contexts, sealer, fulfillmentSystemReasons(t), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first, runErr := worker.Run(scenario.ctx, 1); runErr != nil || first.Selected != 1 || first.Scheduled != 1 {
		t.Fatalf("pre-submit query=%+v err=%v", first, runErr)
	}
	advanceFulfillmentTaskClock(t, scenario, prepared.order.OrderNo)
	if second, runErr := worker.Run(scenario.ctx, 1); runErr != nil || second.Selected != 1 || second.Completed != 1 {
		t.Fatalf("provider completion=%+v err=%v", second, runErr)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_commerce_order
SET requires_contract=TRUE,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND order_no=? AND status_code='paid'`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	audit := &contractCompensationAuditStub{}
	service, err := contractcompensationservice.New(scenario.database, audit)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CompensateContractFailure(
		scenario.ctx,
		requiredContractFailureInput(prepared.order.OrderNo, "contract-generic-completed"),
	)
	if err != nil || result == nil || result.StatusCode != "manual_review" ||
		result.ReasonCode != "fulfillment_started_or_unknown" || result.RefundRequestNo != "" {
		t.Fatalf("completed entitlement compensation=%+v err=%v", result, err)
	}
	var evidence struct {
		OrderStatus, IntentStatus, TaskStatus, EntitlementStatus string
		RefundRequests, RefundEvents                             int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `
SELECT orders.status_code AS order_status,intent.status_code AS intent_status,
       task.status_code AS task_status,entitlement.status_code AS entitlement_status,
       (SELECT count(*) FROM plugin_idc_core_billing_refund_request request
         WHERE request.order_no=orders.order_no) AS refund_requests,
       (SELECT count(*) FROM plugin_idc_core_outbox_event event
         WHERE event.event_type='billing.refund.requested' AND event.correlation_no=?) AS refund_events
FROM plugin_idc_core_commerce_order orders
JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.order_no=orders.order_no
JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=intent.intent_no
JOIN plugin_idc_core_service_entitlement entitlement ON entitlement.source_intent_no=intent.intent_no
WHERE orders.order_no=?`, result.CompensationNo, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	if evidence.OrderStatus != "paid" || evidence.IntentStatus != "completed" ||
		evidence.TaskStatus != "succeeded" || evidence.EntitlementStatus != "active" ||
		evidence.RefundRequests != 0 || evidence.RefundEvents != 0 {
		t.Fatalf("completed entitlement was changed or refunded: %+v", evidence)
	}
}

func TestRequiredContractFailureAuditFailureRollsBackGenericStopPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenarioOptions(t, true, false)
	scenario := prepared.checkout
	if _, err := scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_commerce_order SET requires_contract=TRUE,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND order_no=? AND status_code='paid'`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	service, err := contractcompensationservice.New(
		scenario.database,
		&contractCompensationAuditStub{err: errors.New("audit unavailable")},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CompensateContractFailure(
		scenario.ctx,
		requiredContractFailureInput(prepared.order.OrderNo, "contract-generic-rollback"),
	); err == nil || !strings.Contains(err.Error(), "audit unavailable") {
		t.Fatalf("audit failure must abort compensation, got %v", err)
	}
	var evidence struct {
		OrderStatus, IntentStatus, TaskStatus string
		Compensations, RefundRequests         int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `
SELECT orders.status_code AS order_status,intent.status_code AS intent_status,
       task.status_code AS task_status,
       (SELECT count(*) FROM plugin_idc_core_contract_failure_compensation compensation
         WHERE compensation.order_no=orders.order_no) AS compensations,
       (SELECT count(*) FROM plugin_idc_core_billing_refund_request request
         WHERE request.order_no=orders.order_no) AS refund_requests
FROM plugin_idc_core_commerce_order orders
JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.order_no=orders.order_no
JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=intent.intent_no
WHERE orders.order_no=?`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	if evidence.OrderStatus != "paid" || evidence.IntentStatus != "queued" ||
		evidence.TaskStatus != "pending_query" || evidence.Compensations != 0 || evidence.RefundRequests != 0 {
		t.Fatalf("audit failure did not roll back all generic changes: %+v", evidence)
	}
}

func requiredContractFailureInput(orderNo, suffix string) contractcap.FailureCompensationInput {
	return contractcap.FailureCompensationInput{
		ContractNo:       suffix,
		OrderNo:          orderNo,
		TerminalEventNo:  "event-" + suffix,
		TerminalStatus:   "expired",
		PolicyNo:         "policy-contract-failure",
		PolicyVersion:    1,
		HandlingDeadline: time.Now().UTC().Add(-time.Minute),
		TriggerCode:      "deadline",
	}
}
