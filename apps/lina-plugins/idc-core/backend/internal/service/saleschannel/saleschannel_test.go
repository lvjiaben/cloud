// Package saleschannel tests the deterministic agent pricing rules.
package saleschannel

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestEvaluateAgentOfferFreezesPublishedRuleVersions(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	discount := 1500
	service := New()
	offer, err := service.Evaluate(context.Background(), OfferRequest{
		Now: now, ProductNo: "prod_basic", CurrencyCode: "CNY", BasePriceMinor: 10000,
		CostMinor:     7000,
		Qualification: &entity.SalesAgentQualification{QualificationNo: "qual_1", StatusCode: "active", EffectiveAt: ptrTime(now.Add(-time.Hour)), Version: 3},
		ProductRule:   &entity.SalesAgentProductRule{ProductRuleNo: "pr_1", ProductNo: "prod_basic", StatusCode: "published", EffectiveAt: ptrTime(now.Add(-time.Hour)), Version: 5},
		PriceRule:     &entity.SalesAgentPriceRule{PriceRuleNo: "price_1", ProductRuleNo: "pr_1", CurrencyCode: "CNY", AdjustmentTypeCode: "discount_basis_points", DiscountBasisPoints: &discount, MinimumMarginMinor: 1000, StatusCode: "published", EffectiveAt: ptrTime(now.Add(-time.Hour)), Version: 7},
	})
	if err != nil || !offer.Eligible || offer.PriceMinor != 8500 || offer.Snapshot.QualificationVersion != 3 || offer.Snapshot.ProductRuleVersion != 5 || offer.Snapshot.PriceRuleVersion != 7 {
		t.Fatalf("unexpected offer: %+v, error: %v", offer, err)
	}
}

func TestEvaluateAgentOfferRejectsExpiredRuleAndCreditOverflow(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service := New()
	base := OfferRequest{
		Now: now, ProductNo: "prod_basic", CurrencyCode: "CNY", BasePriceMinor: 10000,
		Qualification: &entity.SalesAgentQualification{QualificationNo: "qual_1", StatusCode: "active", EffectiveAt: ptrTime(now.Add(-time.Hour)), Version: 1},
		ProductRule:   &entity.SalesAgentProductRule{ProductRuleNo: "pr_1", ProductNo: "prod_basic", StatusCode: "published", EffectiveAt: ptrTime(now.Add(-time.Hour)), Version: 1},
		PriceRule:     &entity.SalesAgentPriceRule{PriceRuleNo: "price_1", ProductRuleNo: "pr_1", CurrencyCode: "CNY", AdjustmentTypeCode: "fixed_price", FixedPriceMinor: ptrInt64(9000), StatusCode: "published", EffectiveAt: ptrTime(now.Add(-time.Hour)), ExpiresAt: ptrTime(now.Add(-time.Minute)), Version: 1},
	}
	offer, err := service.Evaluate(context.Background(), base)
	if err != nil || offer.Eligible || offer.ReasonCode != ReasonPriceRuleUnavailable {
		t.Fatalf("unexpected expired-rule result: %+v, error: %v", offer, err)
	}

	base.PriceRule.ExpiresAt = ptrTime(now.Add(time.Hour))
	base.CreditLimit = &entity.SalesAgentCreditLimit{CurrencyCode: "CNY", LimitAmountMinor: 10000, StatusCode: "active", EffectiveAt: ptrTime(now.Add(-time.Hour))}
	base.ExistingCreditUsed = 2000
	offer, err = service.Evaluate(context.Background(), base)
	if err != nil || offer.Eligible || offer.ReasonCode != ReasonCreditLimitExceeded {
		t.Fatalf("unexpected credit result: %+v, error: %v", offer, err)
	}
}

func TestNewWithDatabaseRequiresDatabase(t *testing.T) {
	if _, err := NewWithDatabase(nil); err == nil {
		t.Fatal("expected a missing database error")
	}
}

