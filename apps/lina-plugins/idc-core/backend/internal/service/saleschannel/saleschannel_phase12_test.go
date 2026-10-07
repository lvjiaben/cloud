package saleschannel

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestQualificationTransitionRejectsTerminalAndFutureChanges(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	decision, err := New().ValidateQualificationTransition(context.Background(), QualificationTransitionRequest{
		Now: now, CurrentStatus: QualificationExpired, NextStatus: QualificationActive,
		CurrentLevelNo: "L1", NextLevelNo: "L2", EffectiveAt: now, ReasonCode: "renew", ActorReference: "staff_1", EvidenceDigest: digest,
	})
	if err != nil || decision.Allowed || decision.ReasonCode != QualificationReasonTerminal {
		t.Fatalf("terminal transition decision=%+v err=%v", decision, err)
	}
	decision, err = New().ValidateQualificationTransition(context.Background(), QualificationTransitionRequest{
		Now: now, CurrentStatus: QualificationActive, NextStatus: QualificationSuspended,
		CurrentLevelNo: "L1", NextLevelNo: "L1", EffectiveAt: now.Add(time.Minute), ReasonCode: "manual", ActorReference: "staff_1", EvidenceDigest: digest,
	})
	if err != nil || decision.Allowed || decision.ReasonCode != QualificationReasonEffectiveDateFuture {
		t.Fatalf("future transition decision=%+v err=%v", decision, err)
	}
	decision, err = New().ValidateQualificationTransition(context.Background(), QualificationTransitionRequest{
		Now: now, NextStatus: QualificationActive, NextLevelNo: "L1", EffectiveAt: now,
		ReasonCode: "application_approved", ActorReference: "staff_1", EvidenceDigest: digest,
	})
	if err != nil || !decision.Allowed {
		t.Fatalf("initial qualification decision=%+v err=%v", decision, err)
	}
}

func TestSelectAttributionHonorsFirstLastAndDuplicateProtection(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	touches := []AttributionTouch{
		{TouchNo: "touch_late", OccurredAt: now.Add(-time.Minute)},
		{TouchNo: "touch_early", OccurredAt: now.Add(-2 * time.Minute)},
		{TouchNo: "touch_future", OccurredAt: now.Add(time.Minute)},
	}
	first, err := New().SelectAttribution(context.Background(), AttributionSelectionRequest{Now: now, PolicyCode: AffiliateFirstTouch, OwnerCustomerNo: "owner", ReferredCustomerNo: "buyer", Touches: touches})
	if err != nil || !first.Allowed || first.TouchNo != "touch_early" {
		t.Fatalf("first attribution=%+v err=%v", first, err)
	}
	last, err := New().SelectAttribution(context.Background(), AttributionSelectionRequest{Now: now, PolicyCode: AffiliateLastTouch, OwnerCustomerNo: "owner", ReferredCustomerNo: "buyer", Touches: touches})
	if err != nil || !last.Allowed || last.TouchNo != "touch_late" {
		t.Fatalf("last attribution=%+v err=%v", last, err)
	}
	duplicate, err := New().SelectAttribution(context.Background(), AttributionSelectionRequest{Now: now, PolicyCode: AffiliateFirstTouch, OwnerCustomerNo: "owner", ReferredCustomerNo: "buyer", AlreadyAttributed: true})
	if err != nil || duplicate.Allowed || duplicate.ReasonCode != AffiliateReasonDuplicate {
		t.Fatalf("duplicate attribution=%+v err=%v", duplicate, err)
	}
}

func TestPlanAssignmentBatchSkipsProtectedAndMismatchedRows(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	decision, err := New().PlanAssignmentBatch(context.Background(), AssignmentBatchRequest{
		Now: now, SourceMembershipNo: "sales_old", TargetMembershipNo: "sales_new",
		Items: []AssignmentBatchItem{
			{CustomerNo: "cust_1", Current: &entity.SalesCustomerAssignment{AssignmentNo: "a1", MembershipNo: "sales_old", StatusCode: AssignmentActive, ProtectionExpiresAt: ptrTime(now.Add(time.Hour))}},
			{CustomerNo: "cust_2", Current: &entity.SalesCustomerAssignment{AssignmentNo: "a2", MembershipNo: "other", StatusCode: AssignmentActive}},
		},
	})
	if err != nil || !decision.Allowed || decision.Processed != 0 || decision.Skipped != 2 {
		t.Fatalf("batch assignment=%+v err=%v", decision, err)
	}
}

