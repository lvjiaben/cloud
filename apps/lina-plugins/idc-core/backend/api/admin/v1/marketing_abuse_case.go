package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingAbuseCaseListReq struct {
	g.Meta `path:"/admin/marketing/abuse-cases" method:"get" tags:"IDC Marketing Management" summary:"List redacted marketing abuse cases" dc:"Return a bounded review queue without linked-account identities." permission:"idc:admin:marketing:abuse:list"`
	Status string `json:"status" v:"in:pending_review,confirmed_abuse,false_positive"`
	Limit  int    `json:"limit" v:"min:1|max:100"`
	Page   int    `json:"page" v:"min:1" dc:"Optional one-based page number"`
}

type MarketingAbuseCaseListRes struct {
	Cases []*MarketingAbuseCaseItem `json:"cases"`
	Total int                       `json:"total" dc:"Exact count within the same filters and authorized data scope"`
}

type MarketingAbuseCaseReviewReq struct {
	g.Meta          `path:"/admin/marketing/abuse-cases/{caseNo}/reviews" method:"post" tags:"IDC Marketing Management" summary:"Review a marketing abuse case" dc:"Confirm abuse or false positive with a mandatory reason. Confirmed abuse only creates a campaign-scoped future participation restriction." permission:"idc:admin:marketing:abuse:review"`
	CaseNo          string `json:"caseNo" v:"required|length:1,40"`
	Decision        string `json:"decision" v:"required|in:confirmed_abuse,false_positive"`
	Reason          string `json:"reason" v:"required|length:2,500"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
}

type MarketingAbuseCaseReviewRes struct {
	Case *MarketingAbuseCaseItem `json:"case"`
}

type MarketingAbuseCaseItem struct {
	CaseNo              string `json:"caseNo"`
	CampaignNo          string `json:"campaignNo"`
	CustomerNo          string `json:"customerNo"`
	SignalCode          string `json:"signalCode"`
	StatusCode          string `json:"statusCode"`
	RelatedAccountCount int    `json:"relatedAccountCount"`
	EvidenceSummaryCode string `json:"evidenceSummaryCode"`
	WindowStartedAt     int64  `json:"windowStartedAt"`
	WindowEndedAt       int64  `json:"windowEndedAt"`
	ObservedCount       int    `json:"observedCount"`
	BlockedUntil        *int64 `json:"blockedUntil"`
	ReviewReason        string `json:"reviewReason"`
	ReviewedBy          int    `json:"reviewedBy"`
	ReviewedAt          *int64 `json:"reviewedAt"`
	Version             int64  `json:"version"`
	CreatedAt           int64  `json:"createdAt"`
	UpdatedAt           int64  `json:"updatedAt"`
}
