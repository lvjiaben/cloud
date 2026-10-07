// This file defines the simple product-version inventory update API.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductVersionInventoryUpdateReq struct {
	g.Meta           `path:"/admin/catalog/product-versions/{versionNo}/inventory" method:"put" tags:"IDC Catalog Management" summary:"Update product version inventory" dc:"Update the simple inventory setting of one draft product version." permission:"idc:admin:catalog:product:create"`
	VersionNo        string `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Stable draft product version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	TrackingEnabled  bool   `json:"trackingEnabled" dc:"Whether sales are limited by the configured quantity" eg:"true"`
	SellableQuantity int64  `json:"sellableQuantity" v:"min:0|max:9223372036854775807" dc:"Maximum sellable quantity when tracking is enabled" eg:"100"`
	ExpectedVersion  int64  `json:"expectedVersion" v:"required|min:1" dc:"Inventory version last read by the editor" eg:"1"`
}

type CatalogProductVersionInventoryUpdateRes struct {
	Inventory *CatalogProductVersionInventory `json:"inventory" dc:"Updated product inventory setting" eg:"{}"`
}

type CatalogProductVersionInventory struct {
	TrackingEnabled  bool  `json:"trackingEnabled" dc:"Whether sales are limited by inventory" eg:"true"`
	SellableQuantity int64 `json:"sellableQuantity" dc:"Configured sellable quantity" eg:"100"`
	Version          int64 `json:"version" dc:"Inventory optimistic concurrency version" eg:"2"`
}
