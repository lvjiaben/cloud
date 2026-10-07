package commerce

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

type marketingCostAuthorizerStub struct{}

func (marketingCostAuthorizerStub) AuthorizeMarketingPerformance(context.Context) (int, error) {
	return 81, nil
}
func (marketingCostAuthorizerStub) AuthorizeMarketingAbuseList(context.Context) (int, error) {
	return 81, nil
}
func (marketingCostAuthorizerStub) AuthorizeMarketingAbuseReview(context.Context) (int, error) {
	return 81, nil
}
func (marketingCostAuthorizerStub) AuthorizeMarketingCostList(context.Context) (int, error) {
	return 81, nil
}
func (marketingCostAuthorizerStub) AuthorizeMarketingCostCorrect(context.Context) (int, error) {
	return 81, nil
}
func (marketingCostAuthorizerStub) AuthorizeMarketingNewcomerSet(context.Context) (int, error) {
	return 81, nil
}

type marketingCostAuditStub struct {
	transitions []idccap.BusinessAuditTransition
}

func (s *marketingCostAuditStub) AppendBusinessTransition(_ context.Context, transition idccap.BusinessAuditTransition) error {
	s.transitions = append(s.transitions, transition)
	return nil
}

func (*marketingCostAuditStub) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestMarketingMissingCostCorrectionPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 2, true, 0)
	if _, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-marketing-cost-missing-0001",
	}); err != nil {
		t.Fatalf("checkout marketing order: %v", err)
	}
	audit := &marketingCostAuditStub{}
	service, err := marketing.NewMarketingInsightsService(scenario.database, marketingCostAuthorizerStub{}, audit)
	if err != nil {
		t.Fatalf("create marketing insights: %v", err)
	}
	page, err := service.MissingCosts(scenario.ctx, marketing.MarketingMissingCostSearchInput{Limit: 10})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("list missing marketing costs page=%+v err=%v", page, err)
	}
	missing := page.Items[0]
	if missing.VersionNo != "version-commerce-service-1" || missing.Quantity != 2 {
		t.Fatalf("missing cost did not use generic order dimensions: %+v", missing)
	}
	corrected, err := service.CorrectCost(scenario.ctx, marketing.MarketingCostCorrectionInput{
		RequestNo: "cost-correction-request-0001", OrderItemNo: missing.OrderItemNo,
		UnitCostMinor: 321, Reason: "verified historical provider cost", ExpectedVersion: missing.Version,
	})
	if err != nil || corrected.StatusCode != "corrected" || corrected.UnitCostMinor == nil || *corrected.UnitCostMinor != 321 ||
		corrected.TotalCostMinor == nil || *corrected.TotalCostMinor != 642 || corrected.Version != missing.Version+1 ||
		len(audit.transitions) != 1 {
		t.Fatalf("correct missing cost result=%+v err=%v audit=%+v", corrected, err, audit.transitions)
	}
	replayed, err := service.CorrectCost(scenario.ctx, marketing.MarketingCostCorrectionInput{
		RequestNo: "cost-correction-request-0001", OrderItemNo: missing.OrderItemNo,
		UnitCostMinor: 321, Reason: "verified historical provider cost", ExpectedVersion: missing.Version,
	})
	if err != nil || replayed.Version != corrected.Version || len(audit.transitions) != 1 {
		t.Fatalf("replay historical cost result=%+v err=%v audit=%d", replayed, err, len(audit.transitions))
	}
	if _, err = service.CorrectCost(scenario.ctx, marketing.MarketingCostCorrectionInput{
		RequestNo: "cost-correction-request-0002", OrderItemNo: missing.OrderItemNo,
		UnitCostMinor: 999, Reason: "attempt to overwrite history", ExpectedVersion: corrected.Version,
	}); !bizerr.Is(err, marketing.CodeInsightsConflict) {
		t.Fatalf("overwrite corrected cost error=%v", err)
	}
	page, err = service.MissingCosts(scenario.ctx, marketing.MarketingMissingCostSearchInput{Limit: 10})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("corrected item remained in missing queue page=%+v err=%v", page, err)
	}
}
