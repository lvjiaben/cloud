// This file verifies the fail-closed safety gate before any paid-order
// compensation mutation is allowed.

package contractcompensation

import (
	"testing"
	"time"
)

func TestAutomaticSafetyPlanRejectsStartedFulfillment(t *testing.T) {
	now := time.Now().UTC()
	plan, reason := automaticSafetyPlan(
		&orderRow{RequiresContract: true, PaidAt: &now, StatusCode: "paid"},
		&invoiceRow{
			StatusCode: "paid", PaidAmountMinor: 100,
			RefundableAmountMinor: 100,
		},
		[]fulfillmentIntentRow{{IntentNo: "intent-1", StatusCode: "processing"}},
		[]fulfillmentTaskRow{{
			TaskNo: "task-1", IntentNo: "intent-1", StatusCode: "running",
			SubmitAttempts: 1,
		}},
		[]reservationRow{{StatusCode: "committed"}},
		nil,
	)
	if reason != "fulfillment_started_or_unknown" {
		t.Fatalf("expected started fulfillment to require manual review, got %q", reason)
	}
	if len(plan.ManualIntents) != 1 || plan.ManualIntents[0] != "intent-1" ||
		len(plan.ManualTasks) != 1 || plan.ManualTasks[0] != "task-1" {
		t.Fatalf("started fulfillment must be contained for manual handling: %+v", plan)
	}
}

func TestAutomaticSafetyPlanCancelsOnlyPreSupplierWork(t *testing.T) {
	now := time.Now().UTC()
	plan, reason := automaticSafetyPlan(
		&orderRow{RequiresContract: true, PaidAt: &now, StatusCode: "paid"},
		&invoiceRow{StatusCode: "paid", PaidAmountMinor: 100, RefundableAmountMinor: 100},
		[]fulfillmentIntentRow{{IntentNo: "intent-1", StatusCode: "queued"}},
		[]fulfillmentTaskRow{{TaskNo: "task-1", IntentNo: "intent-1", StatusCode: "pending_query"}},
		[]reservationRow{{StatusCode: "committed"}}, nil,
	)
	if reason != "" || len(plan.CancelIntents) != 1 || len(plan.CancelTasks) != 1 ||
		len(plan.ManualIntents) != 0 || len(plan.RequestTakeoverTasks) != 0 {
		t.Fatalf("pre-supplier fulfillment should be safely cancellable: plan=%+v reason=%q", plan, reason)
	}
}

func TestAutomaticSafetyPlanContainsPreSupplierWorkWhenRefundIsUnsafe(t *testing.T) {
	now := time.Now().UTC()
	plan, reason := automaticSafetyPlan(
		&orderRow{RequiresContract: true, PaidAt: &now, StatusCode: "paid"},
		&invoiceRow{StatusCode: "paid", PaidAmountMinor: 100, RefundableAmountMinor: 50},
		[]fulfillmentIntentRow{{IntentNo: "intent-1", StatusCode: "queued"}},
		[]fulfillmentTaskRow{{TaskNo: "task-1", IntentNo: "intent-1", StatusCode: "pending_query"}},
		[]reservationRow{{StatusCode: "committed"}},
		[]entitlementRow{{EntitlementNo: "entitlement-1", IntentNo: "intent-completed", StatusCode: "active"}},
	)
	if reason != "financial_state_not_refundable" {
		t.Fatalf("unsafe refund must require manual review, got %q", reason)
	}
	if len(plan.CancelIntents) != 1 || plan.CancelIntents[0] != "intent-1" ||
		len(plan.CancelTasks) != 1 || plan.CancelTasks[0] != "task-1" {
		t.Fatalf("manual refund review must still contain untouched fulfillment: %+v", plan)
	}
}
