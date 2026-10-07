// This file defines the authenticated request and safe response for creating one automation interface.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogAutomationInterfaceCreateReq struct {
	g.Meta         `path:"/admin/catalog/automation/interfaces" method:"post" tags:"IDC Catalog Management" summary:"Create automation interface" dc:"Create one interface from an authenticated provider selection." permission:"idc:admin:catalog:automation-interface:manage"`
	DisplayName    string                                 `json:"displayName" v:"required|max-length:128" dc:"Merchant-facing interface name" eg:"SSL Provider A"`
	SelectionToken string                                 `json:"selectionToken" v:"required|max-length:4096" dc:"Short-lived provider selection token" eg:"opaque-selection-token"`
	Configuration  []*CatalogStructuredConfigurationValue `json:"configuration" dc:"Typed interface configuration values; secret plaintext is accepted only here and never returned"`
}
type CatalogAutomationInterfaceCreateRes struct {
	Interface *CatalogAutomationInterfaceItem `json:"interface" dc:"Created interface" eg:"{}"`
}
