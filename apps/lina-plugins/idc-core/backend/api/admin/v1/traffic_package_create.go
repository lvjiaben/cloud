// This file defines new fixed traffic packages, without deleting sale history.

package v1

import "github.com/gogf/gf/v2/frame/g"

type TrafficPackageFields struct {
	NameZhCN      string   `json:"nameZhCN" v:"required|max-length:120" dc:"Chinese package name" eg:"100 GB"`
	NameEnUS      string   `json:"nameEnUS" v:"required|max-length:120" dc:"English package name" eg:"100 GB"`
	CurrencyCode  string   `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Package currency" eg:"CNY"`
	TrafficGB     int      `json:"trafficGb" v:"min:1|max:1000000" dc:"Fixed additional traffic in GB" eg:"100"`
	AmountMinor   int64    `json:"amountMinor" v:"min:0|max:1000000000" dc:"Final tax-inclusive price in minor units" eg:"1000"`
	Enabled       bool     `json:"enabled" dc:"Allow new purchases" eg:"true"`
	StockQuantity *int64   `json:"stockQuantity" dc:"Total sale ceiling; null means unlimited" eg:"100"`
	ProductNos    []string `json:"productNos" dc:"Applicable server product references" eg:"[]"`
}

type TrafficPackageCreateReq struct {
	g.Meta `path:"/admin/resource/traffic-packages" method:"post" tags:"IDC Server Operations" summary:"Create a fixed traffic package" dc:"Publish a fixed capacity and tax-inclusive price with explicit product eligibility and an optional total sale ceiling." permission:"idc:admin:resource:accessory-price:write"`
	TrafficPackageFields
}

type TrafficPackageCreateRes struct {
	Package *TrafficPackage `json:"package" dc:"Created package version" eg:"{}"`
}
