// This file defines current-customer merchant navigation and service reads.
package v1

import "github.com/gogf/gf/v2/frame/g"

type Directory struct {
	DirectoryNo  string `json:"directoryNo" dc:"Opaque merchant directory reference" eg:"pdr_example"`
	NameZhCn     string `json:"nameZhCn" dc:"Chinese directory name" eg:"My cloud services"`
	NameEnUs     string `json:"nameEnUs" dc:"English directory name" eg:"My cloud services"`
	SortOrder    int    `json:"sortOrder" dc:"Navigation order from 0 to 1000000" eg:"10"`
	ServiceCount int    `json:"serviceCount" dc:"Purchased entitlements of the current customer in this directory" eg:"2"`
}
type ListReq struct {
	g.Meta   `path:"/account/product-directories" method:"get" tags:"IDC Product Directories" summary:"List current customer product directories" dc:"List enabled merchant navigation menus and customer-scoped service counts"`
	Page     int `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number; defaults to 1" eg:"1"`
	PageSize int `json:"pageSize" d:"100" v:"min:1|max:100" dc:"Page size from 1 to 100; defaults to 100" eg:"100"`
}
type ListRes struct {
	Items    []Directory `json:"items" dc:"Merchant navigation menus on the current page" eg:"[]"`
	Total    int         `json:"total" dc:"Total enabled directories" eg:"1"`
	Page     int         `json:"page" dc:"Current page number" eg:"1"`
	PageSize int         `json:"pageSize" dc:"Current page size" eg:"100"`
}
type Entitlement struct {
	EntitlementNo   string `json:"entitlementNo" dc:"Opaque purchased service reference" eg:"ent_example"`
	ProductNo       string `json:"productNo" dc:"Opaque product reference" eg:"prd_example"`
	ProductNameZhCn string `json:"productNameZhCn" dc:"Chinese frozen purchased product name" eg:"Cloud server"`
	ProductNameEnUs string `json:"productNameEnUs" dc:"English frozen purchased product name" eg:"Cloud server"`
	PluginId        string `json:"pluginId" dc:"Frozen fulfillment plugin identifier" eg:"yunsu-ssl"`
	StatusCode      string `json:"statusCode" dc:"Service status: pending_activation, active, suspended, expired, terminated or cancelled" eg:"active"`
	OrderNo         string `json:"orderNo" dc:"Current customer order reference for service details" eg:"ord_example"`
	StartedAt       int64  `json:"startedAt" dc:"Service start Unix timestamp in milliseconds" eg:"1791475200000"`
	ExpiresAt       *int64 `json:"expiresAt" dc:"Service expiry Unix timestamp in milliseconds; null for no expiry" eg:"1794153600000"`
}
type ServicesReq struct {
	g.Meta      `path:"/account/product-directories/{directoryNo}/services" method:"get" tags:"IDC Product Directories" summary:"List purchased services by product directory" dc:"Read only current customer entitlements; the reserved all directory includes unclassified and disabled-directory services"`
	DirectoryNo string `json:"directoryNo" in:"path" v:"required|max-length:40" dc:"Merchant directory reference or reserved all fallback" eg:"all"`
	Page        int    `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number; defaults to 1" eg:"1"`
	PageSize    int    `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Page size from 1 to 100; defaults to 20" eg:"20"`
}
type ServicesRes struct {
	Items    []Entitlement `json:"items" dc:"Customer purchased services on the current page" eg:"[]"`
	Total    int           `json:"total" dc:"Total matching purchased services" eg:"1"`
	Page     int           `json:"page" dc:"Current page number" eg:"1"`
	PageSize int           `json:"pageSize" dc:"Current page size" eg:"20"`
}
