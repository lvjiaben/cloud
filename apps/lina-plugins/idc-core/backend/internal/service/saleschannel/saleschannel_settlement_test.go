// This file proves retired approval and batch APIs cannot freeze or pay money.
package saleschannel

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
)

func TestLegacyCommissionWritesAreRetired(t *testing.T) {
	service := New()
	for _, decision := range []string{"approved", "manual_review"} {
		if err := service.ReviewSettlement(context.Background(), SettlementReviewRequest{DecisionCode: decision}); !bizerr.Is(err, codeSettlementRetired) {
			t.Fatalf("review retirement=%v", err)
		}
	}
	if err := service.RecordPayoutResult(context.Background(), PayoutResultRequest{StatusCode: "succeeded"}); !bizerr.Is(err, codeSettlementRetired) {
		t.Fatalf("payout retirement=%v", err)
	}
}

type settlementContext struct {
	bizctxcap.Service
	current bizctxcap.CurrentContext
}

func (c settlementContext) Current(context.Context) bizctxcap.CurrentContext { return c.current }

func TestHistoricalSettlementReleaseRejectsUnsupportedContexts(t *testing.T) {
	contexts := []bizctxcap.CurrentContext{
		{},
		{UserID: 81, DataScope: 3},
		{UserID: 81, DataScope: 1, TenantID: 2},
		{UserID: 81, DataScope: 1, ActingUserID: 82},
		{UserID: 81, DataScope: 1, IsImpersonation: true},
		{UserID: 81, DataScope: 1, ActingAsTenant: true},
		{UserID: 81, DataScope: 1, DataScopeUnsupported: true},
	}
	for _, current := range contexts {
		s := &serviceImpl{bizCtx: settlementContext{current: current}}
		if err := s.ReviewSettlement(context.Background(), SettlementReviewRequest{DecisionCode: "rejected"}); !bizerr.Is(err, codeSettlementDenied) {
			t.Fatalf("review context=%+v err=%v", current, err)
		}
		if err := s.RecordPayoutResult(context.Background(), PayoutResultRequest{StatusCode: "failed"}); !bizerr.Is(err, codeSettlementDenied) {
			t.Fatalf("payout context=%+v err=%v", current, err)
		}
	}
}
