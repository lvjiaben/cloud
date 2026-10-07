// This file defines creation or replacement of one visible draft product version's fulfillment binding.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogFulfillmentBindingUpdateReq struct {
	g.Meta              `path:"/admin/catalog/product-versions/{versionNo}/fulfillment-binding" method:"put" tags:"IDC Catalog Management" summary:"Bind product-version fulfillment interface" dc:"Bind one draft version to at most one currently admissible interface. This action requires both fulfillment-binding write and automation-interface view permissions." permission:"idc:admin:catalog:fulfillment-binding:write"`
	VersionNo           string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Draft product version number" eg:"ver_0123456789abcdef0123456789abcdef"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" v:"required|regex:^pfi_[0-9a-f]{32}$" dc:"Automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
}
type CatalogFulfillmentBindingUpdateRes struct {
	Binding *CatalogFulfillmentBindingSummary `json:"binding" dc:"Frozen binding summary" eg:"{}"`
}
