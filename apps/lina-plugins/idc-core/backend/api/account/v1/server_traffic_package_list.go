// This file lists only packages applicable to the current customer's server.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerTrafficPackageListReq struct {
	g.Meta    `path:"/account/servers/{serviceNo}/traffic-packages" method:"get" tags:"IDC Customer Servers" summary:"List my server traffic packages" dc:"Check the current customer, server ownership and provider capability before returning applicable fixed packages and an exact total."`
	ServiceNo string `json:"serviceNo" in:"path" v:"required|length:1,40" dc:"Server owned by the current customer" eg:"SVC_SERVER_0001"`
	Page      int    `json:"page" d:"1" v:"min:1" dc:"Numeric page starting at one" eg:"1"`
	PageSize  int    `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Maximum packages per page" eg:"20"`
	Keyword   string `json:"keyword" v:"max-length:120" dc:"Package name search" eg:"100 GB"`
}

type ServerTrafficPackage struct {
	PackageNo    string `json:"packageNo" dc:"Applicable package reference" eg:"tpk_00000000000000000000000000000000"`
	NameZhCN     string `json:"nameZhCN" dc:"Chinese package name" eg:"100 GB"`
	NameEnUS     string `json:"nameEnUS" dc:"English package name" eg:"100 GB"`
	TrafficGB    int    `json:"trafficGb" dc:"Fixed additional traffic in GB" eg:"100"`
	CurrencyCode string `json:"currencyCode" dc:"Package currency" eg:"CNY"`
	AmountMinor  int64  `json:"amountMinor" dc:"Final tax-inclusive price in minor units" eg:"1000"`
	Available    bool   `json:"available" dc:"Whether package stock remains available" eg:"true"`
}

type ServerTrafficPackageListRes struct {
	Items []*ServerTrafficPackage `json:"items" dc:"Current page of applicable packages" eg:"[]"`
	Total int                     `json:"total" dc:"Exact applicable package count" eg:"1"`
}