func TestAttributeRejectsSelfReferralAndRelatedIdentity(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service := New()
	link := &entity.SalesAffiliateLink{LinkNo: "link_1", OwnerCustomerNo: "cust_owner", AttributionPolicyCode: AffiliateFirstTouch, AttributionWindowSeconds: 3600, StatusCode: AffiliateLinkActive, ExpiresAt: ptrTime(now.Add(time.Hour))}
	touch := &entity.SalesAffiliateTouch{TouchNo: "touch_1", LinkNo: "link_1", OccurredAt: ptrTime(now.Add(-time.Minute)), ExpiresAt: ptrTime(now.Add(time.Hour))}
	decision, err := service.Attribute(context.Background(), AttributionRequest{Now: now, Link: link, Touch: touch, ReferredCustomerNo: "cust_owner"})
	if err != nil || decision.StatusCode != AffiliateRejectedSelfReferral || decision.RiskReasonCode != AffiliateReasonSelfReferral {
		t.Fatalf("unexpected self-referral decision: %+v, error: %v", decision, err)
	}
	decision, err = service.Attribute(context.Background(), AttributionRequest{Now: now, Link: link, Touch: touch, ReferredCustomerNo: "cust_buyer", relatedIdentity: true})
	if err != nil || decision.StatusCode != AffiliateRejectedSelfReferral || decision.RiskReasonCode != AffiliateReasonRelatedIdentity {
		t.Fatalf("unexpected related-identity decision: %+v, error: %v", decision, err)
	}
}

func TestAttributeAcceptsValidLastTouch(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	decision, err := New().Attribute(context.Background(), AttributionRequest{
		Now:                now,
		Link:               &entity.SalesAffiliateLink{LinkNo: "link_2", OwnerCustomerNo: "cust_owner", AttributionPolicyCode: AffiliateLastTouch, AttributionWindowSeconds: 3600, StatusCode: AffiliateLinkActive},
		Touch:              &entity.SalesAffiliateTouch{TouchNo: "touch_2", LinkNo: "link_2", OccurredAt: ptrTime(now.Add(-time.Minute)), ExpiresAt: ptrTime(now.Add(time.Hour))},
		ReferredCustomerNo: "cust_buyer",
	})
	if err != nil || decision.StatusCode != AffiliateEligible || decision.PolicyCode != AffiliateLastTouch {
		t.Fatalf("unexpected valid attribution decision: %+v, error: %v", decision, err)
	}
}

func TestCalculateCommissionAppliesTierAndCap(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	rate := 1200
	capAmount := int64(900)
	snapshot, err := New().CalculateCommission(context.Background(), CommissionRequest{
		Now: now, OperationTypeCode: "new_purchase", BeneficiaryTypeCode: "agent", ProductNo: "prod_basic", CurrencyCode: "CNY", BaseAmountMinor: 10000,
		Rule: &entity.SalesCommissionRule{RuleNo: "rule_1", BeneficiaryTypeCode: "agent", OperationTypeCode: "new_purchase", CurrencyCode: "CNY", CalculationTypeCode: CommissionRate, RateBasisPoints: &rate, CapAmountMinor: &capAmount, MaturityDays: 7, StatusCode: "published", EffectiveAt: ptrTime(now.Add(-time.Hour)), Version: 2},
	})
	if err != nil || snapshot.AmountMinor != 900 || !snapshot.MaturesAt.Equal(now.Add(7*24*time.Hour)) || snapshot.RuleNo != "rule_1" {
		t.Fatalf("unexpected commission snapshot: %+v, error: %v", snapshot, err)
	}

	tierRate := 2500
	tierUpper := int64(20000)
	snapshot, err = New().CalculateCommission(context.Background(), CommissionRequest{
		Now: now, OperationTypeCode: "new_purchase", BeneficiaryTypeCode: "agent", ProductNo: "prod_basic", CurrencyCode: "CNY", BaseAmountMinor: 10000,
		Rule:  &entity.SalesCommissionRule{RuleNo: "tier_rule", BeneficiaryTypeCode: "agent", OperationTypeCode: "new_purchase", CurrencyCode: "CNY", CalculationTypeCode: CommissionTiered, MaturityDays: 0, StatusCode: "published", EffectiveAt: ptrTime(now.Add(-time.Hour)), Version: 1},
		Tiers: []entity.SalesCommissionTier{{TierNo: "tier_1", RuleNo: "tier_rule", LowerBoundMinor: 0, UpperBoundMinor: &tierUpper, RateBasisPoints: tierRate}},
	})
	if err != nil || snapshot.AmountMinor != 2500 || snapshot.TierNo != "tier_1" {
		t.Fatalf("unexpected tier snapshot: %+v, error: %v", snapshot, err)
	}
}

