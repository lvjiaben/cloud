// This file lists fixed traffic packages for an authorized staff target server.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ServerTrafficPackageListReq uses the existing staff traffic operation grant.
type ServerTrafficPackageListReq struct {
	g.Meta    `path:"/admin/resource/server-accessories/{serviceNo}/traffic-packages" method:"get" tags:"IDC Server Accessories" summary:"List traffic packages for a managed server" dc:"Return applicable fixed packages after exact staff view and traffic-operation grants, merchant scope, server ownership, currency and capability checks." permission:"idc:admin:resource:server-accessory:traffic:add"`
	ServiceNo string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Stable target server service number" eg:"SVC_SERVER_0001"`
	Page      int    `json:"page" d:"1" v:"min:1" dc:"Numeric page starting at one" eg:"1"`
	PageSize  int    `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Maximum packages per page" eg:"20"`
	Keyword   string `json:"keyword" v:"max-length:120" dc:"Package name search" eg:"100 GB"`
}

// ServerTrafficPackage contains purchase facts, not global inventory or scope.
type ServerTrafficPackage struct {
	Version      int64  `json:"version" dc:"Immutable package version shown before confirmation" eg:"1"`
	PackageNo    string `json:"packageNo" dc:"Applicable package reference" eg:"tpk_00000000000000000000000000000000"`
	NameZhCN     string `json:"nameZhCN" dc:"Chinese package name" eg:"100 GB"`
	NameEnUS     string `json:"nameEnUS" dc:"English package name" eg:"100 GB"`
	TrafficGB    int    `json:"trafficGb" dc:"Fixed additional traffic in GB" eg:"100"`
	CurrencyCode string `json:"currencyCode" dc:"Package currency" eg:"CNY"`
	AmountMinor  int64  `json:"amountMinor" dc:"Final tax-inclusive price in minor units" eg:"1000"`
	Available    bool   `json:"available" dc:"Whether package stock remains available" eg:"true"`
}

// ServerTrafficPackageListRes preserves exact filtered totals for Vben paging.
type ServerTrafficPackageListRes struct {
	Items []*ServerTrafficPackage `json:"items" dc:"Current page of applicable packages" eg:"[]"`
	Total int                     `json:"total" dc:"Exact applicable package count" eg:"1"`
}
