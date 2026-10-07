// This file defines append-only interface-configuration writes.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogAutomationInterfaceConfigurationUpdateReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}/configuration" method:"put" tags:"IDC Catalog Management" summary:"Update automation interface configuration" dc:"Validate current exact provider declarations and append one immutable configuration revision. Empty secret input preserves the current secret." permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string                                 `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$" dc:"Automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	Values              []*CatalogStructuredConfigurationValue `json:"values" v:"required" dc:"Complete typed configuration values"`
	ExpectedRevision    int64                                  `json:"expectedRevision" v:"required|min:1" dc:"Expected latest configuration revision" eg:"1"`
}

type CatalogAutomationInterfaceConfigurationUpdateRes struct {
	Configuration *CatalogAutomationInterfaceConfiguration `json:"configuration" dc:"Updated safe interface configuration"`
}
