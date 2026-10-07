// This file defines optimistic display-name changes that affect only future automation bindings.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogAutomationInterfaceUpdateReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}" method:"put" tags:"IDC Catalog Management" summary:"Rename automation interface" dc:"Rename one interface without changing historical binding snapshots." permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$" dc:"Automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	DisplayName         string `json:"displayName" v:"required|max-length:128" dc:"New merchant-facing name" eg:"SSL Provider B"`
	ExpectedVersion     int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected optimistic concurrency version" eg:"2"`
}
type CatalogAutomationInterfaceUpdateRes struct {
	Interface *CatalogAutomationInterfaceItem `json:"interface" dc:"Renamed interface" eg:"{}"`
}
