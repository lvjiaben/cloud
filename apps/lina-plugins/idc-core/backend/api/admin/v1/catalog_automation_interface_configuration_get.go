// This file defines the redacted interface-configuration read endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogAutomationInterfaceConfigurationGetReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}/configuration" method:"get" tags:"IDC Catalog Management" summary:"Get automation interface configuration" dc:"Return the latest exact declaration, safe typed values, and redacted secret status." permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$" dc:"Automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
}

type CatalogAutomationInterfaceConfigurationGetRes struct {
	Configuration *CatalogAutomationInterfaceConfiguration `json:"configuration" dc:"Latest safe interface configuration"`
}
