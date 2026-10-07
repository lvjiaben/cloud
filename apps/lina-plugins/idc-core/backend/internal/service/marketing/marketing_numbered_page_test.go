package marketing

import (
	"context"
	"math"
	"testing"
)

func TestMarketingNumberedPageBounds(t *testing.T) {
	for _, input := range []struct {
		page, limit int
		cursor      string
	}{
		{-1, 10, ""}, {math.MaxInt, 100, ""}, {1, 10, "cursor"}, {1, 101, ""},
	} {
		if validMarketingNumberedPage(input.page, input.limit, input.cursor) {
			t.Fatalf("accepted invalid numeric page: %+v", input)
		}
	}
	for _, input := range []struct{ page, limit, offset int }{{0, 0, 0}, {1, 10, 0}, {3, 10, 20}, {2, 100, 100}} {
		limit, offset, err := marketingInsightsPage(input.page, input.limit)
		if err != nil || offset != input.offset || (input.limit > 0 && limit != input.limit) {
			t.Fatalf("numeric page %+v returned %d/%d/%v", input, limit, offset, err)
		}
	}
}

func TestMarketingNumberedPageAuthorizationPrecedesDatabase(t *testing.T) {
	authorizer := &numberedInsightsDenied{}
	s := &marketingInsightsServiceImpl{authorizer: authorizer}
	if _, err := s.Performance(context.Background(), MarketingPerformanceInput{Page: 2, Limit: 10}); err != context.Canceled {
		t.Fatalf("performance authorization: %v", err)
	}
	if _, err := s.AbuseCases(context.Background(), MarketingAbuseSearchInput{Page: 2, Limit: 10}); err != context.Canceled {
		t.Fatalf("abuse authorization: %v", err)
	}
	if _, err := s.MissingCosts(context.Background(), MarketingMissingCostSearchInput{Page: 2, Limit: 10}); err != context.Canceled {
		t.Fatalf("cost authorization: %v", err)
	}
}

type numberedInsightsDenied struct{ MarketingInsightsAuthorizer }

func (*numberedInsightsDenied) AuthorizeMarketingPerformance(context.Context) (int, error) {
	return 0, context.Canceled
}
func (*numberedInsightsDenied) AuthorizeMarketingAbuseList(context.Context) (int, error) {
	return 0, context.Canceled
}
func (*numberedInsightsDenied) AuthorizeMarketingCostList(context.Context) (int, error) {
	return 0, context.Canceled
}
