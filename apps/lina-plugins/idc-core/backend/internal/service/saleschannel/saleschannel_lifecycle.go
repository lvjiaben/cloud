package saleschannel

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// MaturityRequest is the small immutable projection needed by the maturity
// worker. The worker persists the returned decision as a new evidence row.
type MaturityRequest struct {
	Now                 time.Time
	MaturesAt           time.Time
	PendingAmountMinor  int64
	RefundedAmountMinor int64
	RiskBlocked         bool
}

type MaturityDecision struct {
	StatusCode  string
	AmountMinor int64
	ReasonCode  string
}

const (
	MaturityMatured  = "matured"
	MaturityBlocked  = "blocked"
	MaturityReversed = "reversed"
	MaturityNotDue   = "not_due"
)

func (s *serviceImpl) DecideMaturity(ctx context.Context, request MaturityRequest) (MaturityDecision, error) {
	if ctx == nil {
		return MaturityDecision{}, gerror.New("maturity context is required")
	}
	if err := ctx.Err(); err != nil {
		return MaturityDecision{}, gerror.Wrap(err, "maturity context is unavailable")
	}
	if request.Now.IsZero() || request.MaturesAt.IsZero() || request.PendingAmountMinor < 0 || request.RefundedAmountMinor < 0 || request.RefundedAmountMinor > request.PendingAmountMinor {
		return MaturityDecision{}, gerror.New("maturity request is invalid")
	}
	if request.Now.Before(request.MaturesAt) {
		return MaturityDecision{StatusCode: MaturityNotDue, AmountMinor: request.PendingAmountMinor}, nil
	}
	remaining := request.PendingAmountMinor - request.RefundedAmountMinor
	if remaining == 0 {
		return MaturityDecision{StatusCode: MaturityReversed, ReasonCode: "fully_refunded"}, nil
	}
	if request.RiskBlocked {
		return MaturityDecision{StatusCode: MaturityBlocked, AmountMinor: remaining, ReasonCode: "risk_blocked"}, nil
	}
	return MaturityDecision{StatusCode: MaturityMatured, AmountMinor: remaining}, nil
}

type SettlementRequest struct {
	AvailableAmountMinor int64
	RequestedAmountMinor int64
	MinimumAmountMinor   int64
	PayeeVerified        bool
}

type SettlementDecision struct {
	Allowed           bool
	FrozenAmountMinor int64
	ReasonCode        string
}

func (s *serviceImpl) DecideSettlement(ctx context.Context, request SettlementRequest) (SettlementDecision, error) {
	if ctx == nil {
		return SettlementDecision{}, gerror.New("settlement context is required")
	}
	if err := ctx.Err(); err != nil {
		return SettlementDecision{}, gerror.Wrap(err, "settlement context is unavailable")
	}
	if request.AvailableAmountMinor < 0 || request.RequestedAmountMinor <= 0 || request.MinimumAmountMinor < 0 {
		return SettlementDecision{}, gerror.New("settlement request is invalid")
	}
	if !request.PayeeVerified {
		return SettlementDecision{ReasonCode: "payee_unverified"}, nil
	}
	if request.RequestedAmountMinor < request.MinimumAmountMinor {
		return SettlementDecision{ReasonCode: "below_minimum"}, nil
	}
	if request.RequestedAmountMinor > request.AvailableAmountMinor {
		return SettlementDecision{ReasonCode: "insufficient_available"}, nil
	}
	return SettlementDecision{Allowed: true, FrozenAmountMinor: request.RequestedAmountMinor}, nil
}

type PayoutDecision struct {
	StatusCode   string
	RetryAllowed bool
	ReasonCode   string
}

func DecidePayout(statusCode string, providerAccepted *bool, failureReason string) PayoutDecision {
	switch statusCode {
	case "succeeded":
		return PayoutDecision{StatusCode: "succeeded"}
	case "failed":
		if providerAccepted == nil {
			return PayoutDecision{StatusCode: "manual_review", ReasonCode: "result_unknown"}
		}
		if *providerAccepted {
			return PayoutDecision{StatusCode: "manual_review", ReasonCode: "provider_accepted"}
		}
		return PayoutDecision{StatusCode: "failed", RetryAllowed: true, ReasonCode: failureReason}
	case "result_unknown", "processing":
		return PayoutDecision{StatusCode: "manual_review", ReasonCode: "result_unknown"}
	default:
		return PayoutDecision{StatusCode: "manual_review", ReasonCode: "invalid_status"}
	}
}
