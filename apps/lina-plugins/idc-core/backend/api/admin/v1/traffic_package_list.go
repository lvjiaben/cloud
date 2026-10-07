// This file defines bounded merchant traffic-package settings and name choices.

package v1

import "github.com/gogf/gf/v2/frame/g"

// TrafficPackage preserves explicit currency and minor-unit final price.
type TrafficPackage struct {
	PackageNo         string   `json:"packageNo" dc:"Traffic package reference" eg:"tpk_00000000000000000000000000000000"`
	Version           int64    `json:"version" dc:"Current immutable package version" eg:"1"`
	NameZhCN          string   `json:"nameZhCN" dc:"Chinese package name" eg:"100 GB"`
	NameEnUS          string   `json:"nameEnUS" dc:"English package name" eg:"100 GB"`
	CurrencyCode      string   `json:"currencyCode" dc:"Price currency" eg:"CNY"`
	TrafficGB         int      `json:"trafficGb" dc:"Fixed additional traffic in GB" eg:"100"`
	AmountMinor       int64    `json:"amountMinor" dc:"Final tax-inclusive package price in minor units" eg:"1000"`
	Enabled           bool     `json:"enabled" dc:"Whether new purchases are enabled" eg:"true"`
	StockQuantity     *int64   `json:"stockQuantity" dc:"Total sale ceiling; null means unlimited" eg:"100"`
	OccupiedQuantity  int64    `json:"occupiedQuantity" dc:"Reserved unpaid and paid package quantity" eg:"30"`
	RemainingQuantity *int64   `json:"remainingQuantity" dc:"Remaining sale quantity; null means unlimited" eg:"70"`
	ProductNos        []string `json:"productNos" dc:"Applicable server products" eg:"[]"`
	PublishedAt       int64    `json:"publishedAt" dc:"Version publication time in Unix milliseconds" eg:"1788480000000"`
}

type TrafficPackageListReq struct {
	g.Meta   `path:"/admin/resource/traffic-packages" method:"get" tags:"IDC Server Operations" summary:"List fixed traffic packages" dc:"Return one bounded page and an exact total of merchant traffic-package settings and allocation counts." permission:"idc:admin:resource:accessory-price:read"`
	Page     int    `json:"page" d:"1" v:"min:1" dc:"Numeric page starting at one" eg:"1"`
	PageSize int    `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Maximum rows per page" eg:"20"`
	Keyword  string `json:"keyword" v:"max-length:120" dc:"Package name search" eg:"100 GB"`
}

type TrafficPackageListRes struct {
	Items []*TrafficPackage `json:"items" dc:"Current page of package settings" eg:"[]"`
	Total int               `json:"total" dc:"Exact matching package count" eg:"1"`
}
