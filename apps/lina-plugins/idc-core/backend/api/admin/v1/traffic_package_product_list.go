// This file exposes bounded name choices owned by the product catalog.

package v1

import "github.com/gogf/gf/v2/frame/g"

type TrafficPackageProductListReq struct {
	g.Meta     `path:"/admin/resource/traffic-package-products" method:"get" tags:"IDC Server Operations" summary:"Search traffic package product choices" dc:"Return bounded catalog-owned server product labels for a keyword or an explicit selection of at most one hundred references." permission:"idc:admin:resource:accessory-price:read"`
	Keyword    string   `json:"keyword" v:"max-length:120" dc:"Product name search" eg:"100 GB"`
	ProductNos []string `json:"productNos" dc:"Exact product references for existing selection labels" eg:"[]"`
}

type TrafficPackageProductChoice struct {
	ProductNo string `json:"productNo" dc:"Server product reference" eg:"prd_00000000000000000000000000000000"`
	NameZhCN  string `json:"nameZhCN" dc:"Chinese product name" eg:"100 GB"`
	NameEnUS  string `json:"nameEnUS" dc:"English product name" eg:"100 GB"`
}

type TrafficPackageProductListRes struct {
	Items []*TrafficPackageProductChoice `json:"items" dc:"At most fifty name matches or one hundred exact choices" eg:"[]"`
}
