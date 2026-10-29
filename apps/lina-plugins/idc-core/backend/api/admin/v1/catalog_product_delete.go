// This file defines catalog deletion that preserves orders and existing services.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductDeleteReq struct {
	g.Meta    `path:"/admin/catalog/products/{productNo}" method:"delete" tags:"IDC Catalog Management" summary:"Delete catalog product" dc:"Remove a product from sales and management lists while preserving historical orders, existing services and renewals." permission:"idc:admin:catalog:product:delete"`
	ProductNo string `json:"productNo" in:"path" v:"required|regex:^prd_[0-9a-f]{32}$" dc:"Stable product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	Version   int64  `json:"version" v:"required|min:1" dc:"Last-read product metadata revision" eg:"1"`
}
type CatalogProductDeleteRes struct{}
