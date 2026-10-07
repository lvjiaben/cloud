// This file defines bounded discovery of admitted automation providers through opaque selection tokens.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogAutomationProviderListReq struct {
	g.Meta   `path:"/admin/catalog/automation/providers" method:"get" tags:"IDC Catalog Management" summary:"List automation providers" dc:"Return one bounded page of currently admissible product-fulfillment providers." permission:"idc:admin:catalog:automation-interface:view"`
	Page     int `json:"page" d:"1" v:"min:1" dc:"One-based page" eg:"1"`
	PageSize int `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Page size" eg:"20"`
}
type CatalogAutomationProviderListRes struct {
	Items []*CatalogAutomationProviderItem `json:"items" dc:"Provider candidates" eg:"[]"`
	Total int                              `json:"total" dc:"Total provider candidates" eg:"1"`
}
