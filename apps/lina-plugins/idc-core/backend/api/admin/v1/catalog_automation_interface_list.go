// This file defines the bounded automation-interface list contract and its server-side filters.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogAutomationInterfaceListReq struct {
	g.Meta                 `path:"/admin/catalog/automation/interfaces" method:"get" tags:"IDC Catalog Management" summary:"List automation interfaces" dc:"Return one bounded page filtered by connector selection, name, and new-business state without per-row provider lookups." permission:"idc:admin:catalog:automation-interface:view"`
	Page                   int    `json:"page" d:"1" v:"min:1" dc:"One-based page" eg:"1"`
	PageSize               int    `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Page size" eg:"20"`
	NamePrefix             string `json:"namePrefix" v:"max-length:128" dc:"Optional literal interface-name prefix" eg:"SSL"`
	NewBusinessState       string `json:"newBusinessState" dc:"Optional accepting or stopped state" eg:"accepting"`
	ProviderSelectionToken string `json:"providerSelectionToken" v:"max-length:4096" dc:"Optional authenticated connector-type selection token returned by provider discovery" eg:"eyJ..."`
}
type CatalogAutomationInterfaceListRes struct {
	Items []*CatalogAutomationInterfaceItem `json:"items" dc:"Automation interfaces" eg:"[]"`
	Total int                               `json:"total" dc:"Total matching interfaces" eg:"1"`
}