func TestAssignHonorsProtectionPeriod(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	decision, err := New().Assign(context.Background(), AssignmentRequest{
		Now: now, NewMembershipNo: "member_new",
		Current: &entity.SalesCustomerAssignment{AssignmentNo: "assignment_old", MembershipNo: "member_old", StatusCode: AssignmentActive, ProtectionExpiresAt: ptrTime(now.Add(time.Hour))},
	})
	if err != nil || decision.StatusCode != AssignmentProtected || decision.OldAssignmentNo != "assignment_old" {
		t.Fatalf("unexpected protected transfer: %+v, error: %v", decision, err)
	}
	decision, err = New().Assign(context.Background(), AssignmentRequest{
		Now: now, NewMembershipNo: "member_new", Force: true,
		Current: &entity.SalesCustomerAssignment{AssignmentNo: "assignment_old", MembershipNo: "member_old", StatusCode: AssignmentActive, ProtectionExpiresAt: ptrTime(now.Add(time.Hour))},
	})
	if err != nil || decision.StatusCode != AssignmentAllowed {
		t.Fatalf("unexpected forced transfer: %+v, error: %v", decision, err)
	}
}

func TestCommissionLifecycleSafety(t *testing.T) {
	svc := New()
	now := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	decision, err := svc.DecideMaturity(context.Background(), MaturityRequest{Now: now, MaturesAt: now.Add(-time.Hour), PendingAmountMinor: 1000, RefundedAmountMinor: 250})
	if err != nil || decision.StatusCode != MaturityMatured || decision.AmountMinor != 750 {
		t.Fatalf("maturity decision=%+v err=%v", decision, err)
	}
	decision, err = svc.DecideMaturity(context.Background(), MaturityRequest{Now: now, MaturesAt: now.Add(-time.Hour), PendingAmountMinor: 1000, RefundedAmountMinor: 1000})
	if err != nil || decision.StatusCode != MaturityReversed {
		t.Fatalf("refund maturity decision=%+v err=%v", decision, err)
	}
	decision, err = svc.DecideMaturity(context.Background(), MaturityRequest{Now: now, MaturesAt: now.Add(time.Hour), PendingAmountMinor: 1000})
	if err != nil || decision.StatusCode != MaturityNotDue {
		t.Fatalf("not due decision=%+v err=%v", decision, err)
	}
	settlement, err := svc.DecideSettlement(context.Background(), SettlementRequest{AvailableAmountMinor: 5000, RequestedAmountMinor: 3000, MinimumAmountMinor: 1000, PayeeVerified: true})
	if err != nil || !settlement.Allowed || settlement.FrozenAmountMinor != 3000 {
		t.Fatalf("settlement decision=%+v err=%v", settlement, err)
	}
	accepted := true
	payout := DecidePayout("failed", &accepted, "timeout")
	if payout.StatusCode != "manual_review" || payout.RetryAllowed {
		t.Fatalf("accepted payout=%+v", payout)
	}
	failed := false
	payout = DecidePayout("failed", &failed, "provider_rejected")
	if payout.StatusCode != "failed" || !payout.RetryAllowed {
		t.Fatalf("rejected payout=%+v", payout)
	}
}

func TestSalesChannelOverviewRequiresDatabase(t *testing.T) {
	_, err := New().LoadOverview(context.Background())
	if err == nil {
		t.Fatal("expected overview to require the shared database")
	}
}

func ptrTime(value time.Time) *time.Time { return &value }
func ptrInt64(value int64) *int64        { return &value }
