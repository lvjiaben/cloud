package v1

import "github.com/gogf/gf/v2/frame/g"

// OperationsAnalyticsCapabilitiesGetReq describes the read-only analytics
// contract exposed to the operations workspace.  It deliberately contains no
// customer or financial data; the actual report endpoints still apply their
// own data-scope checks.
type OperationsAnalyticsCapabilitiesGetReq struct {
	g.Meta `path:"/admin/operations/analytics/capabilities" method:"get" tags:"IDC Operations" summary:"Get operations analytics capabilities" dc:"Return the stable risk, report, currency, and audit-export contract for the operations workspace." permission:"idc:admin:operations:workbench:view"`
}

type OperationsRiskEventOption struct {
	Code string `json:"code" dc:"Stable risk event code"`
}

type OperationsRiskActionOption struct {
	Code string `json:"code" dc:"Stable risk action code"`
}

type OperationsManualReviewContract struct {
	ReviewableStatuses []string `json:"reviewableStatuses" dc:"Risk statuses that may be reviewed by a human"`
	Decisions          []string `json:"decisions" dc:"Allowed human review decisions"`
	ReasonRequired     bool     `json:"reasonRequired" dc:"Whether every human decision requires a reason"`
}

type OperationsReportContract struct {
	MetricCodes              []string `json:"metricCodes" dc:"Complete report metric codes; values remain grouped by original currency"`
	OriginalCurrencyGrouped  bool     `json:"originalCurrencyGrouped" dc:"Whether metrics are always grouped by source currency"`
	ExchangeRateOptional     bool     `json:"exchangeRateOptional" dc:"Whether a merchant-configured reporting rate may add converted values"`
	ConvertedValueLedgerSafe bool     `json:"convertedValueLedgerSafe" dc:"Converted values never enter payment, refund, balance, or ledger writes"`
}

type OperationsAuditExportContract struct {
	Format         string `json:"format" dc:"Only supported asynchronous export format"`
	Asynchronous   bool   `json:"asynchronous" dc:"Whether export generation is asynchronous"`
	MaxRows        int    `json:"maxRows" dc:"Maximum rows accepted by an export plan"`
	ShortLivedLink bool   `json:"shortLivedLink" dc:"Whether download uses a short-lived private link"`
	RechecksScope  bool   `json:"rechecksScope" dc:"Whether generation and download recheck the captured data scope"`
}

type OperationsAnalyticsCapabilitiesGetRes struct {
	RiskEvents   []*OperationsRiskEventOption    `json:"riskEvents"`
	RiskActions  []*OperationsRiskActionOption   `json:"riskActions"`
	ManualReview *OperationsManualReviewContract `json:"manualReview"`
	Report       *OperationsReportContract       `json:"report"`
	AuditExport  *OperationsAuditExportContract  `json:"auditExport"`
}
