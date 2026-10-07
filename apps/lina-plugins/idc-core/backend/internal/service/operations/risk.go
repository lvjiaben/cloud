package operations

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
)

// RiskRule is always supplied by the merchant policy. No hidden threshold is
// baked into the service, so enabling a rule remains an explicit business
// decision in the management UI.
type RiskRule struct {
	RuleNo         string
	EventType      string
	MaxAmountMinor *int64
	MaxEvents      int
	Action         string
}

type RiskInput struct {
	EventType   string
	AmountMinor int64
	EventCount  int
}
type RiskDecision struct {
	Matched  bool
	Action   string
	RuleNo   string
	Evidence string
}

func EvaluateRisk(ctx context.Context, rule RiskRule, input RiskInput) (RiskDecision, error) {
	if ctx == nil {
		return RiskDecision{}, gerror.New("risk context is required")
	}
	if err := ctx.Err(); err != nil {
		return RiskDecision{}, err
	}
	if rule.RuleNo == "" || rule.EventType == "" || input.EventType == "" || input.AmountMinor < 0 || input.EventCount < 0 {
		return RiskDecision{}, gerror.New("risk rule input is invalid")
	}
	if rule.EventType != input.EventType {
		return RiskDecision{Action: "allow"}, nil
	}
	matched := (rule.MaxAmountMinor != nil && input.AmountMinor > *rule.MaxAmountMinor) || (rule.MaxEvents > 0 && input.EventCount > rule.MaxEvents)
	if !matched {
		return RiskDecision{Action: "allow", RuleNo: rule.RuleNo}, nil
	}
	if rule.Action != "remind" && rule.Action != "restrict" && rule.Action != "delay" && rule.Action != "manual_review" && rule.Action != "allow" {
		return RiskDecision{}, gerror.New("risk rule action is invalid")
	}
	return RiskDecision{Matched: true, Action: rule.Action, RuleNo: rule.RuleNo, Evidence: "configured_threshold_exceeded"}, nil
}
