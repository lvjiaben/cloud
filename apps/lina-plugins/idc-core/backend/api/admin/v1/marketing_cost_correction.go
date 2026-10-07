package v1

import "github.com/gogf/gf/v2/frame/g"

type MarketingMissingCostListReq struct {
	g.Meta `path:"/admin/marketing/missing-costs" method:"get" tags:"IDC Marketing Management" summary:"List marketing order items with missing cost" dc:"Return a bounded oldest-first list of marketing order items whose order-time cost was unavailable." permission:"idc:admin:marketing:cost:missing:list"`
	Limit  int `json:"limit" v:"min:1|max:100"`
	Page   int `json:"page" v:"min:1" dc:"Optional one-based page number"`
}

type MarketingMissingCostListRes struct {
	Items []*MarketingMissingCostItem `json:"items"`
	Total int                         `json:"total" dc:"Exact count within the same filters and authorized data scope"`
}

type MarketingCostCorrectionReq struct {
	g.Meta          `path:"/admin/marketing/missing-costs/{orderItemNo}/corrections" method:"post" tags:"IDC Marketing Management" summary:"Correct one missing historical order cost" dc:"Fill only a missing order-item cost using a stable request, mandatory reason, optimistic version and immutable staff audit." permission:"idc:admin:marketing:cost:correct"`
	OrderItemNo     string `json:"orderItemNo" v:"required|length:1,40"`
	RequestNo       string `json:"requestNo" v:"required|length:8,64"`
	UnitCostMinor   string `json:"unitCostMinor" v:"required|regex:^\\d+$"`
	Reason          string `json:"reason" v:"required|length:2,500"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
}

type MarketingCostCorrectionRes struct {
	Item *MarketingMissingCostItem `json:"item"`
}

type MarketingMissingCostItem struct {
	OrderItemNo      string  `json:"orderItemNo"`
	OrderNo          string  `json:"orderNo"`
	VersionNo        string  `json:"versionNo" dc:"Frozen product version number"`
	CurrencyCode     string  `json:"currencyCode"`
	PeriodUnitCode   string  `json:"periodUnitCode"`
	PeriodCount      int     `json:"periodCount"`
	Quantity         int     `json:"quantity" dc:"Frozen purchased quantity"`
	PurchaseCycles   int     `json:"purchaseCycles"`
	ProductNameZhCn  string  `json:"productNameZhCn"`
	ProductNameEnUs  string  `json:"productNameEnUs"`
	StatusCode       string  `json:"statusCode"`
	UnitCostMinor    *string `json:"unitCostMinor"`
	TotalCostMinor   *string `json:"totalCostMinor"`
	CorrectionReason string  `json:"correctionReason"`
	Version          int64   `json:"version"`
	CreatedAt        int64   `json:"createdAt"`
	UpdatedAt        int64   `json:"updatedAt"`
}
