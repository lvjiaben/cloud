package productfulfillment

import (
	"math"
	"testing"
)

func TestIntentNumberedPageBounds(t *testing.T) {
	for _, input := range []ListInput{{Page: -1}, {Page: math.MaxInt, Limit: 100}, {Page: 1, Cursor: "old-cursor"}, {Page: 1, Limit: 101}} {
		if _, err := normalizeIntentListInput(input); err == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
	for _, size := range []int{10, 20, 50, 100} {
		if _, err := normalizeIntentListInput(ListInput{Page: 3, Limit: size}); err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
	}
}

func TestSafeIntentStatusDistinguishesManualAttention(t *testing.T) {
	tests := map[string]string{
		"review_pending": "awaiting_review",
		"manual_pending": "manual_processing",
		"gate_pending":   "processing",
		"queued":         "processing",
		"processing":     "processing",
		"completed":      "completed",
		"incomplete":     "not_completed",
		"canceled":       "cancelled",
	}
	for internal, expected := range tests {
		if actual := safeIntentStatus(internal); actual != expected {
			t.Fatalf("status %q projected as %q, want %q", internal, actual, expected)
		}
	}
}

func TestSafeEventContractUsesOnlyGovernedCodes(t *testing.T) {
	tests := map[string]string{
		"review_approved":           "processing",
		"review_rejected":           "processing",
		"automation_started":        "processing",
		"automation_processing":     "processing",
		"automation_completed":      "completed",
		"automation_failed":         "not_completed",
		"moved_to_manual":           "processing",
		"taken_over":                "processing",
		"manual_completed":          "completed",
		"manual_unable_to_complete": "not_completed",
		"cancelled":                 "cancelled",
	}
	for event, expected := range tests {
		if actual := safeCommandStatus(event); actual != expected {
			t.Fatalf("event %q projected as %q, want %q", event, actual, expected)
		}
	}
	if actual := safeCommandStatus("provider_internal_status"); actual != "" {
		t.Fatalf("unknown event was exposed as %q", actual)
	}
}

func TestManualConclusionOutcomeIsClosedSet(t *testing.T) {
	if actionConcludeCompleted == actionConcludeUnable || permissionForAction(actionConcludeCompleted) != PermissionManualConclude || permissionForAction(actionConcludeUnable) != PermissionManualConclude {
		t.Fatal("manual conclusion actions must remain distinct and precisely authorized")
	}
}
