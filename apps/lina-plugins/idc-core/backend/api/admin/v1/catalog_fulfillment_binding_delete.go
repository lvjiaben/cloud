// This file defines idempotent removal of one visible draft product version's fulfillment binding.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogFulfillmentBindingDeleteReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/fulfillment-binding" method:"delete" tags:"IDC Catalog Management" summary:"Unbind product-version fulfillment interface" dc:"Remove the binding only while the visible product version remains a draft. This action requires both fulfillment-binding write and automation-interface view permissions." permission:"idc:admin:catalog:fulfillment-binding:write"`
	VersionNo string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Draft product version number" eg:"ver_0123456789abcdef0123456789abcdef"`
}
type CatalogFulfillmentBindingDeleteRes struct{}
