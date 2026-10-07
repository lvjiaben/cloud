package operations

import (
	"context"
	"testing"
)

func TestEvaluateRiskUsesExplicitMerchantRule(t *testing.T) {
	limit := int64(1000)
	decision, err := EvaluateRisk(context.Background(), RiskRule{RuleNo: "risk-1", EventType: "refund", MaxAmountMinor: &limit, Action: "manual_review"}, RiskInput{EventType: "refund", AmountMinor: 1200})
	if err != nil || !decision.Matched || decision.Action != "manual_review" || decision.RuleNo != "risk-1" {
		t.Fatalf("unexpected risk decision: %+v err=%v", decision, err)
	}
	decision, err = EvaluateRisk(context.Background(), RiskRule{RuleNo: "risk-1", EventType: "refund", MaxAmountMinor: &limit, Action: "manual_review"}, RiskInput{EventType: "refund", AmountMinor: 900})
	if err != nil || decision.Matched || decision.Action != "allow" {
		t.Fatalf("unexpected allowed decision: %+v err=%v", decision, err)
	}
}
