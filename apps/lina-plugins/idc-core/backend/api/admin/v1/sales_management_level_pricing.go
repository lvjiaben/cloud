// This file defines independent tier-wide price percentage management.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SalesManagementLevelPricing struct {
	LevelNo               string `json:"levelNo" dc:"Agent tier reference" eg:"LEVEL_example"`
	PercentageBasisPoints *int   `json:"percentageBasisPoints" dc:"Percentage charged in basis points: 0 is free, 8000 charges 80 percent, 10000 charges full price; null means unset" eg:"8000"`
	Version               int64  `json:"version" dc:"Independent pricing revision; zero means never configured" eg:"1"`
}
type SalesManagementLevelPricingGetReq struct {
	g.Meta  `path:"/admin/sales-management/levels/{levelNo}/pricing" method:"get" tags:"IDC Sales Management" summary:"Read tier-wide price percentage" dc:"Read the independent default percentage used only when customer and product tier prices are absent." permission:"idc:admin:sales-channel:pricing:manage"`
	LevelNo string `json:"levelNo" v:"required|max-length:128" dc:"Agent tier reference" eg:"LEVEL_example"`
}
type SalesManagementLevelPricingGetRes struct {
	Item SalesManagementLevelPricing `json:"item" dc:"Current tier-wide pricing configuration" eg:"{}"`
}
type SalesManagementLevelPricingSaveReq struct {
	g.Meta                `path:"/admin/sales-management/levels/{levelNo}/pricing" method:"put" tags:"IDC Sales Management" summary:"Set or clear tier-wide price percentage" dc:"Append a versioned price percentage independently from tier administration; null clears the default without changing historical orders." permission:"idc:admin:sales-channel:pricing:manage"`
	LevelNo               string `json:"levelNo" v:"required|max-length:128" dc:"Agent tier reference" eg:"LEVEL_example"`
	PercentageBasisPoints *int   `json:"percentageBasisPoints" v:"min:0|max:10000" dc:"Percentage charged in basis points: 0 is free, 8000 charges 80 percent, 10000 charges full price; null clears the default" eg:"8000"`
	ExpectedVersion       int64  `json:"expectedVersion" v:"min:0" dc:"Independent current pricing revision; zero creates the first revision" eg:"0"`
}
type SalesManagementLevelPricingSaveRes struct {
	Item SalesManagementLevelPricing `json:"item" dc:"Current tier-wide pricing configuration" eg:"{}"`
}
