// This file defines optimistic, audited deletion of an automation interface that has never been referenced.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogAutomationInterfaceDeleteReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}" method:"delete" tags:"IDC Catalog Management" summary:"Delete mistaken automation interface" dc:"Delete only an interface that has never been referenced by any product version." permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$" dc:"Automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	ExpectedVersion     int64  `json:"expectedVersion" in:"query" v:"required|min:1" dc:"Expected optimistic concurrency version" eg:"1"`
}
type CatalogAutomationInterfaceDeleteRes struct{}
