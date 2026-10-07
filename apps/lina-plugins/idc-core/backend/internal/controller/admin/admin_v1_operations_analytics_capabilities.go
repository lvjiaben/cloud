package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
)

// OperationsAnalyticsCapabilitiesGet returns the stable, read-only analytics
// contract.  It intentionally contains no data rows and therefore cannot be
// used to infer customers outside the caller's normal report scope.
func (c *ControllerV1) OperationsAnalyticsCapabilitiesGet(_ context.Context, _ *v1.OperationsAnalyticsCapabilitiesGetReq) (*v1.OperationsAnalyticsCapabilitiesGetRes, error) {
	return &v1.OperationsAnalyticsCapabilitiesGetRes{
		RiskEvents: []*v1.OperationsRiskEventOption{
			{Code: "registration"}, {Code: "identity"}, {Code: "order"}, {Code: "payment"},
			{Code: "refund"}, {Code: "ip_change"}, {Code: "commission"}, {Code: "high_frequency"},
		},
		RiskActions: []*v1.OperationsRiskActionOption{
			{Code: "allow"}, {Code: "remind"}, {Code: "restrict"}, {Code: "delay"}, {Code: "manual_review"},
		},
		ManualReview: &v1.OperationsManualReviewContract{
			ReviewableStatuses: []string{"open", "acknowledged"},
			Decisions:          []string{"acknowledge", "resolve", "dismiss"},
			ReasonRequired:     true,
		},
		Report: &v1.OperationsReportContract{
			MetricCodes: []string{
				"orders", "revenue", "receivable", "paid", "cash_flow", "balance", "refund",
				"renewal", "expiry", "active_service", "customer", "product", "sales", "affiliate", "profit", "supplier_quality",
			},
			OriginalCurrencyGrouped:  true,
			ExchangeRateOptional:     true,
			ConvertedValueLedgerSafe: true,
		},
		AuditExport: &v1.OperationsAuditExportContract{
			Format:         "xlsx",
			Asynchronous:   true,
			MaxRows:        100000,
			ShortLivedLink: true,
			RechecksScope:  true,
		},
	}, nil
}