func TestAllocateCommissionRefundIsIncrementalAndCurrencySafe(t *testing.T) {
	allocation, err := AllocateCommissionRefund(context.Background(), CommissionRefundRequest{OriginalBaseMinor: 10000, CommissionMinor: 1000, AlreadyRefundedBase: 2500, RefundBaseMinor: 2500})
	if err != nil || allocation.ReversalMinor != 250 || allocation.RemainingCommission != 500 {
		t.Fatalf("refund allocation=%+v err=%v", allocation, err)
	}
	values, err := AggregateCommissionByCurrency(context.Background(), []CurrencyAmount{{CurrencyCode: "USD", AmountMinor: 5}, {CurrencyCode: "CNY", AmountMinor: 20}, {CurrencyCode: "USD", AmountMinor: 7}})
	if err != nil || len(values) != 2 || values[0].CurrencyCode != "CNY" || values[0].AmountMinor != 20 || values[1].CurrencyCode != "USD" || values[1].AmountMinor != 12 {
		// Keep the failure message useful without converting currencies or
		// comparing values through floating point arithmetic.
		t.Fatalf("currency aggregation=%+v err=%v", values, err)
	}
	if _, err = AggregateCommissionByCurrency(context.Background(), []CurrencyAmount{{CurrencyCode: "CNY", AmountMinor: int64(^uint64(0) >> 1)}, {CurrencyCode: "CNY", AmountMinor: 1}}); err == nil {
		t.Fatal("expected cross-row currency overflow to fail")
	}
}

func TestSalesChannelRejectsOverflowAndMalformedTiers(t *testing.T) {
	now := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	discount := 1
	offer, err := New().Evaluate(context.Background(), OfferRequest{
		Now: now, ProductNo: "p", CurrencyCode: "CNY", BasePriceMinor: int64(^uint64(0) >> 1),
		CostMinor:     int64(^uint64(0) >> 1),
		Qualification: &entity.SalesAgentQualification{QualificationNo: "q", StatusCode: "active", EffectiveAt: ptrTime(now.Add(-time.Hour))},
		ProductRule:   &entity.SalesAgentProductRule{ProductRuleNo: "pr", ProductNo: "p", StatusCode: "published", EffectiveAt: ptrTime(now.Add(-time.Hour))},
		PriceRule:     &entity.SalesAgentPriceRule{PriceRuleNo: "price", ProductRuleNo: "pr", CurrencyCode: "CNY", AdjustmentTypeCode: string(AdjustmentDiscountBasisPoints), DiscountBasisPoints: &discount, MinimumMarginMinor: 1, EffectiveAt: ptrTime(now.Add(-time.Hour)), StatusCode: "published"},
	})
	if err != nil || offer.Eligible || offer.ReasonCode != "agent_margin_floor_exceeded" {
		t.Fatalf("expected overflowing margin floor to be rejected, offer=%+v err=%v", offer, err)
	}
	rate := 1000
	upper := int64(100)
	_, err = New().CalculateCommission(context.Background(), CommissionRequest{
		Now: now, OperationTypeCode: "new_purchase", BeneficiaryTypeCode: "agent", ProductNo: "p", CurrencyCode: "CNY", BaseAmountMinor: 50,
		Rule: &entity.SalesCommissionRule{RuleNo: "r", BeneficiaryTypeCode: "agent", OperationTypeCode: "new_purchase", CurrencyCode: "CNY", CalculationTypeCode: CommissionTiered, EffectiveAt: ptrTime(now.Add(-time.Hour)), StatusCode: "published"},
		Tiers: []entity.SalesCommissionTier{
			{TierNo: "t1", RuleNo: "r", LowerBoundMinor: 0, UpperBoundMinor: &upper, RateBasisPoints: rate},
			{TierNo: "t2", RuleNo: "r", LowerBoundMinor: 50, RateBasisPoints: rate},
		},
	})
	if err == nil {
		t.Fatal("expected overlapping commission tiers to fail")
	}
}

func TestAffiliateRejectsFutureAndExpiredWindowTouches(t *testing.T) {
	now := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	link := &entity.SalesAffiliateLink{LinkNo: "link", OwnerCustomerNo: "owner", AttributionPolicyCode: AffiliateFirstTouch, AttributionWindowSeconds: 60, StatusCode: AffiliateLinkActive}
	future := now.Add(time.Minute)
	decision, err := New().Attribute(context.Background(), AttributionRequest{Now: now, Link: link, Touch: &entity.SalesAffiliateTouch{TouchNo: "future", LinkNo: "link", OccurredAt: &future, ExpiresAt: ptrTime(now.Add(time.Hour))}, ReferredCustomerNo: "buyer"})
	if err != nil || decision.StatusCode != AffiliateRejectedRisk {
		t.Fatalf("future touch decision=%+v err=%v", decision, err)
	}
	old := now.Add(-2 * time.Minute)
	decision, err = New().Attribute(context.Background(), AttributionRequest{Now: now, Link: link, Touch: &entity.SalesAffiliateTouch{TouchNo: "old", LinkNo: "link", OccurredAt: &old, ExpiresAt: ptrTime(now.Add(time.Hour))}, ReferredCustomerNo: "buyer"})
	if err != nil || decision.StatusCode != AffiliateRejectedRisk {
		t.Fatalf("expired window decision=%+v err=%v", decision, err)
	}
}
