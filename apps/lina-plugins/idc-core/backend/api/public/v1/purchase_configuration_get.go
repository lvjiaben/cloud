// This file defines the initial public purchase-configuration endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PurchaseConfigurationGetReq struct {
	g.Meta     `path:"/public/catalog/products/{publicSlug}/purchase-configuration" method:"get" tags:"IDC Public Catalog" summary:"Get governed purchase configuration" dc:"Return opaque fields and server-owned initial state for the current published product version."`
	PublicSlug string `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public browser slug"`
}

type PurchaseConfigurationGetRes struct {
	PurchaseConfigurationView
}
