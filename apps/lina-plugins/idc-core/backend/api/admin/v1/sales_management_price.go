// This file exposes real, permission-controlled prices management.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesManagementPriceListReq struct {
	g.Meta `path:"/admin/sales-management/prices" method:"get" tags:"IDC Sales Management" summary:"List prices" dc:"Return a bounded page of current prices with display names." permission:"idc:admin:sales-channel:pricing:manage"`
	SalesManagementPageQuery
	ProductNo  string `json:"productNo" v:"max-length:128" dc:"Exact product reference filter" eg:"PRD_example"`
	CustomerNo string `json:"customerNo" v:"max-length:128" dc:"Exact customer reference filter" eg:"CUS_example"`
	LevelNo    string `json:"levelNo" v:"max-length:128" dc:"Exact agent tier reference filter" eg:"LEVEL_example"`
	Target     string `json:"target" v:"in:customer,level" dc:"Price target filter" eg:"customer"`
}
type SalesManagementPriceListRes struct {
	Items []SalesManagementPrice `json:"items" dc:"Current page records" eg:"[]"`
	Total int                    `json:"total" dc:"Total matching records" eg:"0"`
}
type SalesManagementPriceCreateReq struct {
	g.Meta              `path:"/admin/sales-management/prices" method:"post" tags:"IDC Sales Management" summary:"Create prices" dc:"Persist one versioned prices change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:pricing:manage"`
	PriceNo             string `json:"priceNo" v:"max-length:128" dc:"Sales management priceNo" eg:"example"`
	CustomerNo          string `json:"customerNo" v:"max-length:128" dc:"Sales management customerNo" eg:"example"`
	LevelNo             string `json:"levelNo" v:"max-length:128" dc:"Sales management levelNo" eg:"example"`
	ProductNo           string `json:"productNo" v:"max-length:128" dc:"Sales management productNo" eg:"example"`
	CurrencyCode        string `json:"currencyCode" v:"max-length:128" dc:"Sales management currencyCode" eg:"example"`
	ChargeTypeCode      string `json:"chargeTypeCode" v:"max-length:128" dc:"Sales management chargeTypeCode" eg:"example"`
	PeriodUnitCode      string `json:"periodUnitCode" v:"max-length:128" dc:"Sales management periodUnitCode" eg:"example"`
	AdjustmentType      string `json:"adjustmentType" v:"max-length:128" dc:"Sales management adjustmentType" eg:"example"`
	Status              string `json:"status" v:"required|in:active,inactive" dc:"Sales management status" eg:"active"`
	FixedPriceMinor     string `json:"fixedPriceMinor" v:"max-length:128" dc:"Sales management fixedPriceMinor" eg:"example"`
	PeriodCount         int    `json:"periodCount" dc:"Sales management periodCount" eg:"1"`
	DiscountBasisPoints *int   `json:"discountBasisPoints" dc:"Sales management discountBasisPoints" eg:"1"`
	ExpectedVersion     int64  `json:"expectedVersion" v:"in:0" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementPriceCreateRes struct {
	Item SalesManagementPrice `json:"item" dc:"Saved record" eg:"{}"`
}
type SalesManagementPriceUpdateReq struct {
	g.Meta              `path:"/admin/sales-management/prices" method:"put" tags:"IDC Sales Management" summary:"Update prices" dc:"Persist one versioned prices change with transaction audit; historical financial snapshots remain unchanged." permission:"idc:admin:sales-channel:pricing:manage"`
	PriceNo             string `json:"priceNo" v:"max-length:128" dc:"Sales management priceNo" eg:"example"`
	CustomerNo          string `json:"customerNo" v:"max-length:128" dc:"Sales management customerNo" eg:"example"`
	LevelNo             string `json:"levelNo" v:"max-length:128" dc:"Sales management levelNo" eg:"example"`
	ProductNo           string `json:"productNo" v:"max-length:128" dc:"Sales management productNo" eg:"example"`
	CurrencyCode        string `json:"currencyCode" v:"max-length:128" dc:"Sales management currencyCode" eg:"example"`
	ChargeTypeCode      string `json:"chargeTypeCode" v:"max-length:128" dc:"Sales management chargeTypeCode" eg:"example"`
	PeriodUnitCode      string `json:"periodUnitCode" v:"max-length:128" dc:"Sales management periodUnitCode" eg:"example"`
	AdjustmentType      string `json:"adjustmentType" v:"max-length:128" dc:"Sales management adjustmentType" eg:"example"`
	Status              string `json:"status" v:"required|in:active,inactive" dc:"Sales management status" eg:"active"`
	FixedPriceMinor     string `json:"fixedPriceMinor" v:"max-length:128" dc:"Sales management fixedPriceMinor" eg:"example"`
	PeriodCount         int    `json:"periodCount" dc:"Sales management periodCount" eg:"1"`
	DiscountBasisPoints *int   `json:"discountBasisPoints" dc:"Sales management discountBasisPoints" eg:"1"`
	ExpectedVersion     int64  `json:"expectedVersion" v:"required|min:1" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementPriceUpdateRes struct {
	Item SalesManagementPrice `json:"item" dc:"Saved record" eg:"{}"`
}

// SalesManagementPriceDeleteReq removes a current rule by appending an immutable tombstone.
type SalesManagementPriceDeleteReq struct {
	g.Meta          `path:"/admin/sales-management/prices" method:"delete" tags:"IDC Sales Management" summary:"Delete a customized or agent tier price" dc:"Append a deletion revision; new quotes fall back to the next eligible price and historical orders remain unchanged." permission:"idc:admin:sales-channel:pricing:manage"`
	PriceNo         string `json:"priceNo" v:"required|max-length:128" dc:"Sales management priceNo" eg:"APR_example"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Sales management expectedVersion" eg:"1"`
}
type SalesManagementPriceDeleteRes struct {
	Item SalesManagementPrice `json:"item" dc:"Saved record" eg:"{}"`
}
