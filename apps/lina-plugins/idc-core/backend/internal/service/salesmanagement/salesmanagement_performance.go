// This file defines independent monthly payment performance and its immutable ownership boundary.
package salesmanagement

import "context"

// PerformanceService reads only payment facts and original owners, independent of commissions.
type PerformanceService interface {
	// ListPerformance requires overview permission and supported merchant-wide data scope.
	// Month uses merchant calendar time; refunds are cumulative through query time.
	// Invalid filters or unavailable identity/merchant data fail without returning partial aggregates.
	ListPerformance(context.Context, PerformanceInput) (PerformancePage, error)
}

// PerformanceInput bounds grouped results and searches visible owner names.
type PerformanceInput struct {
	PageInput
	Month, BeneficiaryType, CurrencyCode string
}

// Performance separates new orders and renewals and never combines currencies.
type Performance struct {
	BeneficiaryType, BeneficiaryReference, BeneficiaryName, CurrencyCode string
	PaidOrderCount, PaidRenewalCount                                     int64
	ReceivedMinor, RefundedMinor, NetReceivedMinor                       string
}

// PerformancePage includes the authoritative calendar and refund cutoff.
type PerformancePage struct {
	Items    []Performance
	Total    int
	Timezone string
	AsOf     int64
}
